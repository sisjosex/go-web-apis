package middleware

import (
	"context"
	"net/http"

	"josex/web/config"
	coreErrors "josex/web/modules/core/errors"
	coreServices "josex/web/modules/core/services"
	tenancyErrors "josex/web/modules/tenancy/errors"
	"josex/web/modules/tenancy/interfaces"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

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

		// Store tenant access info in gin context for controllers
		c.Set("tenant_id", tenantAccess.TenantID.String())
		c.Set("tenant_slug", tenantAccess.Slug)
		c.Set("tenant_database_url", tenantAccess.DatabaseURL)
		c.Set("tenant_schema_name", tenantAccess.SchemaName)
		c.Set("tenant_user_role", tenantAccess.UserRole)

		// Also inject database URL into the standard request context so
		// DatabaseService.resolvePool can pick it up automatically
		if tenantAccess.DatabaseURL != nil && *tenantAccess.DatabaseURL != "" {
			ctx := context.WithValue(c.Request.Context(), coreServices.TenantDatabaseURLKey, *tenantAccess.DatabaseURL)
			c.Request = c.Request.WithContext(ctx)
		}

		c.Next()
	}
}

// TenantMiddlewareFromHeaderOptional validates tenant access when tenant_slug is in X-Tenant-Slug header
// Used for APIs that support both single-database and multi-tenant operations
// - If X-Tenant-Slug is provided: validates user access to that tenant
// - If X-Tenant-Slug is NOT provided: allows operation on main database (admin/super_admin only)
func TenantMiddlewareFromHeaderOptional(tenantService interfaces.TenantService) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if multitenancy is enabled
		tenancyConf := config.ModularAppConfig.Tenancy
		if tenancyConf == nil || !tenancyConf.Enabled {
			c.JSON(http.StatusServiceUnavailable, coreErrors.BuildErrorSingle(c, tenancyErrors.MultitenancyDisabled))
			c.Abort()
			return
		}

		// Get system role from context (set by AuthMiddleware)
		systemRole, _ := c.Get("system_role")
		systemRoleStr := ""
		if sr, ok := systemRole.(string); ok {
			systemRoleStr = sr
		}

		// Get tenant slug from X-Tenant-Slug header
		tenantSlug := c.GetHeader("X-Tenant-Slug")

		// If no tenant slug provided, allow operation on main database for admins
		if tenantSlug == "" {
			// Super_admin and admin can operate on main database (global scope)
			if systemRoleStr == "super_admin" || systemRoleStr == "admin" {
				// Set nil tenant context to indicate global/main database scope
				c.Set("tenant_id", nil)
				c.Set("tenant_slug", "")
				c.Set("tenant_database_url", "")
				c.Set("tenant_schema_name", "")
				c.Set("tenant_user_role", "")
				c.Next()
				return
			}

			// Regular users MUST provide tenant
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

		// Store tenant access info in gin context for controllers
		c.Set("tenant_id", tenantAccess.TenantID.String())
		c.Set("tenant_slug", tenantAccess.Slug)
		c.Set("tenant_database_url", tenantAccess.DatabaseURL)
		c.Set("tenant_schema_name", tenantAccess.SchemaName)
		c.Set("tenant_user_role", tenantAccess.UserRole)

		// Also inject database URL into the standard request context
		if tenantAccess.DatabaseURL != nil && *tenantAccess.DatabaseURL != "" {
			ctx := context.WithValue(c.Request.Context(), coreServices.TenantDatabaseURLKey, *tenantAccess.DatabaseURL)
			c.Request = c.Request.WithContext(ctx)
		}

		c.Next()
	}
}

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
