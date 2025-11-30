package middleware

import (
	"net/http"

	"josex/web/config"
	coreErrors "josex/web/modules/core/errors"
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

		// Store tenant access info in context for use in controllers
		c.Set("tenant_id", tenantAccess.TenantID.String())
		c.Set("tenant_slug", tenantAccess.Slug)
		c.Set("tenant_database_url", tenantAccess.DatabaseURL)
		c.Set("tenant_schema_name", tenantAccess.SchemaName)
		c.Set("tenant_user_role", tenantAccess.UserRole)

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
