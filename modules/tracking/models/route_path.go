package models

import (
	"fmt"
	"time"

	geoModels "josex/web/modules/geo/models"

	"github.com/google/uuid"
)

// The planned line of TRACK-028: a route version's path by streets, computed once when its stops
// change and read by every map.

// RoutePathQuery binds GET /tracking/routes/:route_id/path. Date picks the day whose version is
// wanted; unset is the route's local today.
type RoutePathQuery struct {
	Date *string `form:"date" binding:"omitempty,datetime=2006-01-02"`
}

// RoutePathLeg is one stop-to-stop stretch of the line.
type RoutePathLeg struct {
	DistanceM int `json:"distance_m"`
	DurationS int `json:"duration_s"`
}

// RoutePath is the stored line: precision 6, its metres, its legs in sequence, and whether it follows
// the streets (valhalla) or joins the stops straight (fallback).
type RoutePath struct {
	VersionID  uuid.UUID      `json:"version_id"`
	Polyline6  string         `json:"polyline6"`
	DistanceM  int            `json:"distance_m"`
	Legs       []RoutePathLeg `json:"legs"`
	Source     string         `json:"source"`
	ComputedAt time.Time      `json:"computed_at"`
}

// RoutePathResponse is the endpoint's body: path null while no line is stored for the version.
type RoutePathResponse struct {
	Path *RoutePath `json:"path"`
}

// RoutePathVersion is what sp_get_route_path answers: the version in force that day, when its line was
// last computed, and the line when there is one. A nil *RoutePathVersion is "no version that day".
type RoutePathVersion struct {
	VersionID  uuid.UUID
	ComputedAt *time.Time
	Path       *RoutePath
}

// ETag names the version and the moment its line was stored, so a new line moves it and nothing else
// does.
func (v *RoutePathVersion) ETag() string {
	if v == nil {
		return `"none"`
	}
	stamp := int64(0)
	if v.ComputedAt != nil {
		stamp = v.ComputedAt.UnixMicro()
	}
	return fmt.Sprintf(`"%s-%d"`, v.VersionID, stamp)
}

// RouteVersionPoints is what a version's line is computed from: its located stops in sequence, and
// the hash and source of the line stored now.
type RouteVersionPoints struct {
	Points     []geoModels.LatLng
	PointsHash *string
	Source     *string
}
