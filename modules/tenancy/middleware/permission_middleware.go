package middleware

import (
	"net/http"

	coreErrors "josex/web/modules/core/errors"
	tenancyErrors "josex/web/modules/tenancy/errors"
	"josex/web/modules/tenancy/models"

	"github.com/gin-gonic/gin"
)

// RequirePermission checks that the current user has at least one of the given
// permission codes. Must be placed AFTER TenantMiddleware.
//
// owner, admin, and super_admin roles bypass permission checks automatically.
//
// Usage:
//
//	api.POST("/products",
//	    tenancyMW.RequirePermission(inventoryPerms.ProductsWrite),
//	    productController.Create)
func RequirePermission(permissions ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw, exists := c.Get("tenant_access")
		if !exists {
			c.JSON(http.StatusForbidden, coreErrors.BuildErrorSingle(c, tenancyErrors.TenantUserUnauthorized))
			c.Abort()
			return
		}

		access, ok := raw.(*models.TenantAccessInfo)
		if !ok {
			c.JSON(http.StatusInternalServerError, coreErrors.BuildErrorSingle(c, tenancyErrors.TenantUserUnauthorized))
			c.Abort()
			return
		}

		for _, perm := range permissions {
			if access.HasPermission(perm) {
				c.Next()
				return
			}
		}

		c.JSON(http.StatusForbidden, coreErrors.BuildErrorSingle(c, tenancyErrors.PermissionDenied))
		c.Abort()
	}
}
