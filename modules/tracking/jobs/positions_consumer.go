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

// PositionsConsumer drains the GPS streams into their databases (D2), one loop per database rather
// than per tenant (INFRA-009 D3): gps:shared for every tenant of the platform database, gps:{tenant}
// for a dedicated one. Each read is grouped by tenant into one sp_ingest_positions per tenant; what it
// stored is then published — a vehicle's position at most every gpsPublishEvery, an arrival at once.
type PositionsConsumer struct {
	db      coreServices.DatabaseService
	client  *redis.Client
	tenants *coreJobs.TenantDirectory
	radii   models.IngestRadii
	name    string
	// ClaimIdle is how long an entry stays pending before another consumer takes it over (60 s), and
	// ClaimEvery how often a consumer looks for such entries (30 s).
	ClaimIdle  time.Duration
	ClaimEvery time.Duration

	mu      sync.Mutex
	running map[string]context.CancelFunc
	wg      sync.WaitGroup
	cancel  context.CancelFunc
}

// gpsLoop is one stream a loop drains. tenant is whose an untagged entry is: a dedicated database's
// one tenant, or the shared tenant whose pre-INFRA-009 stream this is (legacy: deleted once empty).
// The shared stream tags every entry and has none.
type gpsLoop struct {
	stream string
	tenant *coreJobs.Tenant
	legacy bool
}

func NewPositionsConsumer(db coreServices.DatabaseService, valkey coreServices.ValkeyService, tenants *coreJobs.TenantDirectory, radii models.IngestRadii) *PositionsConsumer {
	host, _ := os.Hostname()
	return &PositionsConsumer{
		db:         db,
		client:     valkey.Client(),
		tenants:    tenants,
		radii:      radii,
		name:       fmt.Sprintf("%s-%d-%s", host, os.Getpid(), uuid.NewString()[:8]),
		ClaimIdle:  60 * time.Second,
		ClaimEvery: 30 * time.Second,
		running:    map[string]context.CancelFunc{},
	}
}

// Start runs the stream refresh loop until Shutdown, like the relay's.
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

// Shutdown stops every loop and waits for the batch in flight.
func (pc *PositionsConsumer) Shutdown() {
	if pc.cancel != nil {
		pc.cancel()
	}
	pc.wg.Wait()
}

// loops is every stream to drain now: the shared one, each dedicated tenant's, and each shared
// tenant's legacy stream that still exists.
func (pc *PositionsConsumer) loops(ctx context.Context) ([]gpsLoop, error) {
	dbs, err := pc.tenants.Databases(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing tenants: %w", err)
	}
	var loops []gpsLoop
	for _, d := range dbs {
		if !d.Shared() {
			for _, t := range d.Tenants {
				loops = append(loops, gpsLoop{stream: trackingServices.GPSStream(t.ID), tenant: &t})
			}
			continue
		}
		loops = append(loops, gpsLoop{stream: trackingServices.GPSSharedStream})
		for _, t := range d.Tenants {
			legacy := trackingServices.GPSStream(t.ID)
			if n, err := pc.client.Exists(ctx, legacy).Result(); err == nil && n > 0 {
				loops = append(loops, gpsLoop{stream: legacy, tenant: &t, legacy: true})
			}
		}
	}
	return loops, nil
}

func (pc *PositionsConsumer) refresh(ctx context.Context) error {
	loops, err := pc.loops(ctx)
	if err != nil {
		return err
	}
	pc.mu.Lock()
	defer pc.mu.Unlock()
	listed := make(map[string]bool, len(loops))
	for _, l := range loops {
		listed[l.stream] = true
		if _, ok := pc.running[l.stream]; ok {
			continue
		}
		loopCtx, cancel := context.WithCancel(ctx)
		pc.running[l.stream] = cancel
		pc.wg.Add(1)
		go func(l gpsLoop) {
			defer pc.wg.Done()
			pc.runStream(loopCtx, l)
			pc.mu.Lock()
			delete(pc.running, l.stream)
			pc.mu.Unlock()
		}(l)
	}
	for stream, cancel := range pc.running {
		if !listed[stream] {
			cancel()
		}
	}
	return nil
}

// runStream keeps one stream drained. The group starts at the stream's first entry, so what the API
// queued before any worker existed is stored too. A legacy stream ends its loop once nothing in it is
// new or pending, and is deleted (INFRA-009 D3).
func (pc *PositionsConsumer) runStream(ctx context.Context, l gpsLoop) {
	lastClaim := time.Time{}
	for ctx.Err() == nil {
		if err := pc.client.XGroupCreateMkStream(ctx, l.stream, gpsGroup, "0").Err(); err != nil && !strings.HasPrefix(err.Error(), "BUSYGROUP") {
			pc.pause(ctx, fmt.Errorf("create group on %s: %w", l.stream, err))
			continue
		}

		if time.Since(lastClaim) > pc.ClaimEvery {
			lastClaim = time.Now()
			if err := pc.claimStale(ctx, l); err != nil {
				pc.pause(ctx, err)
				continue
			}
		}

		res, err := pc.client.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group: gpsGroup, Consumer: pc.name, Streams: []string{l.stream, ">"}, Count: gpsReadCount, Block: gpsReadBlock,
		}).Result()
		if errors.Is(err, redis.Nil) && l.legacy && pc.retire(ctx, l.stream) {
			return
		}
		if errors.Is(err, redis.Nil) || ctx.Err() != nil {
			continue
		}
		if err != nil {
			pc.pause(ctx, fmt.Errorf("read %s: %w", l.stream, err))
			continue
		}
		for _, s := range res {
			if err := pc.store(ctx, l, s.Messages); err != nil {
				pc.pause(ctx, err)
			}
		}
	}
}

// claimStale takes over what another consumer left pending past ClaimIdle and stores it.
func (pc *PositionsConsumer) claimStale(ctx context.Context, l gpsLoop) error {
	claimed, _, err := pc.client.XAutoClaim(ctx, &redis.XAutoClaimArgs{
		Stream: l.stream, Group: gpsGroup, Consumer: pc.name, MinIdle: pc.ClaimIdle, Start: "0-0", Count: gpsReadCount,
	}).Result()
	if err != nil || len(claimed) == 0 {
		return nil
	}
	return pc.store(ctx, l, claimed)
}

// retire deletes a legacy stream once no entry in it is pending: everything it held is stored.
func (pc *PositionsConsumer) retire(ctx context.Context, stream string) bool {
	pending, err := pc.client.XPending(ctx, stream, gpsGroup).Result()
	if err != nil || pending.Count > 0 {
		return false
	}
	if err := pc.client.Del(ctx, stream).Err(); err != nil {
		return false
	}
	log.Printf("✅ gps consumer: drained and deleted %s", stream)
	return true
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

// store writes a read's entries, one SP call per tenant in it, acks them and publishes what changed.
// An entry that is not a point, or names no tenant the directory knows, is acked and dropped —
// retrying it can only fail again. A database failure leaves every entry pending; the claim picks
// them up again, and a point stored twice is a no-op (vehicle_id + recorded_at).
func (pc *PositionsConsumer) store(ctx context.Context, l gpsLoop, messages []redis.XMessage) error {
	ids := make([]string, 0, len(messages))
	byTenant := map[string][]json.RawMessage{}
	for _, m := range messages {
		ids = append(ids, m.ID)
		raw, ok := m.Values[trackingServices.GPSStreamField].(string)
		if !ok || !json.Valid([]byte(raw)) {
			continue
		}
		tenantID, _ := m.Values[trackingServices.GPSTenantField].(string)
		if tenantID == "" && l.tenant != nil {
			tenantID = l.tenant.ID
		}
		if tenantID != "" {
			byTenant[tenantID] = append(byTenant[tenantID], json.RawMessage(raw))
		}
	}
	repo := trackingRepos.NewTrackingRepository(pc.db)
	for id, points := range byTenant {
		tenant, ok, err := pc.tenants.Find(ctx, id)
		if err != nil {
			return err
		}
		tenantID, parseErr := uuid.Parse(id)
		if !ok || parseErr != nil {
			log.Printf("⚠️  gps consumer: %d point(s) on %s for unknown tenant %q, dropped", len(points), l.stream, id)
			continue
		}
		payload, err := json.Marshal(points)
		if err != nil {
			return err
		}
		tenantCtx := context.WithValue(ctx, coreServices.TenantDatabaseURLKey, tenant.DatabaseURL)
		stored, err := repo.IngestPositions(tenantCtx, tenantID, payload, pc.radii)
		if err != nil {
			return fmt.Errorf("store %d point(s) of %s on %s: %w", len(points), id, l.stream, err)
		}
		defer pc.publish(ctx, id, stored)
	}
	return pc.client.XAck(ctx, l.stream, gpsGroup, ids...).Err()
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
