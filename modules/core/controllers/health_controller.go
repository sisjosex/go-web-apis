package controllers

import (
	"context"
	"net/http"
	"time"

	"josex/web/modules/core/api"
	coreServices "josex/web/modules/core/services"
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

// Probe is what /livez and /readyz answer: ok, degraded or unavailable, and why when not ok.
type Probe struct {
	Status string `json:"status" enum:"ok,degraded,unavailable"`
	Reason string `json:"reason,omitempty"`
}

// Livez reports that the process is up. It never touches a dependency: a
// failing database must not make the orchestrator restart a healthy process.
func (hc *HealthController) Livez(ctx context.Context, _ *api.None) (*api.Out[Probe], error) {
	return api.OK(Probe{Status: "ok"})
}

// Readyz reports that the process can serve traffic — that is, that its primary
// pool answers. InitDatabase connects in the background and retries forever, so
// a nil pool means "still starting", which is not ready either.
//
// Valkey down is a 503 only for a role that needs it (worker, scheduler). The api
// role answers 200 "degraded": REST keeps working and the rate limit fails open,
// so taking the replica out of rotation would only turn a degradation into an
// outage (INFRA-001 D5).
func (hc *HealthController) Readyz(ctx context.Context, _ *api.None) (*api.Reply[Probe], error) {
	c := api.Gin(ctx)
	pool := hc.dbService.GetPrimaryPool()
	if pool == nil {
		return api.Respond(http.StatusServiceUnavailable, Probe{Status: "unavailable", Reason: "database not connected"})
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), readyTimeout)
	defer cancel()

	if err := pool.Ping(ctx); err != nil {
		return api.Respond(http.StatusServiceUnavailable, Probe{Status: "unavailable", Reason: "database unreachable"})
	}

	if hc.valkey != nil {
		err := hc.valkey.Ping(ctx)
		// The healthcheck probes every 15 s, so this is the Valkey gauge of every scraped role.
		coreServices.SetUp(coreServices.DependencyUp.WithLabelValues("valkey"), err == nil)
		if err != nil {
			if hc.needsValkey {
				return api.Respond(http.StatusServiceUnavailable, Probe{Status: "unavailable", Reason: "valkey unreachable"})
			}
			return api.Respond(http.StatusOK, Probe{Status: "degraded", Reason: "valkey unreachable"})
		}
	} else if hc.needsValkey {
		return api.Respond(http.StatusServiceUnavailable, Probe{Status: "unavailable", Reason: "valkey not configured"})
	}

	return api.Respond(http.StatusOK, Probe{Status: "ok"})
}
