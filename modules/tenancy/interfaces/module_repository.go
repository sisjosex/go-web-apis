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
	DisableTenantModule(ctx context.Context, tenantID uuid.UUID, moduleCode string) error
}
