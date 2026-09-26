package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	coreJobs "josex/web/modules/core/jobs"
	coreServices "josex/web/modules/core/services"
	"josex/web/modules/tracking/models"
	trackingRepos "josex/web/modules/tracking/repositories"
	trackingServices "josex/web/modules/tracking/services"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	// gpsGroup is the one consumer group on every tenant's stream: two workers share the entries.
	gpsGroup = "gps"
	// gpsReadCount and gpsReadBlock are D2's batch: up to 1 000 points, or whatever arrived in 200 ms.
	gpsReadCount = 1000
	gpsReadBlock = 200 * time.Millisecond
	// gpsPublishEvery is the position throttle: at most one position frame per vehicle this often.
	gpsPublishEvery = 3 * time.Second
	// gpsRetryAfter is the pause after a batch the database refused; its entries stay pending.
	gpsRetryAfter = time.Second
)

// Channel names every publisher and subscriber agrees on (TRACK-010 step 3, TRACK-025).
func FleetChannel(tenantID string) string { return "fleet:" + tenantID }
func FleetOrgChannel(tenantID, organizationID string) string {
	return "fleet:" + tenantID + ":org:" + organizationID
}
func TripChannel(tripID string) string   { return "trip:" + tripID }
func RiderChannel(riderID string) string { return "rider:" + riderID }

// Frame is what goes over Valkey pub/sub; the gateway adds the channel and its per-connection seq.
type Frame struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

// Frame types published on the channels.
const (
	FramePosition   = "position"
	FrameStop       = "stop"
	FrameTripStatus = "trip_status"
	FrameTask       = "task"
	FrameEta        = "eta"
)

// PositionsConsumer drains every tenant's GPS stream into its database (D2): one goroutine per
// tenant, walking the tenants as the outbox relay does, one sp_ingest_positions per read. What a read
// stored is then published — a vehicle's position at most every gpsPublishEvery, an arrival at once.
type PositionsConsumer struct {
	db             coreServices.DatabaseService
	client         *redis.Client
	tenants        coreJobs.TenantLister
	arrivalRadiusM int
	name           string
	// ClaimIdle is how long an entry stays pending before another consumer takes it over (60 s), and
	// ClaimEvery how often a consumer looks for such entries (30 s).
	ClaimIdle  time.Duration
	ClaimEvery time.Duration

	mu      sync.Mutex
	running map[coreJobs.Tenant]context.CancelFunc
	wg      sync.WaitGroup
	cancel  context.CancelFunc
}

func NewPositionsConsumer(db coreServices.DatabaseService, valkey coreServices.ValkeyService, tenants coreJobs.TenantLister, arrivalRadiusM int) *PositionsConsumer {
	host, _ := os.Hostname()
	return &PositionsConsumer{
		db:             db,
		client:         valkey.Client(),
		tenants:        tenants,
		arrivalRadiusM: arrivalRadiusM,
		name:           fmt.Sprintf("%s-%d-%s", host, os.Getpid(), uuid.NewString()[:8]),
		ClaimIdle:      60 * time.Second,
		ClaimEvery:     30 * time.Second,
		running:        map[coreJobs.Tenant]context.CancelFunc{},
	}
}

// Start runs the tenant refresh loop until Shutdown, like the relay's.
func (pc *PositionsConsumer) Start(ctx context.Context) {
	ctx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	pc.cancel = cancel
	pc.wg.Add(1)
	go func() {
		defer pc.wg.Done()
		for {
			wait := time.Minute
			if err := pc.refresh(ctx); err != nil {
				log.Printf("⚠️  gps consumer: %v", err)
				wait = 5 * time.Second
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(wait):
			}
		}
	}()
	log.Printf("✅ GPS consumer %s started", pc.name)
}

// Shutdown stops every tenant's loop and waits for the batch in flight.
func (pc *PositionsConsumer) Shutdown() {
	if pc.cancel != nil {
		pc.cancel()
	}
	pc.wg.Wait()
}

func (pc *PositionsConsumer) refresh(ctx context.Context) error {
	tenants, err := pc.tenants(ctx)
	if err != nil {
		return fmt.Errorf("listing tenants: %w", err)
	}
	pc.mu.Lock()
	defer pc.mu.Unlock()
	listed := make(map[coreJobs.Tenant]bool, len(tenants))
	for _, t := range tenants {
		listed[t] = true
		if _, ok := pc.running[t]; ok {
			continue
		}
		tenantCtx, cancel := context.WithCancel(ctx)
		pc.running[t] = cancel
		pc.wg.Add(1)
		go func(t coreJobs.Tenant) {
			defer pc.wg.Done()
			pc.runTenant(tenantCtx, t)
		}(t)
	}
	for t, cancel := range pc.running {
		if !listed[t] {
			cancel()
			delete(pc.running, t)
		}
	}
	return nil
}

// runTenant keeps one tenant's stream drained. The group starts at the stream's first entry, so what
// the API queued before any worker existed is stored too.
func (pc *PositionsConsumer) runTenant(ctx context.Context, t coreJobs.Tenant) {
	tenantID, err := uuid.Parse(t.ID)
	if err != nil {
		log.Printf("⚠️  gps consumer: tenant id %q: %v", t.ID, err)
		return
	}
	stream := trackingServices.GPSStream(t.ID)
	repo := trackingRepos.NewTrackingRepository(pc.db)
	tenantCtx := context.WithValue(ctx, coreServices.TenantDatabaseURLKey, t.DatabaseURL)
	lastClaim := time.Time{}

	for ctx.Err() == nil {
		if err := pc.client.XGroupCreateMkStream(ctx, stream, gpsGroup, "0").Err(); err != nil && !strings.HasPrefix(err.Error(), "BUSYGROUP") {
			pc.pause(ctx, fmt.Errorf("create group on %s: %w", stream, err))
			continue
		}

		if time.Since(lastClaim) > pc.ClaimEvery {
			lastClaim = time.Now()
			claimed, _, err := pc.client.XAutoClaim(ctx, &redis.XAutoClaimArgs{
				Stream: stream, Group: gpsGroup, Consumer: pc.name, MinIdle: pc.ClaimIdle, Start: "0-0", Count: gpsReadCount,
			}).Result()
			if err == nil && len(claimed) > 0 {
				if err := pc.store(tenantCtx, repo, tenantID, stream, claimed); err != nil {
					pc.pause(ctx, err)
					continue
				}
			}
		}

		res, err := pc.client.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group: gpsGroup, Consumer: pc.name, Streams: []string{stream, ">"}, Count: gpsReadCount, Block: gpsReadBlock,
		}).Result()
		if errors.Is(err, redis.Nil) || ctx.Err() != nil {
			continue
		}
		if err != nil {
			pc.pause(ctx, fmt.Errorf("read %s: %w", stream, err))
			continue
		}
		for _, s := range res {
			if err := pc.store(tenantCtx, repo, tenantID, stream, s.Messages); err != nil {
				pc.pause(ctx, err)
			}
		}
	}
}

func (pc *PositionsConsumer) pause(ctx context.Context, err error) {
	if ctx.Err() != nil {
		return
	}
	log.Printf("⚠️  gps consumer: %v (retrying in %s)", err, gpsRetryAfter)
	select {
	case <-ctx.Done():
	case <-time.After(gpsRetryAfter):
	}
}

// store writes the entries in one SP call, acks them and publishes what changed. An entry that is not
// a point is acked and dropped — retrying it can only fail again. A database failure leaves every
// entry pending; the claim picks them up again.
func (pc *PositionsConsumer) store(ctx context.Context, repo *trackingRepos.TrackingRepository, tenantID uuid.UUID, stream string, messages []redis.XMessage) error {
	ids := make([]string, 0, len(messages))
	points := make([]json.RawMessage, 0, len(messages))
	for _, m := range messages {
		ids = append(ids, m.ID)
		if raw, ok := m.Values[trackingServices.GPSStreamField].(string); ok && json.Valid([]byte(raw)) {
			points = append(points, json.RawMessage(raw))
		}
	}
	if len(points) > 0 {
		payload, err := json.Marshal(points)
		if err != nil {
			return err
		}
		stored, err := repo.IngestPositions(ctx, tenantID, payload, pc.arrivalRadiusM)
		if err != nil {
			return fmt.Errorf("store %d point(s) of %s: %w", len(points), stream, err)
		}
		defer pc.publish(ctx, tenantID.String(), stored)
	}
	return pc.client.XAck(ctx, stream, gpsGroup, ids...).Err()
}

// publish sends each vehicle's position to its fleet, trip and rider channels when its throttle key
// is free, and an arrival to the same channels always. A publish that fails is lost, not retried: the
// next position replaces it and a client resyncs from the snapshot on a seq gap.
func (pc *PositionsConsumer) publish(ctx context.Context, tenantID string, vehicles []models.IngestedVehicle) {
	pipe := pc.client.Pipeline()
	queued := 0
	for _, v := range vehicles {
		channels := vehicleChannels(tenantID, v)
		if v.ArrivedStopID != nil {
			data, _ := json.Marshal(map[string]any{"trip_id": v.TripID, "trip_stop_id": v.ArrivedStopID, "vehicle_id": v.VehicleID, "type": "arrive"})
			queued += publishTo(ctx, pipe, channels, Frame{Type: FrameStop, Data: data})
		}
		won, err := pc.client.SetNX(ctx, "pub:"+v.VehicleID.String(), 1, gpsPublishEvery).Result()
		if err != nil || !won {
			continue
		}
		queued += publishTo(ctx, pipe, channels, Frame{Type: FramePosition, Data: v.Last})
	}
	if queued == 0 {
		return
	}
	if _, err := pipe.Exec(ctx); err != nil {
		log.Printf("⚠️  gps consumer: publish: %v", err)
	}
}

// vehicleChannels is everyone watching this vehicle: the tenant's fleet, the fleet of each
// organization its trip carries riders of, the trip, and each rider on it.
func vehicleChannels(tenantID string, v models.IngestedVehicle) []string {
	channels := []string{FleetChannel(tenantID)}
	for _, o := range v.OrganizationIDs {
		channels = append(channels, FleetOrgChannel(tenantID, o.String()))
	}
	if v.TripID != nil {
		channels = append(channels, TripChannel(v.TripID.String()))
	}
	for _, r := range v.RiderIDs {
		channels = append(channels, RiderChannel(r.String()))
	}
	return channels
}

func publishTo(ctx context.Context, pipe redis.Pipeliner, channels []string, frame Frame) int {
	payload, err := json.Marshal(frame)
	if err != nil {
		return 0
	}
	for _, ch := range channels {
		pipe.Publish(ctx, ch, payload)
	}
	return len(channels)
}
