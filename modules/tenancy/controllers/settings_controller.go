package controllers

import (
	"context"
	"net/http"

	"josex/web/modules/core/api"
	coreErrors "josex/web/modules/core/errors"
	"josex/web/modules/core/registry"
	tenancyErrors "josex/web/modules/tenancy/errors"
	"josex/web/modules/tenancy/interfaces"
	"josex/web/modules/tenancy/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// SettingsController serves a workspace's typed settings (TENANCY-007 D1): the keys its enabled
// modules declare in the registry, stored in tenancy.tenants.settings.
type SettingsController struct {
	tenantService interfaces.TenantService
	moduleService interfaces.ModuleService
}

// NewSettingsController creates a new SettingsController.
func NewSettingsController(tenantService interfaces.TenantService, moduleService interfaces.ModuleService) *SettingsController {
	return &SettingsController{tenantService: tenantService, moduleService: moduleService}
}

// GetSettings is GET /settings: every setting of the workspace's enabled modules, defaults filled.
func (sc *SettingsController) GetSettings(ctx context.Context, _ *api.None) (*api.Out[models.TenantSettingsResponse], error) {
	c := api.Gin(ctx)
	_, slug, specs, err := sc.scope(c)
	if err != nil {
		return nil, err
	}
	stored, err := sc.tenantService.TenantSettings(c.Request.Context(), slug)
	if err != nil {
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.OK(settingsResponse(specs, stored))
}

// UpdateSettingsInput is UpdateSettings's request: the keys to change and their new values.
type UpdateSettingsInput struct {
	JSON api.Body[map[string]any]
}

// UpdateSettings is PATCH /settings: validates every key and value, then merges them in one statement.
func (sc *SettingsController) UpdateSettings(ctx context.Context, in *UpdateSettingsInput) (*api.Out[models.TenantSettingsResponse], error) {
	c := api.Gin(ctx)
	tenantID, slug, specs, err := sc.scope(c)
	if err != nil {
		return nil, err
	}
	var patch map[string]any
	if err := in.JSON.Into(ctx, &patch); err != nil || len(patch) == 0 {
		return nil, api.Fail(http.StatusUnprocessableEntity, coreErrors.BuildErrorSingle(c, tenancyErrors.TenantSettingsInvalid))
	}
	byKey := make(map[string]registry.SettingSpec, len(specs))
	for _, s := range specs {
		byKey[s.Key] = s
	}
	for key, value := range patch {
		spec, ok := byKey[key]
		if !ok {
			return nil, api.Fail(http.StatusUnprocessableEntity, coreErrors.BuildErrorDetail(c, tenancyErrors.TenantSettingsUnknownKey, key))
		}
		if !spec.Accepts(value) {
			return nil, api.Fail(http.StatusUnprocessableEntity, coreErrors.BuildErrorDetail(c, tenancyErrors.TenantSettingsInvalid, key))
		}
	}
	stored, err := sc.tenantService.MergeSettings(c.Request.Context(), tenantID, slug, patch)
	if err != nil {
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.OK(settingsResponse(specs, stored))
}

// scope answers the request's workspace and the settings its enabled modules declare.
func (sc *SettingsController) scope(c *gin.Context) (uuid.UUID, string, []registry.SettingSpec, error) {
	tenantID, err := uuid.Parse(c.GetString("tenant_id"))
	if err != nil {
		return uuid.Nil, "", nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, tenancyErrors.TenantUserUnauthorized))
	}
	enabled, err := sc.moduleService.GetTenantEnabledModuleCodes(c.Request.Context(), tenantID)
	if err != nil {
		return uuid.Nil, "", nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return tenantID, c.GetString("tenant_slug"), registry.SettingsFor(enabled), nil
}

func settingsResponse(specs []registry.SettingSpec, stored map[string]any) models.TenantSettingsResponse {
	out := models.TenantSettingsResponse{Settings: make(map[string]any, len(specs)), Specs: make([]models.TenantSettingSpec, 0, len(specs))}
	for _, s := range specs {
		out.Settings[s.Key] = s.Value(stored)
		out.Specs = append(out.Specs, models.TenantSettingSpec{Key: s.Key, Module: s.Module, Type: string(s.Type), Options: s.Options})
	}
	return out
}
