package jobs

import (
	"context"
	"fmt"
	"log"
	"log/slog"

	coreJobs "josex/web/modules/core/jobs"
	coreServices "josex/web/modules/core/services"
	trackingRepos "josex/web/modules/tracking/repositories"

	"github.com/google/uuid"
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
	// stopPairSampleDays is how far back the ETA's stop-to-stop median reads (TRACK-066 D1).
	stopPairSampleDays = 90
)

// PositionsPartitionsHandler runs sp_positions_partitions once per database — partitions belong to the
// table, which tenants sharing a database share (INFRA-009 D4) — then drops each of its tenants'
// stop-to-stop samples the ETA no longer reads. A database that fails is logged and the
// walk continues; the task fails, and asynq retries it, only if one did.
func PositionsPartitionsHandler(db coreServices.DatabaseService, tenants *coreJobs.TenantDirectory, retentionDays int) asynq.HandlerFunc {
	return func(ctx context.Context, _ *asynq.Task) error {
		dbs, err := tenants.Databases(ctx)
		if err != nil {
			return err
		}
		repo := trackingRepos.NewTrackingRepository(db)
		failed := 0
		for _, d := range dbs {
			dbCtx := context.WithValue(ctx, coreServices.TenantDatabaseURLKey, d.URL)
			created, dropped, err := repo.PositionsPartitions(dbCtx, positionsAheadDays, retentionDays)
			if err != nil {
				log.Printf("⚠️  positions partitions (tenant %s's database): %v", d.Tenants[0].ID, err)
				failed++
				continue
			}
			if created > 0 || dropped > 0 {
				slog.Debug("positions partitions", "tenant", d.Tenants[0].ID, "created", created, "dropped", dropped)
			}
			if err := pruneStopPairSamples(dbCtx, repo, d.Tenants); err != nil {
				log.Printf("⚠️  stop pair samples prune (tenant %s's database): %v", d.Tenants[0].ID, err)
				failed++
			}
		}
		if failed > 0 {
			return fmt.Errorf("positions partitions failed for %d of %d databases", failed, len(dbs))
		}
		return nil
	}
}

// pruneStopPairSamples drops each tenant's samples past stopPairSampleDays (TRACK-066 D1), one
// indexed DELETE per tenant; the first failure ends the database's walk.
func pruneStopPairSamples(ctx context.Context, repo *trackingRepos.TrackingRepository, tenants []coreJobs.Tenant) error {
	for _, t := range tenants {
		tenantID, err := uuid.Parse(t.ID)
		if err != nil {
			return err
		}
		pruned, err := repo.PruneStopPairSamples(ctx, tenantID, stopPairSampleDays)
		if err != nil {
			return err
		}
		if pruned > 0 {
			slog.Debug("stop pair samples pruned", "tenant", t.ID, "deleted", pruned)
		}
	}
	return nil
}
