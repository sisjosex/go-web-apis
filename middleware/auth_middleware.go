package middleware

import (
	"fmt"
	"josex/web/config"
	"josex/web/utils"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// AuthMiddleware es el middleware que verifica el AccessToken
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString, err := extractToken(c)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			c.Abort()
			return
		}

		claims, err := utils.ParseAccessToken(tokenString, config.AppConfig.JwtSecretKey)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			c.Abort()
			return
		}

		// Guardar claims en el contexto
		c.Set("user_id", claims["user_id"])
		c.Set("session_id", claims["session_id"])

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
