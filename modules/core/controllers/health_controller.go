package controllers

import (
	"context"
	"net/http"
	"time"

	coreServices "josex/web/modules/core/services"

	"github.com/gin-gonic/gin"
)

// readyTimeout caps the database ping so a hung database answers /readyz with a
// 503 instead of holding the orchestrator's healthcheck open until it times out.
const readyTimeout = 2 * time.Second

// HealthController serves the unauthenticated liveness and readiness probes.
type HealthController struct {
	dbService coreServices.DatabaseService
	valkey    coreServices.ValkeyService
	// needsValkey is true for the worker and scheduler roles, which have nothing to do without it.
	needsValkey bool
}

// NewHealthController takes the process's Valkey (nil when REDIS_URL is unset) and whether this
// role cannot work without it (INFRA-001 D5).
func NewHealthController(dbService coreServices.DatabaseService, valkey coreServices.ValkeyService, needsValkey bool) *HealthController {
	return &HealthController{dbService: dbService, valkey: valkey, needsValkey: needsValkey}
}

// Livez reports that the process is up. It never touches a dependency: a
// failing database must not make the orchestrator restart a healthy process.
//
// @Summary Liveness probe
// @Tags health
// @Produce json
// @Success 200 {object} map[string]string
// @Router /livez [get]
func (hc *HealthController) Livez(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// Readyz reports that the process can serve traffic — that is, that its primary
// pool answers. InitDatabase connects in the background and retries forever, so
// a nil pool means "still starting", which is not ready either.
//
// Valkey down is a 503 only for a role that needs it (worker, scheduler). The api
// role answers 200 "degraded": REST keeps working and the rate limit fails open,
// so taking the replica out of rotation would only turn a degradation into an
// outage (INFRA-001 D5).
//
// @Summary Readiness probe
// @Tags health
// @Produce json
// @Success 200 {object} map[string]string
// @Failure 503 {object} map[string]string
// @Router /readyz [get]
func (hc *HealthController) Readyz(c *gin.Context) {
	pool := hc.dbService.GetPrimaryPool()
	if pool == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable", "reason": "database not connected"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), readyTimeout)
	defer cancel()

	if err := pool.Ping(ctx); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable", "reason": "database unreachable"})
		return
	}

	if hc.valkey != nil {
		if err := hc.valkey.Ping(ctx); err != nil {
			if hc.needsValkey {
				c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable", "reason": "valkey unreachable"})
				return
			}
			c.JSON(http.StatusOK, gin.H{"status": "degraded", "reason": "valkey unreachable"})
			return
		}
	} else if hc.needsValkey {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable", "reason": "valkey not configured"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
