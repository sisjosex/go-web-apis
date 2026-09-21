package jobs

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	coreServices "josex/web/modules/core/services"

	"github.com/hibiken/asynq"
)

// The scheduler lease: held 30 s, renewed every 10 s, so a holder that dies is replaced within 30 s
// and a slow renewal has two chances before the lease lapses.
const (
	SchedulerLeaseKey   = "jobs:scheduler:lease"
	SchedulerLeaseTTL   = 30 * time.Second
	SchedulerRenewEvery = 10 * time.Second
)

// Scheduler fires the registered entries only while it holds the lease, so any number of scheduler
// replicas fire each tick once. Losing the lease stops the asynq scheduler; winning it back starts a
// fresh one (an asynq scheduler cannot be restarted once shut down).
type Scheduler struct {
	valkey     coreServices.ValkeyService
	registry   *Registry
	lease      *Lease
	ttl        time.Duration
	renewEvery time.Duration

	// base is the caller's context without its cancellation: Lease calls must still work while the
	// process shuts down, and the loop itself stops on Shutdown, not on the signal.
	base context.Context

	mu        sync.Mutex
	running   *asynq.Scheduler
	confirmed time.Time // last time Valkey confirmed we hold the lease

	stop     chan struct{}
	stopOnce sync.Once
	done     chan struct{}
}

// NewScheduler builds a scheduler; cmd/server passes SchedulerLeaseTTL and SchedulerRenewEvery, tests
// pass shorter ones.
func NewScheduler(valkey coreServices.ValkeyService, registry *Registry, ttl, renewEvery time.Duration) *Scheduler {
	return &Scheduler{
		valkey:     valkey,
		registry:   registry,
		lease:      NewLease(valkey.Client(), SchedulerLeaseKey, ttl),
		ttl:        ttl,
		renewEvery: renewEvery,
		stop:       make(chan struct{}),
		done:       make(chan struct{}),
	}
}

// Start runs the lease loop in the background until Shutdown.
func (s *Scheduler) Start(ctx context.Context) {
	s.base = context.WithoutCancel(ctx)
	go func() {
		defer close(s.done)
		ticker := time.NewTicker(s.renewEvery)
		defer ticker.Stop()
		for {
			s.tick()
			select {
			case <-s.stop:
				return
			case <-ticker.C:
			}
		}
	}()
}

// tick takes or keeps the lease. A Valkey error while holding it keeps the entries firing until the
// lease would have lapsed anyway — past that point another replica may already hold it.
func (s *Scheduler) tick() {
	ctx, cancel := context.WithTimeout(s.base, s.renewEvery)
	defer cancel()

	s.mu.Lock()
	defer s.mu.Unlock()

	held := s.running != nil
	var ok bool
	var err error
	if held {
		ok, err = s.lease.Renew(ctx)
	} else {
		ok, err = s.lease.Acquire(ctx)
	}

	switch {
	case err != nil:
		if held && time.Since(s.confirmed) >= s.ttl {
			log.Printf("⚠️  scheduler: lease unconfirmed for %s, stopping: %v", s.ttl, err)
			s.stopRunning()
		}
	case ok:
		s.confirmed = time.Now()
		if !held {
			s.startRunning()
		}
	case held:
		log.Println("⚠️  scheduler: lease lost to another instance, stopping")
		s.stopRunning()
	}
}

func (s *Scheduler) startRunning() {
	scheduler := asynq.NewSchedulerFromRedisClient(s.valkey.Client(), &asynq.SchedulerOpts{
		LogLevel: asynq.WarnLevel,
		// The entries asynqmon lists are refreshed on this beat; the lease cadence keeps them current.
		HeartbeatInterval: s.renewEvery,
		PostEnqueueFunc: func(info *asynq.TaskInfo, err error) {
			if err != nil && !errors.Is(err, asynq.ErrDuplicateTask) {
				log.Printf("⚠️  scheduler: enqueue failed: %v", err)
			}
		},
	})
	for _, entry := range s.registry.entries {
		opts := append([]asynq.Option{asynq.Unique(entry.Period)}, entry.Opts...)
		if _, err := scheduler.Register(entry.Cron, entry.Task, opts...); err != nil {
			log.Printf("❌ scheduler: entry %q (%s): %v", entry.Task.Type(), entry.Cron, err)
		}
	}
	if err := scheduler.Start(); err != nil {
		log.Printf("❌ scheduler: %v", err)
		return
	}
	s.running = scheduler
	log.Printf("✅ Scheduler holds the lease, firing %d entries", len(s.registry.entries))
}

func (s *Scheduler) stopRunning() {
	if s.running == nil {
		return
	}
	s.running.Shutdown()
	s.running = nil
}

// IsLeader answers whether this instance is firing the entries right now.
func (s *Scheduler) IsLeader() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running != nil
}

// Shutdown stops firing and releases the lease so another replica takes over at its next tick.
// Calling it again is a no-op.
func (s *Scheduler) Shutdown() {
	s.stopOnce.Do(func() { close(s.stop) })
	<-s.done

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running == nil {
		return
	}
	s.stopRunning()
	ctx, cancel := context.WithTimeout(s.base, 2*time.Second)
	defer cancel()
	if err := s.lease.Release(ctx); err != nil {
		log.Printf("⚠️  scheduler: could not release the lease, it lapses in %s: %v", s.ttl, err)
	}
}
