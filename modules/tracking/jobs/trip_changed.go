package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	coreJobs "josex/web/modules/core/jobs"
	coreServices "josex/web/modules/core/services"
	geoInterfaces "josex/web/modules/geo/interfaces"
	"josex/web/modules/tracking/models"
	trackingRepos "josex/web/modules/tracking/repositories"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

// topicTripChanged is what every trip, stop and task transition writes to tracking.outbox (TRACK-020).
const topicTripChanged = "trip.changed"

// TaskTripChanged is the outbox task one of those rows becomes.
var TaskTripChanged = coreJobs.OutboxTask(topicTripChanged)

// TripChanged is one such row: which trip moved, on which route and day, how — the action, or
// "override" — and the task or stop it moved, when one.
type TripChanged struct {
	TripID      string `json:"trip_id"`
	RouteID     string `json:"route_id"`
	ServiceDate string `json:"service_date"`
	Type        string `json:"type"`
	SubjectID   string `json:"subject_id"`
}

// frameOf is the frame type a transition is published as (TRACK-025 step 4).
var frameOf = map[string]string{
	"start": FrameTripStatus, "complete": FrameTripStatus, "cancel": FrameTripStatus, "override": FrameTripStatus,
	"arrive": FrameStop, "skip": FrameStop,
	"done": FrameTask, "no_show": FrameTask,
}

// TripChangedHandler consumes trip.changed: every transition is published to the tenant's fleet, the
// trip, its riders and their organizations' fleets (TRACK-025), and a completed trip gets its driven
// path stored (TRACK-010 step 4). The tenant comes from the TaskID the relay gave the row,
// <tenant>:<id>, as route.changed's. It then becomes the riders' guardians' notices (TRACK-012).
// The trip's driver is told its day changed (TRACK-030), but for an approach, which changes nothing
// the driver's day shows. valkey nil publishes nothing; notify and signal nil notify and signal
// nothing. Delivery is at least once: a repeated frame makes a client refetch the same trip, a
// repeated trace writes the same line, a repeated notice writes nothing.
func TripChangedHandler(db coreServices.DatabaseService, tenants *coreJobs.TenantDirectory, router geoInterfaces.Router, valkey coreServices.ValkeyService, notify *Notifier, signal *DriverSignal) asynq.HandlerFunc {
	return func(ctx context.Context, task *asynq.Task) error {
		var payload TripChanged
		if err := json.Unmarshal(task.Payload(), &payload); err != nil {
			return fmt.Errorf("trip.changed payload: %v: %w", err, asynq.SkipRetry)
		}
		tripID, err := uuid.Parse(payload.TripID)
		if err != nil {
			return fmt.Errorf("trip.changed trip_id %q: %v: %w", payload.TripID, err, asynq.SkipRetry)
		}
		tenant, ok, err := taskTenant(ctx, tenants)
		if err != nil || !ok {
			return err
		}
		// A completed trip's line is stored before the frame goes out: a client re-reads the trip on
		// that frame and must find the line there. A failed trace still lets the frame go, and fails the
		// task so asynq retries both — a repeated frame only makes a client re-read.
		var traceErr error
		if payload.Type == "complete" {
			if _, traceErr = ApplyTripTrace(ctx, db, router, tenant, tripID); traceErr != nil {
				log.Printf("⚠️  trip.changed %s: trace: %v", tripID, traceErr)
			}
		}
		subjectID, _ := uuid.Parse(payload.SubjectID)
		if err := PublishTripChanged(ctx, db, valkey, tenant, tripID, payload.Type, subjectID); err != nil {
			return err
		}
		if err := notify.Notify(ctx, tenant, topicTripChanged, task.Payload()); err != nil {
			return err
		}
		if payload.Type != "approach" {
			if err := signal.Signal(ctx, tenant, nil, &tripID, nil, nil); err != nil {
				return err
			}
		}
		return traceErr
	}
}

// PublishTripChanged sends one transition as a {type, data: {trip_id, type}} frame to everyone
// watching the trip: its riders not yet off it, and the ones whose task or stop subjectID is
// (uuid.Nil when none) — a rider's own dropoff still reaches its channel (TRACK-030 D1).
func PublishTripChanged(ctx context.Context, db coreServices.DatabaseService, valkey coreServices.ValkeyService, tenant coreJobs.Tenant, tripID uuid.UUID, transition string, subjectID uuid.UUID) error {
	frameType, ok := frameOf[transition]
	if !ok || valkey == nil {
		return nil
	}
	tenantID, err := uuid.Parse(tenant.ID)
	if err != nil {
		return err
	}
	tenantCtx := context.WithValue(ctx, coreServices.TenantDatabaseURLKey, tenant.DatabaseURL)
	riders, organizations, err := trackingRepos.NewTrackingRepository(db).TripAudience(tenantCtx, tenantID, tripID, subjectID)
	if err != nil {
		return err
	}
	data, err := json.Marshal(map[string]string{"trip_id": tripID.String(), "type": transition})
	if err != nil {
		return err
	}
	channels := vehicleChannels(tenant.ID, models.IngestedVehicle{TripID: &tripID, RiderIDs: riders, OrganizationIDs: organizations})
	pipe := valkey.Client().Pipeline()
	publishTo(ctx, pipe, channels, Frame{Type: frameType, Data: data})
	_, err = pipe.Exec(ctx)
	return err
}

// taskTenant answers the tenant an outbox task belongs to, from its TaskID (the row's tenant_id,
// INFRA-009), out of the directory held in memory; ok false when that tenant is no longer listed —
// nothing is left to do for it.
func taskTenant(ctx context.Context, tenants *coreJobs.TenantDirectory) (coreJobs.Tenant, bool, error) {
	taskID, _ := asynq.GetTaskID(ctx)
	tenantID, _, _ := strings.Cut(taskID, ":")
	tenant, ok, err := tenants.Find(ctx, tenantID)
	if err != nil || ok {
		return tenant, ok, err
	}
	log.Printf("⚠️  outbox task %s: tenant %q is not listed, dropped", taskID, tenantID)
	return coreJobs.Tenant{}, false, nil
}
