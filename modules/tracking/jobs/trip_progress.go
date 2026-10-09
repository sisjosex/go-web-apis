package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"time"

	coreJobs "josex/web/modules/core/jobs"
	coreServices "josex/web/modules/core/services"
	"josex/web/modules/tracking/interfaces"
	"josex/web/modules/tracking/models"
	trackingRepos "josex/web/modules/tracking/repositories"
	"josex/web/modules/tracking/services"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

// TaskTripProgress sends a trip's guardians where their rider's bus is (MOBILE-024 D1): a data-only push
// the phone draws as an ongoing progress notification — the stops passed, the current one, the ETA.
const TaskTripProgress = "tracking:trip-progress"

// typeTripProgress is the push's `type` and the notice type a guardian mutes it by.
const typeTripProgress = "trip_progress"

// progressAfter groups a trip's events: an arrival, its tasks and the approach that follows send one
// push, the burst's last state.
const progressAfter = 2 * time.Second

// progressDoneTTL keeps "the end was sent" for a day: a trip does not last longer.
const progressDoneTTL = 24 * time.Hour

// TripProgress is the task's payload: the tenant travels with it, so sending needs no directory.
type TripProgress struct {
	TenantID    string `json:"tenant_id"`
	TenantSlug  string `json:"tenant_slug"`
	DatabaseURL string `json:"database_url,omitempty"`
	TripID      string `json:"trip_id"`
}

// ProgressSignal enqueues a trip's progress push. A nil ProgressSignal sends nothing — no FCM.
type ProgressSignal struct {
	client *asynq.Client
}

// NewProgressSignal builds the signal over the jobs client.
func NewProgressSignal(client *asynq.Client) *ProgressSignal {
	return &ProgressSignal{client: client}
}

// Signal enqueues one grouped progress push for the trip; a push already waiting takes this event too.
func (s *ProgressSignal) Signal(ctx context.Context, tenant coreJobs.Tenant, tripID uuid.UUID) error {
	if s == nil {
		return nil
	}
	raw, err := json.Marshal(TripProgress{
		TenantID: tenant.ID, TenantSlug: tenant.Slug, DatabaseURL: tenant.DatabaseURL, TripID: tripID.String(),
	})
	if err != nil {
		return err
	}
	_, err = s.client.EnqueueContext(ctx, asynq.NewTask(TaskTripProgress, raw),
		asynq.TaskID("progress:"+tenant.ID+":"+tripID.String()), asynq.ProcessIn(progressAfter),
		asynq.MaxRetry(3), asynq.Queue(coreJobs.QueueCritical))
	if err != nil && !errors.Is(err, asynq.ErrTaskIDConflict) {
		return err
	}
	return nil
}

// TripProgressHandler reads the trip's progress rows (one SP) and sends each guardian theirs, collapsed
// per trip and rider so a phone that was offline gets only the newest. The ETA comes from the trip's
// shared cache (30 s, one routing call); without one the push says the stops only. A rider's `done`
// goes once — valkey remembers it. A push outage fails the task: the retry repeats the same state.
func TripProgressHandler(db coreServices.DatabaseService, valkey coreServices.ValkeyService, pusher interfaces.Pusher, live *services.LiveService) asynq.HandlerFunc {
	return func(ctx context.Context, task *asynq.Task) error {
		var payload TripProgress
		if err := json.Unmarshal(task.Payload(), &payload); err != nil {
			return fmt.Errorf("trip-progress payload: %v: %w", err, asynq.SkipRetry)
		}
		tenantID, err := uuid.Parse(payload.TenantID)
		if err != nil {
			return fmt.Errorf("trip-progress tenant %q: %v: %w", payload.TenantID, err, asynq.SkipRetry)
		}
		tripID, err := uuid.Parse(payload.TripID)
		if err != nil {
			return fmt.Errorf("trip-progress trip %q: %v: %w", payload.TripID, err, asynq.SkipRetry)
		}
		if pusher == nil {
			return nil
		}
		tenantCtx := context.WithValue(ctx, coreServices.TenantDatabaseURLKey, payload.DatabaseURL)
		rows, err := trackingRepos.NewTrackingRepository(db).TripProgress(tenantCtx, tenantID, tripID)
		if err != nil || len(rows) == 0 {
			return err
		}
		eta := progressEta(tenantCtx, live, tenantID, tripID, rows)
		var outage error
		for _, row := range rows {
			if row.Phase == "done" && !firstDone(ctx, valkey, tripID, row) {
				continue
			}
			msg := interfaces.PushMessage{
				Data:       progressData(payload, row, eta),
				CollapseID: progressCollapseID(tripID, row.RiderID),
				Silent:     true,
				Urgent:     true,
			}
			if err := pushToUser(ctx, db, pusher, row.UserID, msg, "trip-progress "+tripID.String()); err != nil {
				outage = err
			}
		}
		return outage
	}
}

// progressEta answers each pending stop's ETA while the trip runs; nil when it cannot (no router, no
// position): the push then names the stops left without a time.
func progressEta(ctx context.Context, live *services.LiveService, tenantID, tripID uuid.UUID, rows []models.TripProgressRow) map[uuid.UUID]time.Time {
	if live == nil || rows[0].TripStatus != "in_progress" {
		return nil
	}
	// A cursor of now: the progress push needs no trail.
	now := time.Now()
	_, eta, err := live.Trip(ctx, tenantID, tripID, &now)
	if err != nil {
		log.Printf("⚠️  trip-progress %s: eta: %v", tripID, err)
		return nil
	}
	out := make(map[uuid.UUID]time.Time, len(eta.Eta))
	for _, e := range eta.Eta {
		out[e.TripStopID] = e.EtaAt
	}
	return out
}

// firstDone answers whether this rider's end has not been sent to this guardian yet, and marks it sent.
// Without valkey every end is sent: a repeated end only clears a notification already gone.
func firstDone(ctx context.Context, valkey coreServices.ValkeyService, tripID uuid.UUID, row models.TripProgressRow) bool {
	if valkey == nil {
		return true
	}
	key := "progress-done:" + tripID.String() + ":" + row.RiderID.String() + ":" + row.UserID.String()
	first, err := valkey.Client().SetNX(ctx, key, 1, progressDoneTTL).Result()
	return err != nil || first
}

// progressData is the push's data: strings only (FCM's), empty values left out.
func progressData(payload TripProgress, row models.TripProgressRow, eta map[uuid.UUID]time.Time) map[string]string {
	data := map[string]string{
		"type":        typeTripProgress,
		"tenant_slug": payload.TenantSlug,
		"trip_id":     payload.TripID,
		"rider_id":    row.RiderID.String(),
		"rider_name":  row.RiderName,
		"phase":       row.Phase,
		"stops_done":  strconv.Itoa(row.StopsDone),
		"stops_total": strconv.Itoa(row.StopsTotal),
	}
	for key, value := range map[string]*string{
		"current_stop": row.CurrentStop, "next_stop": row.NextStop, "target_stop": row.TargetStop,
		"vehicle_type": row.VehicleType, "organization_kind": row.OrganizationKind,
	} {
		if value != nil && *value != "" {
			data[key] = *value
		}
	}
	if row.TargetTripStopID != nil {
		if at, ok := eta[*row.TargetTripStopID]; ok {
			data["eta_at"] = at.UTC().Format(time.RFC3339)
		}
	}
	return data
}

// progressCollapseID is one rider's progress on one trip: each push replaces the last on the way, and
// it never collapses with the rider's notices (their ids are the plain v5 of the rider in the trip).
func progressCollapseID(tripID, riderID uuid.UUID) string {
	return uuid.NewSHA1(tripID, append([]byte(typeTripProgress), riderID[:]...)).String()
}
