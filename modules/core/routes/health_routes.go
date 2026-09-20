package routes

import (
	coreControllers "josex/web/modules/core/controllers"
	coreServices "josex/web/modules/core/services"

	"github.com/gin-gonic/gin"
)

// RegisterHealthRoutes mounts /livez and /readyz on the engine root — no
// /api/v1 prefix, no auth, no tenant. It is called before the CORS, rate limit
// and language middleware are installed so a probe is never rate-limited away
// and never depends on the middleware stack being healthy.
func RegisterHealthRoutes(r *gin.Engine, dbService coreServices.DatabaseService) {
	healthController := coreControllers.NewHealthController(dbService)

	r.GET("/livez", healthController.Livez)
	r.GET("/readyz", healthController.Readyz)
}
