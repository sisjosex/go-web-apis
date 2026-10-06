package controllers

import (
	"errors"
	"net/http"
	"sort"

	"josex/web/config"
	coreErrors "josex/web/modules/core/errors"
	"josex/web/modules/core/registry"
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

// ListAvailableModules returns the modules this deployment runs (ENABLED_MODULES), each with what it
// requires and complements, and the business types the onboarding offers (TENANCY-003 D1) — a type
// keeps only the modules this deployment runs, and one left with users alone is not offered.
func (mc *ModuleController) ListAvailableModules(c *gin.Context) {
	coreConf := config.ModularAppConfig.Core
	all := registry.GetAll()

	available := make([]registry.ModuleDescriptor, 0, len(all))
	for _, m := range all {
		if coreConf.IsModuleEnabled(m.Code) {
			available = append(available, m)
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

	c.JSON(http.StatusOK, gin.H{"modules": available, "verticals": verticals})
}

// moduleWriteError answers a module write's refusal: 409 naming the modules that still need it, else 400.
func moduleWriteError(c *gin.Context, err error) {
	var requiredBy *tenancyErrors.ModuleRequiredByError
	if errors.As(err, &requiredBy) {
		c.JSON(http.StatusConflict, coreErrors.BuildErrorDetail(c, tenancyErrors.ModuleRequiredBy, gin.H{"required_by": requiredBy.Dependents}))
		return
	}
	c.JSON(http.StatusBadRequest, coreErrors.BuildError(c, err))
}

// GetTenantModules returns the list of modules enabled for a tenant.
// Requires tenant context from TenantMiddleware.
func (mc *ModuleController) GetTenantModules(c *gin.Context) {
	tenantIDStr, exists := c.Get("tenant_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, tenancyErrors.TenantUserUnauthorized))
		return
	}
	tenantID, err := uuid.Parse(tenantIDStr.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildError(c, err))
		return
	}

	tenantModules, err := mc.moduleService.GetTenantModules(c.Request.Context(), tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}

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

	c.JSON(http.StatusOK, result)
}

// UpsertTenantModule enables or updates the configuration for a module in a tenant.
// Requires owner or admin role.
func (mc *ModuleController) UpsertTenantModule(c *gin.Context) {
	tenantIDStr, _ := c.Get("tenant_id")
	tenantID, err := uuid.Parse(tenantIDStr.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildError(c, err))
		return
	}

	userIDStr, _ := c.Get("user_id")
	userID, err := uuid.Parse(userIDStr.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildError(c, err))
		return
	}

	moduleCode := c.Param("module_code")

	if _, ok := registry.Get(moduleCode); !ok {
		c.JSON(http.StatusNotFound, coreErrors.BuildErrorSingle(c, tenancyErrors.ModuleNotFound))
		return
	}

	var dto models.UpsertTenantModuleDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildError(c, err))
		return
	}

	module, err := mc.moduleService.UpsertTenantModule(c.Request.Context(), tenantID, moduleCode, dto, userID)
	if err != nil {
		moduleWriteError(c, err)
		return
	}

	// What is on now, requirements included, so the client updates its list without a second read.
	enabled, err := mc.moduleService.GetTenantEnabledModuleCodes(c.Request.Context(), tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	codes := make([]string, 0, len(enabled))
	for code := range enabled {
		codes = append(codes, code)
	}
	sort.Strings(codes)

	c.JSON(http.StatusOK, gin.H{
		"module_code": module.ModuleCode,
		"is_enabled":  module.IsEnabled,
		"config":      module.Config,
		"enabled_at":  module.EnabledAt,
		"enabled":     codes,
	})
}

// DisableTenantModule disables a module for a tenant.
// Requires owner or admin role.
func (mc *ModuleController) DisableTenantModule(c *gin.Context) {
	tenantIDStr, _ := c.Get("tenant_id")
	tenantID, err := uuid.Parse(tenantIDStr.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildError(c, err))
		return
	}

	moduleCode := c.Param("module_code")

	if err := mc.moduleService.DisableTenantModule(c.Request.Context(), tenantID, moduleCode); err != nil {
		moduleWriteError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}
