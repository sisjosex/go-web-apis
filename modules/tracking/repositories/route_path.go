package repositories

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	trackingErrors "josex/web/modules/tracking/errors"
	"josex/web/modules/tracking/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// The planned-line slice of TRACK-028: the read every map makes, and what the route.changed handler
// reads and writes.

// GetRoutePath answers the version in force on date (the route's local today when nil) with its line,
// nil when the route has no version that day. Scope as ListRouteStops: out of it, route not-found.
func (r *TrackingRepository) GetRoutePath(ctx context.Context, tenantID, routeID uuid.UUID, date *string, scopeUserID *uuid.UUID) (*models.RoutePathVersion, error) {
	var version models.RoutePathVersion
	var polyline, source *string
	var distanceM *int
	var legs []byte
	err := r.dbService.QueryRow(ctx,
		`SELECT * FROM tracking.sp_get_route_path(p_tenant_id := $1, p_route_id := $2, p_date := $3::DATE, p_scope_user_id := $4)`,
		tenantID, routeID, date, scopeUserID).
		Scan(&version.VersionID, &polyline, &distanceM, &legs, &source, &version.ComputedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, scopedErr(err, trackingErrors.RoutePathReadFailed)
	}
	if polyline == nil || version.ComputedAt == nil {
		return &version, nil
	}
	path := &models.RoutePath{VersionID: version.VersionID, Polyline6: *polyline, Legs: []models.RoutePathLeg{}, ComputedAt: *version.ComputedAt}
	if distanceM != nil {
		path.DistanceM = *distanceM
	}
	if source != nil {
		path.Source = *source
	}
	if legs != nil {
		if err := json.Unmarshal(legs, &path.Legs); err != nil {
			return nil, err
		}
	}
	version.Path = path
	return &version, nil
}

// RoutePathVersions answers the versions of a route in force from dateFrom on — all of them when nil.
func (r *TrackingRepository) RoutePathVersions(ctx context.Context, tenantID, routeID uuid.UUID, dateFrom *time.Time) ([]uuid.UUID, error) {
	rows, err := r.dbService.Query(ctx,
		`SELECT version_id FROM tracking.sp_route_path_versions(p_tenant_id := $1, p_route_id := $2, p_date_from := $3::DATE)`,
		tenantID, routeID, dateFrom)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
}

// RoutePathsBackfill writes a route.changed row for every route with a version whose line was never
// computed, and answers how many.
func (r *TrackingRepository) RoutePathsBackfill(ctx context.Context, tenantID uuid.UUID) (int, error) {
	var enqueued int
	err := r.dbService.QueryRow(ctx, `SELECT tracking.sp_route_paths_backfill(p_tenant_id := $1)`, tenantID).Scan(&enqueued)
	return enqueued, err
}

// RouteVersionPathPoints answers a version's located stops and what its stored line was computed
// from; nil when the version is not the tenant's.
func (r *TrackingRepository) RouteVersionPathPoints(ctx context.Context, tenantID, versionID uuid.UUID) (*models.RouteVersionPoints, error) {
	var out models.RouteVersionPoints
	var points []byte
	err := r.dbService.QueryRow(ctx,
		`SELECT points, path_points_hash, path_source FROM tracking.sp_route_version_path_points(p_tenant_id := $1, p_version_id := $2)`,
		tenantID, versionID).Scan(&points, &out.PointsHash, &out.Source)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(points, &out.Points); err != nil {
		return nil, err
	}
	return &out, nil
}

// SetRouteVersionPath stores a version's line; path nil clears it. Either way the version is stamped
// computed, from pointsHash.
func (r *TrackingRepository) SetRouteVersionPath(ctx context.Context, tenantID, versionID uuid.UUID, path *models.RoutePath, pointsHash string) error {
	var polyline, source *string
	var distanceM *int
	var legs []byte
	if path != nil {
		raw, err := json.Marshal(path.Legs)
		if err != nil {
			return err
		}
		polyline, distanceM, legs, source = &path.Polyline6, &path.DistanceM, raw, &path.Source
	}
	_, err := r.dbService.Execute(ctx,
		`SELECT tracking.sp_set_route_version_path(p_tenant_id := $1, p_version_id := $2, p_polyline := $3, p_distance_m := $4,
			p_legs := $5::JSONB, p_source := $6, p_points_hash := $7)`,
		tenantID, versionID, polyline, distanceM, legs, source, pointsHash)
	return err
}
