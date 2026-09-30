package jobs

import (
	"context"
	"fmt"
	"log"
	"log/slog"

	coreJobs "josex/web/modules/core/jobs"
	coreServices "josex/web/modules/core/services"
	trackingRepos "josex/web/modules/tracking/repositories"

	"github.com/hibiken/asynq"
)

const (
	// TaskPositionsPartitions keeps every tenant's vehicle_positions partitions a week ahead and drops
	// the ones past TRACKING_LOCATION_RETENTION_DAYS (TRACK-010 D1).
	TaskPositionsPartitions = "tracking:positions-partitions"
	// PositionsPartitionsCron is 01:00 UTC: the partitions are UTC days.
	PositionsPartitionsCron = "0 1 * * *"
	// positionsAheadDays is how many days of partitions exist ahead of today: a week of scheduler
	// outage before a point falls into the default partition.
	positionsAheadDays = 7
)

// PositionsPartitionsHandler runs sp_positions_partitions once per tenant. A tenant that fails is
// logged and the walk continues; the task fails, and asynq retries it, only if one did.
func PositionsPartitionsHandler(db coreServices.DatabaseService, tenants coreJobs.TenantLister, retentionDays int) asynq.HandlerFunc {
	return func(ctx context.Context, _ *asynq.Task) error {
		list, err := tenants(ctx)
		if err != nil {
			return err
		}
		repo := trackingRepos.NewTrackingRepository(db)
		failed := 0
		for _, t := range list {
			tenantCtx := context.WithValue(ctx, coreServices.TenantDatabaseURLKey, t.DatabaseURL)
			created, dropped, err := repo.PositionsPartitions(tenantCtx, positionsAheadDays, retentionDays)
			if err != nil {
				log.Printf("⚠️  positions partitions %s: %v", t.ID, err)
				failed++
				continue
			}
			if created > 0 || dropped > 0 {
				slog.Debug("positions partitions", "tenant", t.ID, "created", created, "dropped", dropped)
			}
		}
		if failed > 0 {
			return fmt.Errorf("positions partitions failed for %d of %d tenants", failed, len(list))
		}
		return nil
	}
}
