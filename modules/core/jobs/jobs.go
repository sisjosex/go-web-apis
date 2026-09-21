// Package jobs is the background half of the process (INFRA-001): the asynq client every role may
// enqueue with, the worker that runs handlers, the scheduler that fires periodic entries while it
// holds the lease, and the outbox relay. Modules contribute handlers and entries through a Registry
// built in cmd/server, so core never imports a business module.
package jobs

import (
	"time"

	coreServices "josex/web/modules/core/services"

	"github.com/hibiken/asynq"
)

// Queues, by urgency. The worker polls them 6:3:1, so a burst of low work never starves a critical
// task and low work still moves while critical is busy.
const (
	QueueCritical = "critical"
	QueueDefault  = "default"
	QueueLow      = "low"
)

var queueWeights = map[string]int{QueueCritical: 6, QueueDefault: 3, QueueLow: 1}

// Entry is one periodic task. Period is the task's Unique window: a second scheduler firing the same
// tick during a lease hand-over enqueues a duplicate that asynq drops.
type Entry struct {
	Cron   string
	Period time.Duration
	Task   *asynq.Task
	Opts   []asynq.Option
}

// Registry collects what the modules contribute: handlers for the worker, entries for the scheduler.
type Registry struct {
	mux     *asynq.ServeMux
	entries []Entry
}

func NewRegistry() *Registry {
	return &Registry{mux: asynq.NewServeMux()}
}

// Handle routes a task type to its handler. A handler must be idempotent: asynq delivers at least
// once, and the outbox relay may publish a row again after a crash between enqueue and UPDATE.
func (r *Registry) Handle(taskType string, handler asynq.HandlerFunc) {
	r.mux.HandleFunc(taskType, handler)
}

// Schedule adds a periodic entry, fired by whichever scheduler holds the lease.
func (r *Registry) Schedule(entry Entry) {
	r.entries = append(r.entries, entry)
}

// NewClient returns an enqueue-only client on the shared Valkey pool; closing it leaves the pool open.
func NewClient(valkey coreServices.ValkeyService) *asynq.Client {
	return asynq.NewClientFromRedisClient(valkey.Client())
}
