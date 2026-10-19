package jobs

import (
	"context"
	"fmt"
	"log"
	"log/slog"

	coreJobs "josex/web/modules/core/jobs"
	coreServices "josex/web/modules/core/services"
	tenancyModels "josex/web/modules/tenancy/models"
	trackingRepos "josex/web/modules/tracking/repositories"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

const (
	// TaskContactRetention clears inactive riders' contact data past each tenant's retention and
	// prunes the access log (TRACK-063 D3).
	TaskContactRetention = "tracking:contact-retention"
	// ContactRetentionCron is 02:00 UTC, the quiet of the night in every zone the operators work in.
	ContactRetentionCron = "0 2 * * *"
	// contactRetentionSetting is the tenancy.tenants.settings key that overrides the default.
	contactRetentionSetting = "tracking.contact_retention_days"
)

// ContactRetentionHandler walks every tenant: its inactive riders past the retention — the tenant's
// setting, else defaultDays — lose their contact data, a guardian left with no rider loses its portal
// membership as RemoveRiderGuardian takes it, and log rows past logDays go. A tenant that fails is
// logged and the walk continues; the task fails, and asynq retries it, only if one did. A retry is
// harmless: an anonymised rider is not picked again.
func ContactRetentionHandler(db coreServices.DatabaseService, tenants *coreJobs.TenantDirectory, defaultDays, logDays int) asynq.HandlerFunc {
	return func(ctx context.Context, _ *asynq.Task) error {
		list, err := tenants.List(ctx)
		if err != nil {
			return err
		}
		repo := trackingRepos.NewTrackingRepository(db)
		failed := 0
		for _, tenant := range list {
			if err := retainTenant(ctx, repo, tenant, defaultDays, logDays); err != nil {
				log.Printf("⚠️  contact retention %s: %v", tenant.ID, err)
				failed++
			}
		}
		if failed > 0 {
			return fmt.Errorf("contact retention failed for %d of %d tenants", failed, len(list))
		}
		return nil
	}
}

func retainTenant(ctx context.Context, repo *trackingRepos.TrackingRepository, tenant coreJobs.Tenant, defaultDays, logDays int) error {
	tenantID, err := uuid.Parse(tenant.ID)
	if err != nil {
		return err
	}
	tenantCtx := context.WithValue(ctx, coreServices.TenantDatabaseURLKey, tenant.DatabaseURL)
	unlinked, err := repo.AnonymiseRiderContacts(tenantCtx, tenantID, retentionDays(tenant, defaultDays))
	if err != nil {
		return err
	}
	for _, userID := range unlinked {
		linked, err := repo.UserLinked(tenantCtx, tenantID, userID)
		if err != nil {
			return err
		}
		if !linked {
			if err := repo.RevokeAppAccess(tenantCtx, tenantID, userID, tenancyModels.RolePortal); err != nil {
				return err
			}
		}
	}
	pruned, err := repo.PruneRiderAccessLog(tenantCtx, tenantID, logDays)
	if err != nil {
		return err
	}
	if len(unlinked) > 0 || pruned > 0 {
		slog.Debug("contact retention", "tenant", tenant.ID, "unlinked", len(unlinked), "log_pruned", pruned)
	}
	return nil
}

// retentionDays is the tenant's own retention when its settings name a positive number of days.
func retentionDays(tenant coreJobs.Tenant, defaultDays int) int {
	if days, ok := tenant.Settings[contactRetentionSetting].(float64); ok && days >= 1 {
		return int(days)
	}
	return defaultDays
}
