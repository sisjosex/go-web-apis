package services

import (
	"context"

	"josex/web/modules/tenancy/interfaces"
	"josex/web/modules/tenancy/models"

	"github.com/google/uuid"
)

type moduleService struct {
	moduleRepository interfaces.ModuleRepository
}

// NewModuleService creates a new ModuleService.
func NewModuleService(moduleRepository interfaces.ModuleRepository) interfaces.ModuleService {
	return &moduleService{moduleRepository: moduleRepository}
}

func (s *moduleService) GetTenantModules(ctx context.Context, tenantID uuid.UUID) ([]models.TenantModule, error) {
	return s.moduleRepository.GetTenantModules(ctx, tenantID)
}

func (s *moduleService) GetTenantEnabledModuleCodes(ctx context.Context, tenantID uuid.UUID) (map[string]bool, error) {
	modules, err := s.moduleRepository.GetTenantModules(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	result := make(map[string]bool, len(modules))
	for _, m := range modules {
		if m.IsEnabled {
			result[m.ModuleCode] = true
		}
	}
	return result, nil
}

func (s *moduleService) UpsertTenantModule(ctx context.Context, tenantID uuid.UUID, moduleCode string, dto models.UpsertTenantModuleDto, enabledBy uuid.UUID) (*models.TenantModule, error) {
	return s.moduleRepository.UpsertTenantModule(ctx, tenantID, moduleCode, dto, enabledBy)
}

func (s *moduleService) DisableTenantModule(ctx context.Context, tenantID uuid.UUID, moduleCode string) error {
	return s.moduleRepository.DisableTenantModule(ctx, tenantID, moduleCode)
}
