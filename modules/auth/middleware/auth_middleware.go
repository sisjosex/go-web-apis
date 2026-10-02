package middleware

import (
	"fmt"
	"josex/web/modules/auth/services"
	"josex/web/modules/core/errors"
	coreModels "josex/web/modules/core/models"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

func AuthMiddleware(jwtService services.JWTService) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString, err := extractToken(c)
		if err != nil {
			c.JSON(http.StatusUnauthorized, errors.BuildError(c, err))
			c.Abort()
			return
		}

		claims, err := jwtService.ValidateToken(tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, errors.BuildError(c, err))
			c.Abort()
			return
		}

		// Guardar datos en el contexto de la request
		c.Set("user_id", claims["user_id"])
		c.Set("session_id", claims["session_id"])
		// When the token stops being valid: a long-lived connection (the realtime socket) ends there.
		if exp, ok := claims["exp"].(float64); ok {
			c.Set("token_exp", time.Unix(int64(exp), 0))
		}

		// Set system_role if present (backward compatible with old tokens)
		if systemRole, ok := claims["system_role"].(string); ok {
			c.Set("system_role", systemRole)
		} else {
			c.Set("system_role", coreModels.SystemRoleUser) // Default for old tokens
		}

		c.Next()
	}
}

// RequireSystemRole blocks requests whose JWT system_role is not in the allowed list.
// Must be placed after AuthMiddleware.
func RequireSystemRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		systemRole, _ := c.Get("system_role")
		roleStr, _ := systemRole.(string)
		for _, r := range roles {
			if roleStr == r {
				c.Next()
				return
			}
		}
		c.JSON(http.StatusForbidden, errors.BuildErrorSingle(c, "auth.insufficient-permissions"))
		c.Abort()
	}
}

func extractToken(c *gin.Context) (string, error) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		return "", fmt.Errorf("authorization header is missing")
	}

	// Verificar el esquema 'Bearer'
	tokenString := strings.TrimSpace(strings.Replace(authHeader, "Bearer", "", 1))
	if tokenString == "" {
		return "", fmt.Errorf("invalid authorization header format")
	}

	return tokenString, nil
}
