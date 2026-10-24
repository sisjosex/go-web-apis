package controllers

import (
	"context"
	"errors"
	"net/http"

	"josex/web/config"
	billingInterfaces "josex/web/modules/billing/interfaces"
	"josex/web/modules/core/api"
	coreErrors "josex/web/modules/core/errors"
	coreModels "josex/web/modules/core/models"
	"josex/web/modules/core/utils"
	tenancyErrors "josex/web/modules/tenancy/errors"
	"josex/web/modules/tenancy/interfaces"
	"josex/web/modules/tenancy/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

type TenantController struct {
	tenantService  interfaces.TenantService
	billingService billingInterfaces.BillingService
}

// NewTenantController creates a new instance of TenantController
func NewTenantController(tenantService interfaces.TenantService, billingService billingInterfaces.BillingService) *TenantController {
	return &TenantController{
		tenantService:  tenantService,
		billingService: billingService,
	}
}

// CreateTenantInput is CreateTenant's request.
type CreateTenantInput struct {
	JSON api.Body[models.CreateTenantDto]
}

// CreateTenant is POST : Create a new tenant (authenticated users).
func (tc *TenantController) CreateTenant(ctx context.Context, in *CreateTenantInput) (*api.Out[*models.TenantDetailResponse], error) {
	c := api.Gin(ctx)
	// Check if multitenancy is enabled
	tenancyConf := config.ModularAppConfig.Tenancy
	if tenancyConf == nil || !tenancyConf.Enabled {
		return nil, api.Fail(http.StatusServiceUnavailable, coreErrors.BuildErrorSingle(c, tenancyErrors.MultitenancyDisabled))
	}

	// Get user ID from context (set by AuthMiddleware)
	userIDStr, exists := c.Get("user_id")
	if !exists {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, tenancyErrors.TenantUserUnauthorized))
	}

	userID, err := uuid.Parse(userIDStr.(string))
	if err != nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildError(c, err))
	}

	var dto models.CreateTenantDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, tenancyErrors.TenantCreateFailed, utils.ExtractValidationError(c, err)))
	}

	// Create tenant with user as owner
	tenant, err := tc.tenantService.CreateTenant(c.Request.Context(), &dto, userID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildError(c, err))
	}

	return api.OK(tenant)
}

// CreateTenantSelfServiceInput is CreateTenantSelfService's request.
type CreateTenantSelfServiceInput struct {
	JSON api.Body[models.CreateTenantDto]
}

// CreateTenantSelfService is POST /self-service: Create tenant (self-service with subscription limits).
func (tc *TenantController) CreateTenantSelfService(ctx context.Context, in *CreateTenantSelfServiceInput) (*api.Out[*models.TenantDetailResponse], error) {
	c := api.Gin(ctx)
	// Check if multitenancy is enabled
	tenancyConf := config.ModularAppConfig.Tenancy
	if tenancyConf == nil || !tenancyConf.Enabled {
		return nil, api.Fail(http.StatusServiceUnavailable, coreErrors.BuildErrorSingle(c, tenancyErrors.MultitenancyDisabled))
	}

	// Check if self-service is enabled
	if !tenancyConf.AllowSelfService {
		return nil, api.Fail(http.StatusForbidden, coreErrors.BuildErrorSingle(c, tenancyErrors.TenantSelfServiceDisabled))
	}

	// Get user info from context (set by AuthMiddleware)
	userIDStr, exists := c.Get("user_id")
	if !exists {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, tenancyErrors.TenantUserUnauthorized))
	}

	userID, err := uuid.Parse(userIDStr.(string))
	if err != nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildError(c, err))
	}

	// Get system role (super_admin bypasses limits)
	systemRoleRaw, _ := c.Get("system_role")
	systemRole := coreModels.SystemRoleUser
	if role, ok := systemRoleRaw.(string); ok {
		systemRole = role
	}

	// Super admins can create unlimited tenants
	if systemRole != coreModels.SystemRoleSuperAdmin {
		if err := tc.ensureTenantAllowance(c, userID); err != nil {
			return nil, err
		}
	}

	var dto models.CreateTenantDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, tenancyErrors.TenantCreateFailed, utils.ExtractValidationError(c, err)))
	}

	// Self-service users cannot use custom database URLs (security)
	if systemRole != coreModels.SystemRoleSuperAdmin && dto.DatabaseURL != nil && *dto.DatabaseURL != "" {
		return nil, api.Fail(http.StatusForbidden, coreErrors.BuildErrorSingle(c, "tenant.database-url.not-allowed"))
	}

	// Create tenant with user as owner
	tenant, err := tc.tenantService.CreateTenant(c.Request.Context(), &dto, userID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildError(c, err))
	}

	return api.OK(tenant)
}

// ensureTenantAllowance refuses with 403 when the user already owns as many workspaces as their
// active plan allows.
func (tc *TenantController) ensureTenantAllowance(c *gin.Context, userID uuid.UUID) error {
	count, err := tc.tenantService.CountUserOwnedTenants(c.Request.Context(), userID)
	if err != nil {
		return api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	limitReached, err := tc.billingService.IsLimitReached(c.Request.Context(), userID, "tenants", count)
	if err != nil {
		return api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	if limitReached {
		return api.Fail(http.StatusForbidden, coreErrors.BuildErrorSingle(c, tenancyErrors.TenantLimitReached))
	}
	return nil
}

// GetUserTenants is GET /me: Get user's accessible tenants.
func (tc *TenantController) GetUserTenants(ctx context.Context, _ *api.None) (*api.Out[[]*models.UserTenantResponse], error) {
	c := api.Gin(ctx)
	// Check if multitenancy is enabled
	tenancyConf := config.ModularAppConfig.Tenancy
	if tenancyConf == nil || !tenancyConf.Enabled {
		return nil, api.Fail(http.StatusServiceUnavailable, coreErrors.BuildErrorSingle(c, tenancyErrors.MultitenancyDisabled))
	}

	userIDStr, exists := c.Get("user_id")
	if !exists {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, tenancyErrors.TenantUserUnauthorized))
	}

	userID, err := uuid.Parse(userIDStr.(string))
	if err != nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildError(c, err))
	}

	tenants, err := tc.tenantService.GetUserTenants(c.Request.Context(), userID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildError(c, err))
	}

	return api.OK(tenants)
}

// UpdateTenantInput is UpdateTenant's request.
type UpdateTenantInput struct {
	JSON api.Body[models.UpdateTenantDto]
}

// UpdateTenant is PATCH : Update tenant information (owner/admin only).
func (tc *TenantController) UpdateTenant(ctx context.Context, in *UpdateTenantInput) (*api.Out[*models.TenantDetailResponse], error) {
	c := api.Gin(ctx)
	tenantIDStr, exists := c.Get("tenant_id")
	if !exists {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, tenancyErrors.TenantUserUnauthorized))
	}

	tenantID, err := uuid.Parse(tenantIDStr.(string))
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildError(c, err))
	}

	var dto models.UpdateTenantDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, tenancyErrors.TenantUpdateFailed, utils.ExtractValidationError(c, err)))
	}

	tenant, err := tc.tenantService.UpdateTenant(c.Request.Context(), tenantID, &dto)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildError(c, err))
	}

	return api.OK(tenant)
}

// AddUserToTenantInput is AddUserToTenant's request.
type AddUserToTenantInput struct {
	JSON api.Body[models.AddUserToTenantDto]
}

// AddUserToTenantResponse is AddUserToTenant's answer.
type AddUserToTenantResponse struct {
	Message string `json:"message"`
}

// AddUserToTenant is POST /users: Add a user to tenant (owner/admin only).
func (tc *TenantController) AddUserToTenant(ctx context.Context, in *AddUserToTenantInput) (*api.Out[AddUserToTenantResponse], error) {
	c := api.Gin(ctx)
	tenantIDStr, exists := c.Get("tenant_id")
	if !exists {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, tenancyErrors.TenantUserUnauthorized))
	}

	tenantID, err := uuid.Parse(tenantIDStr.(string))
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildError(c, err))
	}

	// Get requester user ID from JWT context
	requesterUserIDStr, exists := c.Get("user_id")
	if !exists {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, tenancyErrors.TenantUserUnauthorized))
	}

	requesterUserID, err := uuid.Parse(requesterUserIDStr.(string))
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildError(c, err))
	}

	var dto models.AddUserToTenantDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, tenancyErrors.TenantUserAddFailed, utils.ExtractValidationError(c, err)))
	}

	err = tc.tenantService.AddUserToTenant(c.Request.Context(), tenantID, requesterUserID, &dto)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildError(c, err))
	}

	return api.OK(AddUserToTenantResponse{Message: "User added to tenant successfully"})
}

// RemoveUserFromTenantInput is RemoveUserFromTenant's request.
type RemoveUserFromTenantInput struct {
	UserID string `path:"user_id"`
}

// RemoveUserFromTenantResponse is RemoveUserFromTenant's answer.
type RemoveUserFromTenantResponse struct {
	Message string `json:"message"`
}

// RemoveUserFromTenant is DELETE /users/{user_id}: Remove a user from tenant (owner/admin only).
func (tc *TenantController) RemoveUserFromTenant(ctx context.Context, in *RemoveUserFromTenantInput) (*api.Out[RemoveUserFromTenantResponse], error) {
	c := api.Gin(ctx)
	tenantIDStr, exists := c.Get("tenant_id")
	if !exists {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, tenancyErrors.TenantUserUnauthorized))
	}

	tenantID, err := uuid.Parse(tenantIDStr.(string))
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildError(c, err))
	}

	// Get requester user ID from JWT context
	requesterUserIDStr, exists := c.Get("user_id")
	if !exists {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, tenancyErrors.TenantUserUnauthorized))
	}

	requesterUserID, err := uuid.Parse(requesterUserIDStr.(string))
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildError(c, err))
	}

	userIDParam := in.UserID
	userID, err := uuid.Parse(userIDParam)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildError(c, err))
	}

	err = tc.tenantService.RemoveUserFromTenant(c.Request.Context(), tenantID, requesterUserID, userID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildError(c, err))
	}

	return api.OK(RemoveUserFromTenantResponse{Message: "User removed from tenant successfully"})
}

// UpdateUserRoleInput is UpdateUserRole's request.
type UpdateUserRoleInput struct {
	UserID string `path:"user_id"`
	JSON   api.Body[models.UpdateUserRoleDto]
}

// UpdateUserRole is PATCH /users/{user_id}/role: Update a user's role in the tenant (owner/admin only).
func (tc *TenantController) UpdateUserRole(ctx context.Context, in *UpdateUserRoleInput) (*api.Empty, error) {
	c := api.Gin(ctx)
	tenantIDStr, exists := c.Get("tenant_id")
	if !exists {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, tenancyErrors.TenantUserUnauthorized))
	}

	tenantID, err := uuid.Parse(tenantIDStr.(string))
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildError(c, err))
	}

	requesterIDStr, exists := c.Get("user_id")
	if !exists {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, tenancyErrors.TenantUserUnauthorized))
	}

	requesterID, err := uuid.Parse(requesterIDStr.(string))
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildError(c, err))
	}

	userID, err := uuid.Parse(in.UserID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
	}

	var dto models.UpdateUserRoleDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, tenancyErrors.TenantUserUpdateFailed, utils.ExtractValidationError(c, err)))
	}

	err = tc.tenantService.UpdateUserRole(c.Request.Context(), tenantID, requesterID, userID, dto.Role)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Message {
			case "tenant.user.not-found":
				return nil, api.Fail(http.StatusNotFound, coreErrors.BuildErrorSingle(c, tenancyErrors.TenantUserNotFound))
			case "tenant.user.invalid-role":
				return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, tenancyErrors.TenantUserInvalidRole))
			case "tenant.user.not-authorized", "tenant.user.insufficient-permissions":
				return nil, api.Fail(http.StatusForbidden, coreErrors.BuildErrorSingle(c, tenancyErrors.TenantUserInsufficientPermissions))
			case "tenant.user.cannot-change-own-role":
				return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, tenancyErrors.TenantUserUpdateFailed))
			}
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}

	return api.Done()
}

// RunTenantMigrationsInput is RunTenantMigrations's request.
type RunTenantMigrationsInput struct {
	TenantSlug string `path:"tenant_slug"`
}

// RunTenantMigrationsResponse is RunTenantMigrations's answer.
type RunTenantMigrationsResponse struct {
	Message string `json:"message"`
	Tenant  string `json:"tenant"`
}

// RunTenantMigrations is POST /migrate: Run migrations on tenant database.
func (tc *TenantController) RunTenantMigrations(ctx context.Context, in *RunTenantMigrationsInput) (*api.Out[RunTenantMigrationsResponse], error) {
	c := api.Gin(ctx)
	// Check system role (only super_admin or admin)
	systemRole, exists := c.Get("system_role")
	if !exists || (systemRole != coreModels.SystemRoleSuperAdmin && systemRole != models.RoleAdmin) {
		return nil, api.Fail(http.StatusForbidden, coreErrors.BuildErrorSingle(c, tenancyErrors.TenantUserInsufficientPermissions))
	}

	tenantSlug := in.TenantSlug
	if tenantSlug == "" {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
	}

	err := tc.tenantService.RunTenantMigrations(c.Request.Context(), tenantSlug)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildError(c, err))
	}

	return api.OK(RunTenantMigrationsResponse{Message: "Migrations executed successfully", Tenant: tenantSlug})
}
