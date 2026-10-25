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
	// SetVerticals applies a business type preset (TENANCY-006): the union of the types' modules, the
	// types and main module stored, their capabilities and, with dto.Starter, the starter categories.
	SetVerticals(ctx context.Context, tenantID uuid.UUID, slug string, dto models.SetVerticalsDto, lang string, enabledBy uuid.UUID) (*models.VerticalsResponse, error)
	// TenantSetup answers the first-run checklist (TENANCY-008): shared steps around the enabled modules' own.
	TenantSetup(ctx context.Context, tenantID uuid.UUID) (*models.TenantSetupResponse, error)
}
