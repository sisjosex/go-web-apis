package controllers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"time"

	"josex/web/config"
	"josex/web/modules/core/api"
	coreErrors "josex/web/modules/core/errors"
	"josex/web/modules/core/registry"
	"josex/web/modules/core/utils"
	tenancyErrors "josex/web/modules/tenancy/errors"
	"josex/web/modules/tenancy/interfaces"
	"josex/web/modules/tenancy/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ModuleController handles tenant module management endpoints.
type ModuleController struct {
	moduleService interfaces.ModuleService
}

// NewModuleController creates a new ModuleController.
func NewModuleController(moduleService interfaces.ModuleService) *ModuleController {
	return &ModuleController{moduleService: moduleService}
}

// ListAvailableModulesResponse is ListAvailableModules's answer.
type ListAvailableModulesResponse struct {
	Modules   []registry.ModuleDescriptor `json:"modules"`
	Verticals []registry.Vertical         `json:"verticals"`
}

// ListAvailableModules returns the modules this deployment runs (ENABLED_MODULES), each with what it
// requires and complements, and the business types the onboarding offers (TENANCY-003 D1) — a type
// keeps only the modules this deployment runs, and one left with users alone is not offered.
func (mc *ModuleController) ListAvailableModules(ctx context.Context, _ *api.None) (*api.Out[ListAvailableModulesResponse], error) {
	c := api.Gin(ctx)
	coreConf := config.ModularAppConfig.Core
	all := registry.GetAll()

	lang := c.GetString("lang")
	available := make([]registry.ModuleDescriptor, 0, len(all))
	for _, m := range all {
		if coreConf.IsModuleEnabled(m.Code) {
			available = append(available, localized(lang, m))
		}
	}

	verticals := make([]registry.Vertical, 0, len(registry.Verticals()))
	for _, v := range registry.Verticals() {
		codes := make([]string, 0, len(v.Modules))
		business := false
		for _, code := range v.Modules {
			if coreConf.IsModuleEnabled(code) {
				codes = append(codes, code)
				business = business || code != "users"
			}
		}
		if business {
			verticals = append(verticals, registry.Vertical{Code: v.Code, Name: v.Name, Modules: codes})
		}
	}

	return api.OK(ListAvailableModulesResponse{Modules: available, Verticals: verticals})
}

// localized answers the descriptor's description and features in lang (TENANCY-004 D2), from the owning
// module's lang file (`module.<code>.description`, `module.<code>.feature.<i>`); the registry's English
// is the fallback. Map lookups only, no I/O.
func localized(lang string, m registry.ModuleDescriptor) registry.ModuleDescriptor {
	m.Description = translateOr(lang, "module."+m.Code+".description", m.Description)
	features := make([]string, len(m.Features))
	for i, feature := range m.Features {
		features[i] = translateOr(lang, fmt.Sprintf("module.%s.feature.%d", m.Code, i), feature)
	}
	m.Features = features
	return m
}

func translateOr(lang, key, fallback string) string {
	if text := utils.GetTranslation(lang, key); text != key {
		return text
	}
	return fallback
}

// moduleWriteError answers a module write's refusal: 409 for a locked module or one others still need,
// else 400.
func moduleWriteError(c *gin.Context, err error) error {
	var requiredBy *tenancyErrors.ModuleRequiredByError
	if errors.As(err, &requiredBy) {
		return api.Fail(http.StatusConflict, coreErrors.BuildErrorDetail(c, tenancyErrors.ModuleRequiredBy, gin.H{"required_by": requiredBy.Dependents}))
	}
	var locked *tenancyErrors.ModuleLockedError
	if errors.As(err, &locked) {
		return api.Fail(http.StatusConflict, coreErrors.BuildErrorSingle(c, tenancyErrors.ModuleLocked))
	}
	return api.Fail(http.StatusBadRequest, coreErrors.BuildError(c, err))
}

// EnrichedModule is a tenant's module with its catalogue entry, localized.
type EnrichedModule struct {
	models.TenantModuleResponse
	Name        string   `json:"name,omitempty"`
	Description string   `json:"description,omitempty"`
	Icon        string   `json:"icon,omitempty"`
	Route       string   `json:"route,omitempty"`
	Category    string   `json:"category,omitempty"`
	Features    []string `json:"features,omitempty"`
	AdminOnly   bool     `json:"admin_only,omitempty"`
}

// GetTenantModules returns the list of modules enabled for a tenant.
// Requires tenant context from TenantMiddleware.
func (mc *ModuleController) GetTenantModules(ctx context.Context, _ *api.None) (*api.Out[[]EnrichedModule], error) {
	c := api.Gin(ctx)
	tenantIDStr, exists := c.Get("tenant_id")
	if !exists {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, tenancyErrors.TenantUserUnauthorized))
	}
	tenantID, err := uuid.Parse(tenantIDStr.(string))
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildError(c, err))
	}

	tenantModules, err := mc.moduleService.GetTenantModules(c.Request.Context(), tenantID)
	if err != nil {
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}

	lang := c.GetString("lang")
	result := make([]EnrichedModule, 0, len(tenantModules))
	for _, m := range tenantModules {
		em := EnrichedModule{
			TenantModuleResponse: models.TenantModuleResponse{
				ModuleCode: m.ModuleCode,
				IsEnabled:  m.IsEnabled,
				Config:     m.Config,
				EnabledAt:  m.EnabledAt,
			},
		}
		if desc, ok := registry.Get(m.ModuleCode); ok {
			desc = localized(lang, desc)
			em.Name = desc.Name
			em.Description = desc.Description
			em.Icon = desc.Icon
			em.Route = desc.Route
			em.Category = desc.Category
			em.Features = desc.Features
			em.AdminOnly = desc.AdminOnly
		}
		result = append(result, em)
	}

	return api.OK(result)
}

// UpsertTenantModuleInput is UpsertTenantModule's request.
type UpsertTenantModuleInput struct {
	ModuleCode string `path:"module_code"`
	JSON       api.Body[models.UpsertTenantModuleDto]
}

// UpsertTenantModuleResponse is UpsertTenantModule's answer.
type UpsertTenantModuleResponse struct {
	ModuleCode string    `json:"module_code"`
	IsEnabled  bool      `json:"is_enabled"`
	Config     string    `json:"config"`
	EnabledAt  time.Time `json:"enabled_at"`
	Enabled    []string  `json:"enabled"`
}

// UpsertTenantModule enables or updates the configuration for a module in a tenant.
// Requires owner or admin role.
func (mc *ModuleController) UpsertTenantModule(ctx context.Context, in *UpsertTenantModuleInput) (*api.Out[UpsertTenantModuleResponse], error) {
	c := api.Gin(ctx)
	tenantIDStr, _ := c.Get("tenant_id")
	tenantID, err := uuid.Parse(tenantIDStr.(string))
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildError(c, err))
	}

	userIDStr, _ := c.Get("user_id")
	userID, err := uuid.Parse(userIDStr.(string))
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildError(c, err))
	}

	moduleCode := in.ModuleCode

	if _, ok := registry.Get(moduleCode); !ok {
		return nil, api.Fail(http.StatusNotFound, coreErrors.BuildErrorSingle(c, tenancyErrors.ModuleNotFound))
	}

	var dto models.UpsertTenantModuleDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildError(c, err))
	}

	module, err := mc.moduleService.UpsertTenantModule(c.Request.Context(), tenantID, moduleCode, dto, userID)
	if err != nil {
		return nil, moduleWriteError(c, err)
	}

	// What is on now, requirements included, so the client updates its list without a second read.
	enabled, err := mc.moduleService.GetTenantEnabledModuleCodes(c.Request.Context(), tenantID)
	if err != nil {
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	codes := make([]string, 0, len(enabled))
	for code := range enabled {
		codes = append(codes, code)
	}
	sort.Strings(codes)

	return api.OK(UpsertTenantModuleResponse{ModuleCode: module.ModuleCode, IsEnabled: module.IsEnabled, Config: module.Config, EnabledAt: module.EnabledAt, Enabled: codes})
}

// DisableTenantModuleInput is DisableTenantModule's request.
type DisableTenantModuleInput struct {
	ModuleCode string `path:"module_code"`
}

// DisableTenantModuleResponse is DisableTenantModule's answer.
type DisableTenantModuleResponse struct {
	Success bool `json:"success"`
}

// DisableTenantModule disables a module for a tenant.
// Requires owner or admin role.
func (mc *ModuleController) DisableTenantModule(ctx context.Context, in *DisableTenantModuleInput) (*api.Out[DisableTenantModuleResponse], error) {
	c := api.Gin(ctx)
	tenantIDStr, _ := c.Get("tenant_id")
	tenantID, err := uuid.Parse(tenantIDStr.(string))
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildError(c, err))
	}

	moduleCode := in.ModuleCode

	if err := mc.moduleService.DisableTenantModule(c.Request.Context(), tenantID, moduleCode); err != nil {
		return nil, moduleWriteError(c, err)
	}

	return api.OK(DisableTenantModuleResponse{Success: true})
}
