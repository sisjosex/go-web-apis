package middleware

import (
	"fmt"
	"josex/web/modules/auth/services"
	"josex/web/modules/core/errors"
	"net/http"
	"strings"

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

		// Set system_role if present (backward compatible with old tokens)
		if systemRole, ok := claims["system_role"].(string); ok {
			c.Set("system_role", systemRole)
		} else {
			c.Set("system_role", "user") // Default for old tokens
		}

		c.Next()
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
