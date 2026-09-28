package jobs

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"time"

	coreJobs "josex/web/modules/core/jobs"
	coreServices "josex/web/modules/core/services"
	geoInterfaces "josex/web/modules/geo/interfaces"
	geoModels "josex/web/modules/geo/models"
	"josex/web/modules/geo/services/routing"
	"josex/web/modules/tracking/models"
	trackingRepos "josex/web/modules/tracking/repositories"

	"github.com/google/uuid"
)

// Where a route version's planned line came from (TRACK-028 D1).
const (
	PathValhalla = "valhalla"
	PathFallback = "fallback"
)

// ErrPathFallback fails a route.changed task that stored a straight line because the router was down
// or found no route, so asynq retries it with backoff until the streets answer.
var ErrPathFallback = errors.New("route path stored as fallback")

// PathPlanner computes the line a route version's buses drive. Router nil stores the straight line and
// never retries; FallbackKmh times its legs.
type PathPlanner struct {
	Router      geoInterfaces.Router
	FallbackKmh float64
}

// ApplyRoutePaths computes the line of every version of routeID in force from dateFrom (all of them
// when nil). A version whose points did not change keeps its line with no router call; one that fell
// back is stored and the whole call answers ErrPathFallback once the rest are done.
func (p PathPlanner) ApplyRoutePaths(ctx context.Context, db coreServices.DatabaseService, tenant coreJobs.Tenant, routeID uuid.UUID, dateFrom *time.Time) error {
	tenantID, err := uuid.Parse(tenant.ID)
	if err != nil {
		return err
	}
	tenantCtx := context.WithValue(ctx, coreServices.TenantDatabaseURLKey, tenant.DatabaseURL)
	repo := trackingRepos.NewTrackingRepository(db)
	versions, err := repo.RoutePathVersions(tenantCtx, tenantID, routeID, dateFrom)
	if err != nil {
		return err
	}
	var fellBack error
	for _, versionID := range versions {
		source, err := p.applyVersion(tenantCtx, repo, tenantID, versionID)
		if err != nil {
			return err
		}
		if source == PathFallback && p.Router != nil {
			fellBack = fmt.Errorf("version %s: %w", versionID, ErrPathFallback)
		}
	}
	return fellBack
}

// applyVersion stores one version's line and answers its source, "" when there is none to draw.
func (p PathPlanner) applyVersion(ctx context.Context, repo *trackingRepos.TrackingRepository, tenantID, versionID uuid.UUID) (string, error) {
	stored, err := repo.RouteVersionPathPoints(ctx, tenantID, versionID)
	if err != nil || stored == nil {
		return "", err
	}
	hash := pointsHash(stored.Points)
	same := stored.PointsHash != nil && *stored.PointsHash == hash
	if len(stored.Points) < 2 {
		if same && stored.Source == nil {
			return "", nil
		}
		return "", repo.SetRouteVersionPath(ctx, tenantID, versionID, nil, hash)
	}
	if same && stored.Source != nil && (*stored.Source == PathValhalla || p.Router == nil) {
		return *stored.Source, nil
	}

	path, err := p.plan(ctx, stored.Points)
	if err != nil {
		return "", err
	}
	if same && path.Source == PathFallback {
		// Still no streets for the same points: the straight line already stored stays, unstamped, so
		// the maps' ETag does not move for nothing.
		return PathFallback, nil
	}
	return path.Source, repo.SetRouteVersionPath(ctx, tenantID, versionID, path, hash)
}

// plan asks the router for the line through points; down or with no route, it answers the straight line.
func (p PathPlanner) plan(ctx context.Context, points []geoModels.LatLng) (*models.RoutePath, error) {
	if p.Router == nil {
		return p.straight(points), nil
	}
	route, err := p.Router.Route(ctx, points, "bus")
	switch {
	case err == nil:
		return streetPath(route), nil
	case errors.Is(err, geoInterfaces.ErrNoRoute), errors.Is(err, geoInterfaces.ErrRouteUnavailable):
		return p.straight(points), nil
	default:
		return nil, err
	}
}

// streetPath is the router's answer as a stored line.
func streetPath(route *geoModels.Route) *models.RoutePath {
	path := &models.RoutePath{Polyline6: route.Polyline6, DistanceM: int(math.Round(route.DistanceM)), Source: PathValhalla,
		Legs: make([]models.RoutePathLeg, 0, len(route.Legs))}
	for _, leg := range route.Legs {
		path.Legs = append(path.Legs, models.RoutePathLeg{DistanceM: int(math.Round(leg.DistanceM)), DurationS: int(math.Round(leg.DurationS))})
	}
	return path
}

// straight is the line joining the stops, each leg timed at the fallback speed.
func (p PathPlanner) straight(points []geoModels.LatLng) *models.RoutePath {
	path := &models.RoutePath{Polyline6: routing.EncodePolyline6(points), Source: PathFallback,
		Legs: make([]models.RoutePathLeg, 0, len(points)-1)}
	metresPerSecond := p.FallbackKmh * 1000 / 3600
	for i := 1; i < len(points); i++ {
		distance := routing.LineLengthM(points[i-1 : i+1])
		leg := models.RoutePathLeg{DistanceM: int(math.Round(distance))}
		if metresPerSecond > 0 {
			leg.DurationS = int(math.Round(distance / metresPerSecond))
		}
		path.Legs = append(path.Legs, leg)
		path.DistanceM += leg.DistanceM
	}
	return path
}

// pointsHash names the ordered coordinates a line is computed from, to the precision the line keeps.
func pointsHash(points []geoModels.LatLng) string {
	sum := sha256.New()
	for _, point := range points {
		_, _ = fmt.Fprintf(sum, "%.6f,%.6f;", point.Lat, point.Lng)
	}
	return hex.EncodeToString(sum.Sum(nil))
}
