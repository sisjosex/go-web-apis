package interfaces

import (
	"context"

	"josex/web/modules/tenancy/models"

	"github.com/google/uuid"
)

// ModuleRepository handles database operations for tenant modules.
type ModuleRepository interface {
	GetTenantModules(ctx context.Context, tenantID uuid.UUID) ([]models.TenantModule, error)
	UpsertTenantModule(ctx context.Context, tenantID uuid.UUID, moduleCode string, dto models.UpsertTenantModuleDto, enabledBy uuid.UUID) (*models.TenantModule, error)
	// EnableWithRequirements turns on requirements, then moduleCode, in one transaction.
	EnableWithRequirements(ctx context.Context, tenantID uuid.UUID, requirements []string, moduleCode string, dto models.UpsertTenantModuleDto, enabledBy uuid.UUID) (*models.TenantModule, error)
	DisableTenantModule(ctx context.Context, tenantID uuid.UUID, moduleCode string) error
}
