package routes

import (
	"josex/web/modules/tenancy/controllers"

	"github.com/gin-gonic/gin"
)

// RegisterPlatformRoutes mounts tenancy's cross-tenant reads on the platform group, which already
// carries auth and super_admin and no tenant scope.
func RegisterPlatformRoutes(platform *gin.RouterGroup, usage *controllers.PlatformUsageController) {
	// What each business used per day (TRACK-013 D3).
	platform.GET("/usage", usage.ListUsage)
}
