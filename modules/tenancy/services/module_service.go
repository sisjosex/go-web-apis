package services

import (
	"context"
	"sync"
	"time"

	"josex/web/modules/core/registry"
	tenancyErrors "josex/web/modules/tenancy/errors"
	"josex/web/modules/tenancy/interfaces"
	"josex/web/modules/tenancy/models"

	"github.com/google/uuid"
)

// moduleCacheTTL bounds how long another replica may keep serving a module list changed elsewhere; in
// this process a change drops the entry at once (TENANCY-003 D2).
const moduleCacheTTL = 60 * time.Second

type cachedModules struct {
	codes map[string]bool
	until time.Time
}

// moduleCache holds each tenant's enabled modules for every module check in this process — one list
// per tenant per minute instead of one query per request. Package-level, so every ModuleService
// instance (the routes', the middleware's) reads and drops the same entries.
var moduleCache sync.Map // uuid.UUID → cachedModules

// InvalidateTenantModules drops a tenant's cached module list; the next check reads it again.
func InvalidateTenantModules(tenantID uuid.UUID) {
	moduleCache.Delete(tenantID)
}

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

// GetTenantEnabledModuleCodes answers from the cache while it is fresh. The map is shared: callers read it.
func (s *moduleService) GetTenantEnabledModuleCodes(ctx context.Context, tenantID uuid.UUID) (map[string]bool, error) {
	if entry, ok := moduleCache.Load(tenantID); ok {
		if cached := entry.(cachedModules); time.Now().Before(cached.until) {
			return cached.codes, nil
		}
	}
	codes, err := s.enabledCodes(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	moduleCache.Store(tenantID, cachedModules{codes: codes, until: time.Now().Add(moduleCacheTTL)})
	return codes, nil
}

// enabledCodes reads the list from the database, never the cache: a write decides on it.
func (s *moduleService) enabledCodes(ctx context.Context, tenantID uuid.UUID) (map[string]bool, error) {
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

// UpsertTenantModule enables a module together with what it requires (TENANCY-003 D1), or — with
// is_enabled false — disables it the way DisableTenantModule does.
func (s *moduleService) UpsertTenantModule(ctx context.Context, tenantID uuid.UUID, moduleCode string, dto models.UpsertTenantModuleDto, enabledBy uuid.UUID) (*models.TenantModule, error) {
	enabled, err := s.enabledCodes(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	if !dto.IsEnabled {
		if dependents := registry.RequiredBy(moduleCode, enabled); len(dependents) > 0 {
			return nil, &tenancyErrors.ModuleRequiredByError{Module: moduleCode, Dependents: dependents}
		}
	}

	var missing []string
	if dto.IsEnabled {
		for _, code := range registry.WithRequirements(moduleCode) {
			if code != moduleCode && !enabled[code] {
				missing = append(missing, code)
			}
		}
	}
	defer InvalidateTenantModules(tenantID)
	if len(missing) == 0 {
		return s.moduleRepository.UpsertTenantModule(ctx, tenantID, moduleCode, dto, enabledBy)
	}
	return s.moduleRepository.EnableWithRequirements(ctx, tenantID, missing, moduleCode, dto, enabledBy)
}

// DisableTenantModule refuses while an enabled module still requires this one.
func (s *moduleService) DisableTenantModule(ctx context.Context, tenantID uuid.UUID, moduleCode string) error {
	enabled, err := s.enabledCodes(ctx, tenantID)
	if err != nil {
		return err
	}
	if dependents := registry.RequiredBy(moduleCode, enabled); len(dependents) > 0 {
		return &tenancyErrors.ModuleRequiredByError{Module: moduleCode, Dependents: dependents}
	}
	defer InvalidateTenantModules(tenantID)
	return s.moduleRepository.DisableTenantModule(ctx, tenantID, moduleCode)
}
