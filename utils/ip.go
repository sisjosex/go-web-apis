package utils

import (
	"strings"

	"github.com/gin-gonic/gin"
)

func GetClientIp(c *gin.Context) string {
	forwardedFor := c.GetHeader("X-Forwarded-For")
	if forwardedFor != "" {
		// If the header contains multiple IPs, the first one is usually the original IP
		ipParts := strings.Split(forwardedFor, ",")
		return strings.TrimSpace(ipParts[0])
	}
	// If there is no X-Forwarded-For, use the client's IP
	return c.ClientIP()
}
