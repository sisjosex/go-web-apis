package middleware

import (
	"net/http"

	coreErrors "josex/web/modules/core/errors"

	"github.com/gin-gonic/gin"
)

// RequireSystemRole middleware ensures user has required global system role
// Reads system_role from JWT claims (set by AuthMiddleware)
// Does NOT query database - role is trusted from signed JWT
func RequireSystemRole(allowedRoles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get system_role from context (set by AuthMiddleware from JWT)
		systemRoleRaw, exists := c.Get("system_role")
		if !exists {
			// If system_role not in context, user has old JWT (before migration)
			// Default to 'user' role for backward compatibility
			systemRoleRaw = "user"
		}

		systemRole, ok := systemRoleRaw.(string)
		if !ok {
			c.JSON(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, coreErrors.UserUnauthorized))
			c.Abort()
			return
		}

		// Check if user's role is in allowed roles
		allowed := false
		for _, role := range allowedRoles {
			if systemRole == role {
				allowed = true
				break
			}
		}

		if !allowed {
			c.JSON(http.StatusForbidden, coreErrors.BuildErrorSingle(c, coreErrors.UserInsufficientPermissions))
			c.Abort()
			return
		}

		c.Next()
	}
}
