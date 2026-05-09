package interfaces

import (
	"context"

	"josex/web/modules/tenancy/models"

	"github.com/google/uuid"
)

// ModuleService handles business logic for tenant modules.
type ModuleService interface {
	GetTenantModules(ctx context.Context, tenantID uuid.UUID) ([]models.TenantModule, error)
	GetTenantEnabledModuleCodes(ctx context.Context, tenantID uuid.UUID) (map[string]bool, error)
	UpsertTenantModule(ctx context.Context, tenantID uuid.UUID, moduleCode string, dto models.UpsertTenantModuleDto, enabledBy uuid.UUID) (*models.TenantModule, error)
	DisableTenantModule(ctx context.Context, tenantID uuid.UUID, moduleCode string) error
}
