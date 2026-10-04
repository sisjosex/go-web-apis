package config

import (
	"time"

	"josex/web/modules/core/utils"
)

// GeoConfig holds the geo module's configuration (INFRA-003 D5).
type GeoConfig struct {
	// ValhallaURL is the routing service: the compose `valhalla` in prod, :8002 on the host in dev.
	ValhallaURL string
	// Timeout bounds one Valhalla call; a slow router counts as a failure toward the breaker.
	Timeout time.Duration
	// FallbackSpeedKmh turns a straight-line distance into a bus ETA when Valhalla is down.
	FallbackSpeedKmh int
	// VroomURL is the route optimizer (TRACK-014 D1): the compose `vroom` in prod, :3000 on the host in dev.
	VroomURL string
	// OptimizeTimeout bounds one solve; 1000 riders take seconds to about a minute.
	OptimizeTimeout time.Duration
}

// LoadGeoConfig loads configuration from environment variables
func LoadGeoConfig() *GeoConfig {
	return &GeoConfig{
		ValhallaURL:      utils.GetEnv("GEO_VALHALLA_URL", "http://127.0.0.1:8002"),
		Timeout:          utils.GetEnvAsDuration("GEO_TIMEOUT", 5*time.Second),
		FallbackSpeedKmh: utils.GetEnvAsInt("GEO_FALLBACK_SPEED_KMH", 25),
		VroomURL:         utils.GetEnv("GEO_VROOM_URL", "http://127.0.0.1:3000"),
		OptimizeTimeout:  utils.GetEnvAsDuration("GEO_OPTIMIZE_TIMEOUT", 120*time.Second),
	}
}
