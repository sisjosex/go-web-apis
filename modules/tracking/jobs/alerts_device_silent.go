package jobs

import (
	"context"
	"fmt"
	"log"

	coreJobs "josex/web/modules/core/jobs"
	coreServices "josex/web/modules/core/services"
	trackingRepos "josex/web/modules/tracking/repositories"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

const (
	// TaskAlertsDeviceSilent is the sweep for buses that stopped reporting during a trip (TRACK-071 D2).
	TaskAlertsDeviceSilent = "tracking:alerts-device-silent"
	// AlertsDeviceSilentCron is every minute: a threshold is whole minutes, 1 at the least.
	AlertsDeviceSilentCron = "* * * * *"
)

// RunDeviceSilentPass raises every tenant's device_silent alerts, as RunDocumentAlertsPass walks them:
// the SP writes the alert.changed rows itself, so the frames and the staff push leave from the worker.
// A tenant that fails is reported and the walk continues.
func RunDeviceSilentPass(ctx context.Context, db coreServices.DatabaseService, tenants []coreJobs.Tenant) []PassResult {
	repo := trackingRepos.NewTrackingRepository(db)
	results := make([]PassResult, 0, len(tenants))
	for _, tenant := range tenants {
		result := PassResult{Tenant: tenant}
		tenantID, err := uuid.Parse(tenant.ID)
		if err == nil {
			tenantCtx := context.WithValue(ctx, coreServices.TenantDatabaseURLKey, tenant.DatabaseURL)
			raised, raiseErr := repo.RaiseDeviceSilentAlerts(tenantCtx, tenantID)
			result.Claimed, err = len(raised), raiseErr
		}
		result.Err = err
		results = append(results, result)
	}
	return results
}

// DeviceSilentHandler is the TaskAlertsDeviceSilent handler: the pass over whatever tenants exist.
func DeviceSilentHandler(db coreServices.DatabaseService, tenants *coreJobs.TenantDirectory) asynq.HandlerFunc {
	return func(ctx context.Context, _ *asynq.Task) error {
		list, err := tenants.List(ctx)
		if err != nil {
			return err
		}
		failed := 0
		for _, r := range RunDeviceSilentPass(ctx, db, list) {
			if r.Err != nil {
				log.Printf("⚠️  device silent alerts %s: %v", r.Tenant.ID, r.Err)
				failed++
			}
		}
		if failed > 0 {
			return fmt.Errorf("device silent alerts failed for %d of %d tenants", failed, len(list))
		}
		return nil
	}
}
