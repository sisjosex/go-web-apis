package repositories

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// The trip slice of TRACK-008: the materialiser both background callers share.

// MaterialiseTrips brings the not-started trips of routeID — every route of the tenant when nil — in
// line with the plan over the route's local today..today+14, narrowed by from/to when set. It answers
// how many trips it brought in line. Idempotent: a second call changes nothing.
func (r *TrackingRepository) MaterialiseTrips(ctx context.Context, tenantID uuid.UUID, routeID *uuid.UUID, from, to *time.Time) (int, error) {
	var trips int
	err := r.dbService.QueryRow(ctx,
		`SELECT tracking.sp_materialise_trips(p_tenant_id := $1, p_route_id := $2, p_from := $3::DATE, p_to := $4::DATE)`,
		tenantID, routeID, from, to,
	).Scan(&trips)
	return trips, err
}
