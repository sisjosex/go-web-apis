package controllers

import (
	"net/http"

	"josex/web/config"
	coreErrors "josex/web/modules/core/errors"
	"josex/web/modules/core/utils"
	tenancyErrors "josex/web/modules/tenancy/errors"
	"josex/web/modules/tenancy/interfaces"
	"josex/web/modules/tenancy/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type TenantController struct {
	tenantService interfaces.TenantService
}

// NewTenantController creates a new instance of TenantController
func NewTenantController(tenantService interfaces.TenantService) *TenantController {
	return &TenantController{
		tenantService: tenantService,
	}
}

// CreateTenant godoc
// @Summary Create a new tenant (authenticated users)
// @Description Create a new tenant with optional custom database URL. Creator becomes owner automatically.
// @Tags Tenants
// @Accept json
// @Produce json
// @Param request body models.CreateTenantDto true "Tenant data"
// @Success 201 {object} models.TenantDetailResponse
// @Failure 400 {object} errors.ErrorResponse
// @Router /tenants [post]
// @Security ApiKeyAuth
func (tc *TenantController) CreateTenant(c *gin.Context) {
	// Check if multitenancy is enabled
	tenancyConf := config.ModularAppConfig.Tenancy
	if tenancyConf == nil || !tenancyConf.Enabled {
		c.JSON(http.StatusServiceUnavailable, coreErrors.BuildErrorSingle(c, tenancyErrors.MultitenancyDisabled))
		return
	}

	// Get user ID from context (set by AuthMiddleware)
	userIDStr, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, tenancyErrors.TenantUserUnauthorized))
		return
	}

	userID, err := uuid.Parse(userIDStr.(string))
	if err != nil {
		c.JSON(http.StatusUnauthorized, coreErrors.BuildError(c, err))
		return
	}

	var dto models.CreateTenantDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, tenancyErrors.TenantCreateFailed, utils.ExtractValidationError(c, err)))
		return
	}

	// Create tenant with user as owner
	tenant, err := tc.tenantService.CreateTenant(c.Request.Context(), &dto, userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusCreated, tenant)
}

// CreateTenantSelfService godoc
// @Summary Create tenant (self-service with subscription limits)
// @Description Allow authenticated users to create tenants with limits based on their subscription plan
// @Tags Tenants
// @Accept json
// @Produce json
// @Param request body models.CreateTenantDto true "Tenant data"
// @Success 201 {object} models.TenantDetailResponse
// @Failure 400 {object} errors.ErrorResponse
// @Failure 403 {object} errors.ErrorResponse
// @Router /tenants/self-service [post]
// @Security ApiKeyAuth
func (tc *TenantController) CreateTenantSelfService(c *gin.Context) {
	// Check if multitenancy is enabled
	tenancyConf := config.ModularAppConfig.Tenancy
	if tenancyConf == nil || !tenancyConf.Enabled {
		c.JSON(http.StatusServiceUnavailable, coreErrors.BuildErrorSingle(c, tenancyErrors.MultitenancyDisabled))
		return
	}

	// Check if self-service is enabled
	if !tenancyConf.AllowSelfService {
		c.JSON(http.StatusForbidden, coreErrors.BuildErrorSingle(c, tenancyErrors.TenantSelfServiceDisabled))
		return
	}

	// Get user info from context (set by AuthMiddleware)
	userIDStr, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, tenancyErrors.TenantUserUnauthorized))
		return
	}

	userID, err := uuid.Parse(userIDStr.(string))
	if err != nil {
		c.JSON(http.StatusUnauthorized, coreErrors.BuildError(c, err))
		return
	}

	// Get system role (super_admin bypasses limits)
	systemRoleRaw, _ := c.Get("system_role")
	systemRole := "user"
	if role, ok := systemRoleRaw.(string); ok {
		systemRole = role
	}

	// Super admins can create unlimited tenants
	if systemRole != "super_admin" {
		// Get subscription plan from JWT context
		// Note: In a real system, you'd query the database for the latest plan
		// For now, we'll use a simplified approach assuming plan is in user record

		// Count current owned tenants
		count, err := tc.tenantService.CountUserOwnedTenants(c.Request.Context(), userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
			return
		}

		// Determine limit based on plan (simplified - in production query from user record)
		limit := tenancyConf.FreePlanLimit // Default to free plan

		// Check if limit reached
		if limit > 0 && count >= limit {
			c.JSON(http.StatusForbidden, coreErrors.BuildErrorSingle(c, tenancyErrors.TenantLimitReached))
			return
		}
	}

	var dto models.CreateTenantDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, tenancyErrors.TenantCreateFailed, utils.ExtractValidationError(c, err)))
		return
	}

	// Self-service users cannot use custom database URLs (security)
	if systemRole != "super_admin" && dto.DatabaseURL != nil && *dto.DatabaseURL != "" {
		c.JSON(http.StatusForbidden, coreErrors.BuildErrorSingle(c, "tenant.database-url.not-allowed"))
		return
	}

	// Create tenant with user as owner
	tenant, err := tc.tenantService.CreateTenant(c.Request.Context(), &dto, userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusCreated, tenant)
}

// GetUserTenants godoc
// @Summary Get user's accessible tenants
// @Description Returns list of tenants that the authenticated user can access
// @Tags Tenants
// @Produce json
// @Success 200 {array} models.UserTenantResponse
// @Failure 401 {object} errors.ErrorResponse
// @Router /tenants/my-tenants [get]
// @Security ApiKeyAuth
func (tc *TenantController) GetUserTenants(c *gin.Context) {
	// Check if multitenancy is enabled
	tenancyConf := config.ModularAppConfig.Tenancy
	if tenancyConf == nil || !tenancyConf.Enabled {
		c.JSON(http.StatusServiceUnavailable, coreErrors.BuildErrorSingle(c, tenancyErrors.MultitenancyDisabled))
		return
	}

	userIDStr, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, tenancyErrors.TenantUserUnauthorized))
		return
	}

	userID, err := uuid.Parse(userIDStr.(string))
	if err != nil {
		c.JSON(http.StatusUnauthorized, coreErrors.BuildError(c, err))
		return
	}

	tenants, err := tc.tenantService.GetUserTenants(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusOK, tenants)
}

// UpdateTenant godoc
// @Summary Update tenant information (owner/admin only)
// @Description Update tenant name, database URL, schema, or settings
// @Tags Tenants
// @Accept json
// @Produce json
// @Param tenant_slug path string true "Tenant slug"
// @Param request body models.UpdateTenantDto true "Updated tenant data"
// @Success 200 {object} models.TenantDetailResponse
// @Failure 400 {object} errors.ErrorResponse
// @Router /tenants/{tenant_slug} [patch]
// @Security ApiKeyAuth
func (tc *TenantController) UpdateTenant(c *gin.Context) {
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

	var dto models.UpdateTenantDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, tenancyErrors.TenantUpdateFailed, utils.ExtractValidationError(c, err)))
		return
	}

	tenant, err := tc.tenantService.UpdateTenant(c.Request.Context(), tenantID, &dto)
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusOK, tenant)
}

// AddUserToTenant godoc
// @Summary Add a user to tenant (owner/admin only)
// @Description Add a user to the tenant with specified role
// @Tags Tenants
// @Accept json
// @Produce json
// @Param tenant_slug path string true "Tenant slug"
// @Param request body models.AddUserToTenantDto true "User and role data"
// @Success 200 {object} map[string]string
// @Failure 400 {object} errors.ErrorResponse
// @Router /tenants/{tenant_slug}/users [post]
// @Security ApiKeyAuth
func (tc *TenantController) AddUserToTenant(c *gin.Context) {
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

	// Get requester user ID from JWT context
	requesterUserIDStr, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, tenancyErrors.TenantUserUnauthorized))
		return
	}

	requesterUserID, err := uuid.Parse(requesterUserIDStr.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildError(c, err))
		return
	}

	var dto models.AddUserToTenantDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, tenancyErrors.TenantUserAddFailed, utils.ExtractValidationError(c, err)))
		return
	}

	err = tc.tenantService.AddUserToTenant(c.Request.Context(), tenantID, requesterUserID, &dto)
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "User added to tenant successfully"})
}

// RemoveUserFromTenant godoc
// @Summary Remove a user from tenant (owner/admin only)
// @Description Remove a user from the tenant
// @Tags Tenants
// @Accept json
// @Produce json
// @Param tenant_slug path string true "Tenant slug"
// @Param user_id path string true "User ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} errors.ErrorResponse
// @Router /tenants/{tenant_slug}/users/{user_id} [delete]
// @Security ApiKeyAuth
func (tc *TenantController) RemoveUserFromTenant(c *gin.Context) {
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

	// Get requester user ID from JWT context
	requesterUserIDStr, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, tenancyErrors.TenantUserUnauthorized))
		return
	}

	requesterUserID, err := uuid.Parse(requesterUserIDStr.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildError(c, err))
		return
	}

	userIDParam := c.Param("user_id")
	userID, err := uuid.Parse(userIDParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildError(c, err))
		return
	}

	err = tc.tenantService.RemoveUserFromTenant(c.Request.Context(), tenantID, requesterUserID, userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "User removed from tenant successfully"})
}

// RunTenantMigrations godoc
// @Summary Run migrations on tenant database
// @Description Execute all pending migrations on a tenant's custom database (admin/super_admin only)
// @Tags Tenants
// @Produce json
// @Param tenant_slug path string true "Tenant slug"
// @Success 200 {object} map[string]string
// @Failure 400 {object} errors.ErrorResponse
// @Failure 403 {object} errors.ErrorResponse
// @Failure 404 {object} errors.ErrorResponse
// @Router /tenants/{tenant_slug}/migrate [post]
// @Security ApiKeyAuth
func (tc *TenantController) RunTenantMigrations(c *gin.Context) {
	// Check system role (only super_admin or admin)
	systemRole, exists := c.Get("system_role")
	if !exists || (systemRole != "super_admin" && systemRole != "admin") {
		c.JSON(http.StatusForbidden, coreErrors.BuildErrorSingle(c, tenancyErrors.TenantUserInsufficientPermissions))
		return
	}

	tenantSlug := c.Param("tenant_slug")
	if tenantSlug == "" {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
		return
	}

	err := tc.tenantService.RunTenantMigrations(c.Request.Context(), tenantSlug)
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Migrations executed successfully",
		"tenant":  tenantSlug,
	})
}
