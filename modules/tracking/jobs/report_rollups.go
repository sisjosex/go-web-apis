package jobs

import (
	"context"
	"fmt"
	"log"
	"time"

	coreJobs "josex/web/modules/core/jobs"
	coreServices "josex/web/modules/core/services"
	trackingRepos "josex/web/modules/tracking/repositories"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

const (
	// TaskReportRollups rolls every tenant's report aggregates and yesterday's usage (TRACK-013 D1, D3).
	TaskReportRollups = "tracking:report-rollups"
	// ReportRollupsCron is 03:00 UTC: after the materialiser, and yesterday's UTC partition is closed.
	ReportRollupsCron = "0 3 * * *"
	// reportSweepDays is how far back the sweep looks for trips without a summary or changed after it.
	reportSweepDays = 3
	// stopDelayWeeks is the history the per-stop lateness is computed over.
	stopDelayWeeks = 8
)

// UsageWriter stores one tenant's day of usage in the platform database — tenancy.usage_daily.
type UsageWriter interface {
	UpsertUsageDaily(ctx context.Context, tenantID uuid.UUID, day time.Time, vehiclesActive, tripsCompleted int, positions int64, ridersBoarded int) error
}

// SummariseTrip rolls an ended trip's summary right after its trace (D1), so distance_km is in.
func SummariseTrip(ctx context.Context, db coreServices.DatabaseService, tenant coreJobs.Tenant, tripID uuid.UUID) error {
	tenantID, err := uuid.Parse(tenant.ID)
	if err != nil {
		return err
	}
	tenantCtx := context.WithValue(ctx, coreServices.TenantDatabaseURLKey, tenant.DatabaseURL)
	_, err = trackingRepos.NewTrackingRepository(db).SummariseTrip(tenantCtx, tenantID, tripID)
	return err
}

// RunReportRollupsPass walks the tenants: the summaries the trip close missed, the per-stop
// lateness, and yesterday's (UTC) usage counted in the tenant's database and written to the
// platform's. Every call is idempotent. A tenant that fails is reported and the walk continues.
func RunReportRollupsPass(ctx context.Context, db coreServices.DatabaseService, usage UsageWriter, tenants []coreJobs.Tenant, now time.Time) []error {
	repo := trackingRepos.NewTrackingRepository(db)
	today := now.UTC().Truncate(24 * time.Hour)
	yesterday := today.AddDate(0, 0, -1)
	errs := make([]error, len(tenants))
	for i, tenant := range tenants {
		errs[i] = rollTenant(ctx, repo, usage, tenant, today, yesterday)
	}
	return errs
}

func rollTenant(ctx context.Context, repo *trackingRepos.TrackingRepository, usage UsageWriter, tenant coreJobs.Tenant, today, yesterday time.Time) error {
	tenantID, err := uuid.Parse(tenant.ID)
	if err != nil {
		return err
	}
	tenantCtx := context.WithValue(ctx, coreServices.TenantDatabaseURLKey, tenant.DatabaseURL)
	if _, err := repo.SweepTripSummaries(tenantCtx, tenantID, today.AddDate(0, 0, -reportSweepDays)); err != nil {
		return fmt.Errorf("summaries: %w", err)
	}
	if _, err := repo.RollStopDelayStats(tenantCtx, tenantID, stopDelayWeeks); err != nil {
		return fmt.Errorf("stop delays: %w", err)
	}
	u, err := repo.UsageForDay(tenantCtx, tenantID, yesterday)
	if err != nil {
		return fmt.Errorf("usage: %w", err)
	}
	return usage.UpsertUsageDaily(ctx, tenantID, yesterday, u.VehiclesActive, u.TripsCompleted, u.Positions, u.RidersBoarded)
}

// ReportRollupsHandler is the TaskReportRollups handler: the pass over whatever tenants exist when it
// fires; the task fails when any tenant did, so asynq retries the lot.
func ReportRollupsHandler(db coreServices.DatabaseService, usage UsageWriter, tenants *coreJobs.TenantDirectory) asynq.HandlerFunc {
	return func(ctx context.Context, _ *asynq.Task) error {
		list, err := tenants.List(ctx)
		if err != nil {
			return err
		}
		failed := 0
		for i, err := range RunReportRollupsPass(ctx, db, usage, list, time.Now()) {
			if err != nil {
				log.Printf("⚠️  report rollups %s: %v", list[i].ID, err)
				failed++
			}
		}
		if failed > 0 {
			return fmt.Errorf("report rollups failed for %d of %d tenants", failed, len(list))
		}
		return nil
	}
}
