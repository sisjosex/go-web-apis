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
//
// needsValkey marks a role that cannot work without Valkey (worker, scheduler):
// for it an unreachable Valkey makes /readyz a 503 instead of "degraded".
func RegisterHealthRoutes(r *gin.Engine, dbService coreServices.DatabaseService, valkey coreServices.ValkeyService, needsValkey bool) {
	healthController := coreControllers.NewHealthController(dbService, valkey, needsValkey)

	r.GET("/livez", healthController.Livez)
	r.GET("/readyz", healthController.Readyz)
}
