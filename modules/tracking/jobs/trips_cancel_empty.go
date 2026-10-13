package jobs

import (
	"context"
	"fmt"
	"log"
	"time"

	coreJobs "josex/web/modules/core/jobs"
	coreServices "josex/web/modules/core/services"
	"josex/web/modules/tracking/interfaces"
	trackingRepos "josex/web/modules/tracking/repositories"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

const (
	// TaskTripsCancelEmpty cancels the trips of today still without riders once their client's lead is
	// reached (TRACK-069 D1); the scheduler fires it every five minutes.
	TaskTripsCancelEmpty = "tracking:trips-cancel-empty"
	// TripsCancelEmptyCron is every five minutes: a trip cancels at most five minutes past its lead.
	TripsCancelEmptyCron = "*/5 * * * *"
	// TripsCancelEmptyPeriod is the cron's step, the task's Unique window.
	TripsCancelEmptyPeriod = 5 * time.Minute
)

// CancelEmptyResult is one tenant's share of a pass.
type CancelEmptyResult struct {
	Tenant    coreJobs.Tenant
	Cancelled int
	Err       error
}

// RunTripsCancelEmptyPass calls sp_cancel_empty_trips once per tenant and tells each cancelled trip's
// driver, when linked to a user and a pusher is set (TRACK-069 D2). A tenant that fails is reported
// and the walk continues; a push that fails is logged — the cancel already happened and the board
// shows it, so the pass never repeats it.
func RunTripsCancelEmptyPass(ctx context.Context, db coreServices.DatabaseService, pusher interfaces.Pusher, tenants []coreJobs.Tenant) []CancelEmptyResult {
	repo := trackingRepos.NewTrackingRepository(db)
	results := make([]CancelEmptyResult, 0, len(tenants))
	for _, tenant := range tenants {
		result := CancelEmptyResult{Tenant: tenant}
		tenantID, err := uuid.Parse(tenant.ID)
		if err == nil {
			tenantCtx := context.WithValue(ctx, coreServices.TenantDatabaseURLKey, tenant.DatabaseURL)
			var trips []trackingRepos.CancelledEmptyTrip
			trips, err = repo.CancelEmptyTrips(tenantCtx, tenantID)
			result.Cancelled = len(trips)
			for _, trip := range trips {
				if pusher == nil || trip.DriverUserID == nil {
					continue
				}
				if pushErr := pushToUser(ctx, db, pusher, *trip.DriverUserID, cancelledEmptyMessage(tenant, trip), "trip-cancel-empty "+trip.TripID.String()); pushErr != nil {
					log.Printf("⚠️  trips cancel empty %s: push %s: %v", tenant.ID, trip.TripID, pushErr)
				}
			}
		}
		result.Err = err
		results = append(results, result)
	}
	return results
}

// cancelledEmptyMessage is the driver's visible notice: the route and the start as a clock time in
// the trip's own zone.
func cancelledEmptyMessage(tenant coreJobs.Tenant, trip trackingRepos.CancelledEmptyTrip) interfaces.PushMessage {
	start := trip.PlannedStart.UTC()
	if loc, err := time.LoadLocation(trip.Timezone); err == nil {
		start = trip.PlannedStart.In(loc)
	}
	return interfaces.PushMessage{
		Type:       "trip_cancelled_empty",
		Args:       []string{trip.RouteName, start.Format("15:04")},
		Data:       map[string]string{"type": "trip_cancelled_empty", "trip_id": trip.TripID.String(), "tenant_slug": tenant.Slug},
		CollapseID: "cancel:" + trip.TripID.String(),
		Channel:    "trip",
	}
}

// TripsCancelEmptyHandler is the TaskTripsCancelEmpty handler: the pass over whatever tenants exist
// when it fires.
func TripsCancelEmptyHandler(db coreServices.DatabaseService, tenants *coreJobs.TenantDirectory, pusher interfaces.Pusher) asynq.HandlerFunc {
	return func(ctx context.Context, _ *asynq.Task) error {
		list, err := tenants.List(ctx)
		if err != nil {
			return err
		}
		failed := 0
		for _, r := range RunTripsCancelEmptyPass(ctx, db, pusher, list) {
			if r.Err != nil {
				log.Printf("⚠️  trips cancel empty %s: %v", r.Tenant.ID, r.Err)
				failed++
			}
		}
		if failed > 0 {
			return fmt.Errorf("trips cancel empty failed for %d of %d tenants", failed, len(list))
		}
		return nil
	}
}
