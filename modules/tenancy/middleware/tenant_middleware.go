package middleware

import (
	"context"
	"net/http"

	"josex/web/config"
	coreErrors "josex/web/modules/core/errors"
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
func TenantMiddlewareFromHeader(tenantService interfaces.TenantService) gin.HandlerFunc {
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

		var tenantAccess *models.TenantAccessInfo

		if systemRoleStr == "super_admin" {
			// super_admin can switch to any tenant — bypass membership check
			tenant, err := tenantService.GetTenantBySlug(c.Request.Context(), tenantSlug)
			if err != nil {
				c.JSON(http.StatusNotFound, coreErrors.BuildErrorSingle(c, tenancyErrors.TenantNotFound))
				c.Abort()
				return
			}
			tenantAccess = &models.TenantAccessInfo{
				TenantID:     tenant.ID,
				Slug:         tenant.Slug,
				Name:         tenant.Name,
				DatabaseURL:  tenant.DatabaseURL,
				SchemaName:   tenant.SchemaName,
				IsActive:     tenant.IsActive,
				IsSuspended:  tenant.IsSuspended,
				UserRole:     "super_admin",
				UserIsActive: true,
				Permissions:  make(map[string]bool),
			}
		} else {
			// Regular users must be members of the tenant
			tenantAccess, err = tenantService.VerifyUserTenantAccess(c.Request.Context(), userID, tenantSlug)
			if err != nil {
				c.JSON(http.StatusForbidden, coreErrors.BuildError(c, err))
				c.Abort()
				return
			}
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

		c.Next()
	}
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
