package jobs

import (
	"context"
	"errors"

	coreJobs "josex/web/modules/core/jobs"
	coreServices "josex/web/modules/core/services"
	geoInterfaces "josex/web/modules/geo/interfaces"
	geoModels "josex/web/modules/geo/models"
	"josex/web/modules/geo/services/routing"
	trackingRepos "josex/web/modules/tracking/repositories"

	"github.com/google/uuid"
)

// Where a trip's stored line came from (TRACK-010 step 4).
const (
	TraceValhalla = "valhalla"
	TraceFallback = "fallback"
)

// ApplyTripTrace map-matches a completed trip's points and stores the line and its km on the trip.
// The router down, refusing the path or absent (nil): the raw GPS line is stored instead, flagged
// fallback. Fewer than two points: nothing to draw, nothing stored. It answers the source it stored,
// "" when none.
func ApplyTripTrace(ctx context.Context, db coreServices.DatabaseService, router geoInterfaces.Router, tenant coreJobs.Tenant, tripID uuid.UUID) (string, error) {
	tenantID, err := uuid.Parse(tenant.ID)
	if err != nil {
		return "", err
	}
	tenantCtx := context.WithValue(ctx, coreServices.TenantDatabaseURLKey, tenant.DatabaseURL)
	repo := trackingRepos.NewTrackingRepository(db)
	points, err := repo.TripTracePoints(tenantCtx, tenantID, tripID)
	if err != nil || len(points) < 2 {
		return "", err
	}

	trace, source := rawTrace(points), TraceFallback
	if router != nil {
		matched, err := router.Trace(ctx, points, "bus")
		switch {
		case err == nil:
			trace, source = matched, TraceValhalla
		case !errors.Is(err, geoInterfaces.ErrNoRoute) && !errors.Is(err, geoInterfaces.ErrRouteUnavailable):
			return "", err
		}
	}
	return source, repo.CloseTripTrace(tenantCtx, tenantID, tripID, trace.Polyline6, trace.DistanceM/1000, source)
}

// rawTrace is the recorded path as it is: the line through the points and its length.
func rawTrace(points []geoModels.LatLng) *geoModels.Trace {
	return &geoModels.Trace{Polyline6: routing.EncodePolyline6(points), DistanceM: routing.LineLengthM(points)}
}
