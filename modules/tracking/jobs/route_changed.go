package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	coreJobs "josex/web/modules/core/jobs"
	coreServices "josex/web/modules/core/services"
	trackingRepos "josex/web/modules/tracking/repositories"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

// topicRouteChanged is what an SP writes to tracking.outbox whenever a route's plan changes: its stop
// list (TRACK-007), its schedules, calendar or exceptions (TRACK-018), its riders (TRACK-008).
const topicRouteChanged = "route.changed"

// TaskRouteChanged is the outbox task one of those rows becomes.
var TaskRouteChanged = coreJobs.OutboxTask(topicRouteChanged)

// RouteChanged is one such row: which route changed, and over which days. DateTo empty means "and
// every day after" — a published version has no end until the next one closes it.
type RouteChanged struct {
	RouteID  string `json:"route_id"`
	DateFrom string `json:"date_from"`
	DateTo   string `json:"date_to"`
}

// ApplyRouteChanged rebuilds the not-started trips of one route over the days a route.changed row
// names — clamped by the SP to the route's local today..today+14. It is the handler's whole job, kept
// apart so it can be called without an asynq context.
func ApplyRouteChanged(ctx context.Context, db coreServices.DatabaseService, tenant coreJobs.Tenant, payload RouteChanged) (int, error) {
	tenantID, err := uuid.Parse(tenant.ID)
	if err != nil {
		return 0, err
	}
	routeID, err := uuid.Parse(payload.RouteID)
	if err != nil {
		return 0, err
	}
	from, err := optionalDate(payload.DateFrom)
	if err != nil {
		return 0, err
	}
	to, err := optionalDate(payload.DateTo)
	if err != nil {
		return 0, err
	}
	tenantCtx := context.WithValue(ctx, coreServices.TenantDatabaseURLKey, tenant.DatabaseURL)
	return trackingRepos.NewTrackingRepository(db).MaterialiseTrips(tenantCtx, tenantID, &routeID, from, to)
}

// RouteChangedHandler is the TaskRouteChanged handler. The row carries no tenant — it lives in the
// tenant's own database — so the tenant comes from the TaskID the relay gave it, <tenant>:<id>.
// Delivery is at least once and the materialiser is idempotent, so a repeat is harmless.
func RouteChangedHandler(db coreServices.DatabaseService, tenants coreJobs.TenantLister) asynq.HandlerFunc {
	return func(ctx context.Context, task *asynq.Task) error {
		var payload RouteChanged
		if err := json.Unmarshal(task.Payload(), &payload); err != nil {
			return fmt.Errorf("route.changed payload: %v: %w", err, asynq.SkipRetry)
		}
		taskID, _ := asynq.GetTaskID(ctx)
		tenantID, _, _ := strings.Cut(taskID, ":")
		list, err := tenants(ctx)
		if err != nil {
			return err
		}
		for _, tenant := range list {
			if tenant.ID == tenantID {
				_, err := ApplyRouteChanged(ctx, db, tenant, payload)
				return err
			}
		}
		// The tenant is gone: there is nothing left to rebuild.
		log.Printf("⚠️  route.changed %s: tenant %q is not listed, dropped", payload.RouteID, tenantID)
		return nil
	}
}

// optionalDate parses a YYYY-MM-DD payload date; empty is "no bound".
func optionalDate(value string) (*time.Time, error) {
	if value == "" {
		return nil, nil
	}
	date, err := time.Parse(time.DateOnly, value)
	if err != nil {
		return nil, fmt.Errorf("route.changed date %q: %v: %w", value, err, asynq.SkipRetry)
	}
	return &date, nil
}
