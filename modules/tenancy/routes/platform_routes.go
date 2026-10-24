package routes

import (
	"josex/web/modules/core/api"
	"josex/web/modules/tenancy/controllers"

	"github.com/gin-gonic/gin"
)

// RegisterPlatformRoutes mounts tenancy's cross-tenant reads on the platform group, which already
// carries auth and super_admin and no tenant scope.
func RegisterPlatformRoutes(platform *gin.RouterGroup, usage *controllers.PlatformUsageController) {
	// What each business used per day (TRACK-013 D3).
	api.Get(platform, "/usage", usage.ListUsage, api.Doc{Summary: "What each business used per day", Description: "Per business: its days between from and to (default the last 30, at most 366) with vehicles active (a trip completed or a GPS point), trips completed, GPS points and riders boarded, plus the range's peak and mean active vehicles. super_admin only", Errors: []int{400, 403}})
}
