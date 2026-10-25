package interfaces

import (
	"context"

	"josex/web/modules/core/registry"
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
	// ApplyVerticals turns on w.Missing and stores the types, main module and capabilities in one
	// transaction; it answers the stored types, main module and settings (TENANCY-006).
	ApplyVerticals(ctx context.Context, tenantID uuid.UUID, w models.VerticalsWrite, enabledBy uuid.UUID) ([]string, *string, *string, error)
	// SeedCategories creates the starter categories on the tenant's own database and answers how many.
	SeedCategories(ctx context.Context, tenantID uuid.UUID, names []string) (int, error)
	// TenantSetup answers the checklist's shared steps and the stored business types (TENANCY-008).
	TenantSetup(ctx context.Context, tenantID uuid.UUID) (business, team bool, verticals []string, err error)
	// CheckSetup runs a module's checker on the request's database.
	CheckSetup(ctx context.Context, check registry.SetupChecker, tenantID uuid.UUID) (map[string]bool, error)
}
