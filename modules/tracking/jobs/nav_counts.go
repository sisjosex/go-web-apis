package jobs

import (
	"context"
	"encoding/json"

	coreJobs "josex/web/modules/core/jobs"
	coreServices "josex/web/modules/core/services"
	trackingRepos "josex/web/modules/tracking/repositories"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// NavChannel carries the rail's badges to the tenant's operators (TRACK-081): one low-volume channel
// the shell listens to on every page, apart from fleet:{tenant} and its position frames.
func NavChannel(tenantID string) string { return "nav:" + tenantID }

// FrameNavCounts is the frame the rail's badges go out as, the body of GET /tracking/nav-counts.
const FrameNavCounts = "nav_counts"

// navCountsMoved is the trip transitions that change how many trips are in progress.
var navCountsMoved = map[string]bool{"start": true, "complete": true, "cancel": true}

// publishNavCounts sends the tenant's counts on their own; valkey nil publishes nothing.
func publishNavCounts(ctx context.Context, db coreServices.DatabaseService, valkey coreServices.ValkeyService, tenant coreJobs.Tenant) error {
	if valkey == nil {
		return nil
	}
	pipe := valkey.Client().Pipeline()
	if err := queueNavCounts(ctx, db, pipe, tenant); err != nil {
		return err
	}
	_, err := pipe.Exec(ctx)
	return err
}

// queueNavCounts reads the tenant's counts once, unscoped — only operators listen to nav:{tenant} — and
// queues them on pipe: one query per change, whatever the number of open tabs.
func queueNavCounts(ctx context.Context, db coreServices.DatabaseService, pipe redis.Pipeliner, tenant coreJobs.Tenant) error {
	tenantID, err := uuid.Parse(tenant.ID)
	if err != nil {
		return err
	}
	tenantCtx := context.WithValue(ctx, coreServices.TenantDatabaseURLKey, tenant.DatabaseURL)
	counts, err := trackingRepos.NewTrackingRepository(db).NavCounts(tenantCtx, tenantID, nil)
	if err != nil {
		return err
	}
	data, err := json.Marshal(counts)
	if err != nil {
		return err
	}
	publishTo(ctx, pipe, []string{NavChannel(tenant.ID)}, Frame{Type: FrameNavCounts, Data: data})
	return nil
}
