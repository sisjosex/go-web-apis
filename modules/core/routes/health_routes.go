package routes

import (
	"josex/web/modules/core/api"
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

	api.Get(&r.RouterGroup, "/livez", healthController.Livez, api.Doc{Summary: "Liveness probe"})
	api.Get(&r.RouterGroup, "/readyz", healthController.Readyz, api.Doc{Summary: "Readiness probe", Description: "200 ok or degraded (Valkey down where it is optional), 503 unavailable."})
}
