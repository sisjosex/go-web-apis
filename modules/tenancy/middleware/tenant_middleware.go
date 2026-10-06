package middleware

import (
	"context"
	"net/http"

	"josex/web/config"
	coreErrors "josex/web/modules/core/errors"
	coreModels "josex/web/modules/core/models"
	coreServices "josex/web/modules/core/services"
	tenancyErrors "josex/web/modules/tenancy/errors"
	"josex/web/modules/tenancy/interfaces"
	"josex/web/modules/tenancy/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// LoadTenantModules enriches the TenantAccessInfo with enabled module codes.
// Must be placed after TenantMiddleware or TenantMiddlewareFromHeader.
func LoadTenantModules(moduleService interfaces.ModuleService) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantAccess, exists := c.Get("tenant_access")
		if !exists {
			c.Next()
			return
		}
		ta, ok := tenantAccess.(*models.TenantAccessInfo)
		if !ok {
			c.Next()
			return
		}

		codes, err := moduleService.GetTenantEnabledModuleCodes(c.Request.Context(), ta.TenantID)
		if err == nil {
			ta.EnabledModules = codes
			c.Set("tenant_access", ta)
		}

		c.Next()
	}
}

// ModuleCheck is LoadTenantModules and RequireModule as one check for TenantMiddlewareFromHeaderThen:
// the tenant's modules from the cache, then 403 tenant.module.not-enabled unless code is one of them.
func ModuleCheck(moduleService interfaces.ModuleService, code string) func(*gin.Context) bool {
	return func(c *gin.Context) bool {
		ta, ok := c.Value("tenant_access").(*models.TenantAccessInfo)
		if ok {
			if codes, err := moduleService.GetTenantEnabledModuleCodes(c.Request.Context(), ta.TenantID); err == nil {
				ta.EnabledModules = codes
			}
		}
		if !ok || !ta.HasModule(code) {
			c.AbortWithStatusJSON(http.StatusForbidden, coreErrors.BuildErrorSingle(c, tenancyErrors.ModuleNotEnabled))
			return false
		}
		return true
	}
}

// RequireModule checks that the current tenant has the specified module enabled.
// Must be placed after TenantMiddleware or TenantMiddlewareFromHeader.
func RequireModule(moduleCode string) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantAccess, exists := c.Get("tenant_access")
		if !exists {
			c.JSON(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, tenancyErrors.TenantUserUnauthorized))
			c.Abort()
			return
		}
		ta, ok := tenantAccess.(*models.TenantAccessInfo)
		if !ok || !ta.HasModule(moduleCode) {
			c.JSON(http.StatusForbidden, coreErrors.BuildErrorSingle(c, tenancyErrors.ModuleNotEnabled))
			c.Abort()
			return
		}
		c.Next()
	}
}

// TenantMiddleware validates tenant access and injects tenant context
// This middleware should be applied AFTER AuthMiddleware
func TenantMiddleware(tenantService interfaces.TenantService) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if multitenancy is enabled
		tenancyConf := config.ModularAppConfig.Tenancy
		if tenancyConf == nil || !tenancyConf.Enabled {
			c.JSON(http.StatusServiceUnavailable, coreErrors.BuildErrorSingle(c, tenancyErrors.MultitenancyDisabled))
			c.Abort()
			return
		}

		// Get tenant slug from URL parameter
		tenantSlug := c.Param("tenant_slug")
		if tenantSlug == "" {
			c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, tenancyErrors.TenantSlugRequired))
			c.Abort()
			return
		}

		// Get user ID from context (set by AuthMiddleware)
		userIDStr, exists := c.Get("user_id")
		if !exists {
			c.JSON(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, tenancyErrors.TenantUserUnauthorized))
			c.Abort()
			return
		}

		userID, err := uuid.Parse(userIDStr.(string))
		if err != nil {
			c.JSON(http.StatusUnauthorized, coreErrors.BuildError(c, err))
			c.Abort()
			return
		}

		// Verify user has access to this tenant
		tenantAccess, err := tenantService.VerifyUserTenantAccess(c.Request.Context(), userID, tenantSlug)
		if err != nil {
			c.JSON(http.StatusForbidden, coreErrors.BuildError(c, err))
			c.Abort()
			return
		}

		// Check if tenant is active
		if !tenantAccess.IsActive {
			c.JSON(http.StatusForbidden, coreErrors.BuildErrorSingle(c, tenancyErrors.TenantInactive))
			c.Abort()
			return
		}

		// Check if tenant is suspended
		if tenantAccess.IsSuspended {
			c.JSON(http.StatusForbidden, coreErrors.BuildErrorSingle(c, tenancyErrors.TenantSuspended))
			c.Abort()
			return
		}

		// A system super_admin bypasses permission checks regardless of tenant role
		if systemRole, ok := c.Get("system_role"); ok {
			tenantAccess.IsSuperAdmin = systemRole == coreModels.SystemRoleSuperAdmin
		}

		// Store tenant access info in gin context for controllers and RequirePermission
		c.Set("tenant_id", tenantAccess.TenantID.String())
		c.Set("tenant_slug", tenantAccess.Slug)
		c.Set("tenant_database_url", tenantAccess.DatabaseURL)
		c.Set("tenant_schema_name", tenantAccess.SchemaName)
		c.Set("tenant_user_role", tenantAccess.UserRole)
		c.Set("tenant_access", tenantAccess)

		// Also inject database URL into the standard request context so
		// DatabaseService.resolvePool can pick it up automatically
		if tenantAccess.DatabaseURL != nil && *tenantAccess.DatabaseURL != "" {
			ctx := context.WithValue(c.Request.Context(), coreServices.TenantDatabaseURLKey, *tenantAccess.DatabaseURL)
			c.Request = c.Request.WithContext(ctx)
		}

		c.Next()
	}
}

// TenantMiddlewareFromHeader validates tenant access from the X-Tenant-Slug header.
// X-Tenant-Slug is always required — no bypass for any role.
//
// - super_admin: can switch to ANY tenant; membership in tenant_users is not required.
// - All other roles: must be a member of the requested tenant (verified via tenant_users).
//
// allowMobileRoles names the mobile-only access levels this chain lets through (TRACK-017 D1).
// Empty — the production default — keeps the blanket refusal of TRACK-015 D2; a module that serves
// a mobile client builds a second chain naming the level it serves, so the exception is read at the
// route rather than from a path list in here.
func TenantMiddlewareFromHeader(tenantService interfaces.TenantService, allowMobileRoles ...string) gin.HandlerFunc {
	return TenantMiddlewareFromHeaderThen(tenantService, nil, allowMobileRoles...)
}

// TenantMiddlewareFromHeaderThen is TenantMiddlewareFromHeader with one more check — then — run once
// the tenant is resolved and before the handlers; then aborts and answers false to stop the request.
// It exists because a middleware that calls c.Next() cannot be wrapped in a closure with another: the
// handlers would run inside the first call, before the second check (TENANCY-003 D2).
func TenantMiddlewareFromHeaderThen(tenantService interfaces.TenantService, then func(*gin.Context) bool, allowMobileRoles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if multitenancy is enabled
		tenancyConf := config.ModularAppConfig.Tenancy
		if tenancyConf == nil || !tenancyConf.Enabled {
			c.JSON(http.StatusServiceUnavailable, coreErrors.BuildErrorSingle(c, tenancyErrors.MultitenancyDisabled))
			c.Abort()
			return
		}

		// X-Tenant-Slug is always required
		tenantSlug := c.GetHeader("X-Tenant-Slug")
		if tenantSlug == "" {
			c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, tenancyErrors.TenantSlugRequired))
			c.Abort()
			return
		}

		// Get user ID from context (set by AuthMiddleware)
		userIDStr, exists := c.Get("user_id")
		if !exists {
			c.JSON(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, tenancyErrors.TenantUserUnauthorized))
			c.Abort()
			return
		}

		userID, err := uuid.Parse(userIDStr.(string))
		if err != nil {
			c.JSON(http.StatusUnauthorized, coreErrors.BuildError(c, err))
			c.Abort()
			return
		}

		systemRoleStr, _ := c.Get("system_role")

		tenantAccess := resolveTenantAccess(c, tenantService, tenantSlug, userID,
			systemRoleStr == coreModels.SystemRoleSuperAdmin)
		if tenantAccess == nil {
			return
		}

		// A system super_admin bypasses permission checks regardless of tenant role
		tenantAccess.IsSuperAdmin = systemRoleStr == coreModels.SystemRoleSuperAdmin

		// A portal guardian and a driver are mobile-only accounts (TRACK-015 D2, TRACK-006 D1). The
		// level is already on the access row, so refusing it here costs nothing and covers every web
		// tenant route at once.
		if models.IsMobileOnlyTenantRole(tenantAccess.UserRole) && !roleAllowed(tenantAccess.UserRole, allowMobileRoles) {
			c.JSON(http.StatusForbidden, coreErrors.BuildErrorSingle(c, tenancyErrors.TenantUserPortalWebForbidden))
			c.Abort()
			return
		}

		// Check if tenant is active
		if !tenantAccess.IsActive {
			c.JSON(http.StatusForbidden, coreErrors.BuildErrorSingle(c, tenancyErrors.TenantInactive))
			c.Abort()
			return
		}

		// Check if tenant is suspended
		if tenantAccess.IsSuspended {
			c.JSON(http.StatusForbidden, coreErrors.BuildErrorSingle(c, tenancyErrors.TenantSuspended))
			c.Abort()
			return
		}

		// Inject tenant context for controllers, downstream middleware, and RequirePermission
		c.Set("tenant_id", tenantAccess.TenantID.String())
		c.Set("tenant_slug", tenantAccess.Slug)
		c.Set("tenant_database_url", tenantAccess.DatabaseURL)
		c.Set("tenant_schema_name", tenantAccess.SchemaName)
		c.Set("tenant_user_role", tenantAccess.UserRole)
		c.Set("tenant_access", tenantAccess)

		// Inject DB URL into request context so DatabaseService.resolvePool picks it up
		if tenantAccess.DatabaseURL != nil && *tenantAccess.DatabaseURL != "" {
			ctx := context.WithValue(c.Request.Context(), coreServices.TenantDatabaseURLKey, *tenantAccess.DatabaseURL)
			c.Request = c.Request.WithContext(ctx)
		}

		if then != nil && !then(c) {
			return
		}
		c.Next()
	}
}

// resolveTenantAccess answers the access row this request runs under: a super_admin switches to any
// tenant without a membership, everyone else must be a member of the one they asked for. It writes
// its own refusal and aborts, returning nil, when the tenant is unreachable either way.
func resolveTenantAccess(
	c *gin.Context,
	tenantService interfaces.TenantService,
	tenantSlug string,
	userID uuid.UUID,
	isSuperAdmin bool,
) *models.TenantAccessInfo {
	if isSuperAdmin {
		tenant, err := tenantService.GetTenantBySlug(c.Request.Context(), tenantSlug)
		if err != nil {
			c.JSON(http.StatusNotFound, coreErrors.BuildErrorSingle(c, tenancyErrors.TenantNotFound))
			c.Abort()
			return nil
		}
		return &models.TenantAccessInfo{
			TenantID:     tenant.ID,
			Slug:         tenant.Slug,
			Name:         tenant.Name,
			DatabaseURL:  tenant.DatabaseURL,
			SchemaName:   tenant.SchemaName,
			IsActive:     tenant.IsActive,
			IsSuspended:  tenant.IsSuspended,
			UserRole:     coreModels.SystemRoleSuperAdmin,
			UserIsActive: true,
			Permissions:  make(map[string]bool),
		}
	}

	tenantAccess, err := tenantService.VerifyUserTenantAccess(c.Request.Context(), userID, tenantSlug)
	if err != nil {
		c.JSON(http.StatusForbidden, coreErrors.BuildError(c, err))
		c.Abort()
		return nil
	}
	return tenantAccess
}

// roleAllowed reports whether role is one of allowed. The slice is a route-registration constant of
// at most one element, so the scan costs nothing per request.
func roleAllowed(role string, allowed []string) bool {
	for _, r := range allowed {
		if role == r {
			return true
		}
	}
	return false
}

// TenantMiddlewareFromHeaderOptional is kept as an alias for backward compatibility.
// Prefer TenantMiddlewareFromHeader for new code.
var TenantMiddlewareFromHeaderOptional = TenantMiddlewareFromHeader

// RequireTenantRole middleware ensures user has specific role in tenant
func RequireTenantRole(allowedRoles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userRole, exists := c.Get("tenant_user_role")
		if !exists {
			c.JSON(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, tenancyErrors.TenantUserUnauthorized))
			c.Abort()
			return
		}

		roleStr := userRole.(string)
		allowed := false
		for _, role := range allowedRoles {
			if roleStr == role {
				allowed = true
				break
			}
		}

		if !allowed {
			c.JSON(http.StatusForbidden, coreErrors.BuildErrorSingle(c, tenancyErrors.TenantUserUnauthorized))
			c.Abort()
			return
		}

		c.Next()
	}
}

// DenyTenantRole refuses the named access levels on this route. It is the counterpart of
// RequireTenantRole for the handful of endpoints a level must not reach at all — an organization
// user has no business in the operator's fleet, however its assigned roles are configured. A system
// super_admin passes, as it does everywhere else.
func DenyTenantRole(deniedRoles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if systemRole, ok := c.Get("system_role"); ok && systemRole == coreModels.SystemRoleSuperAdmin {
			c.Next()
			return
		}

		userRole, exists := c.Get("tenant_user_role")
		if !exists {
			c.JSON(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, tenancyErrors.TenantUserUnauthorized))
			c.Abort()
			return
		}

		for _, role := range deniedRoles {
			if userRole == role {
				c.JSON(http.StatusForbidden, coreErrors.BuildErrorSingle(c, tenancyErrors.TenantUserUnauthorized))
				c.Abort()
				return
			}
		}

		c.Next()
	}
}
