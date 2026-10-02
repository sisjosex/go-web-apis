package repositories

import (
	"context"
	"encoding/json"
	"time"

	trackingErrors "josex/web/modules/tracking/errors"
	"josex/web/modules/tracking/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// The live slice of TRACK-025: the two snapshots and the subscribe check.

// LiveFleet answers every reporting vehicle of the tenant, narrowed to an organization's trips.
func (r *TrackingRepository) LiveFleet(ctx context.Context, tenantID uuid.UUID, organizationID *string) ([]models.LiveVehicle, error) {
	rows, err := r.dbService.Query(ctx,
		`SELECT * FROM tracking.sp_live_fleet(p_tenant_id := $1, p_organization_id := $2::UUID)`, tenantID, organizationID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (models.LiveVehicle, error) {
		var v models.LiveVehicle
		err := row.Scan(&v.VehicleID, &v.LicensePlate, &v.TripID, &v.RouteID, &v.RouteName, &v.Lat, &v.Lng, &v.Speed, &v.Heading, &v.RecordedAt)
		return v, err
	})
}

// TripLive answers the trip detail with its vehicle's last position, and the pending stops the ETA
// runs to. Eta is left empty for the caller.
func (r *TrackingRepository) TripLive(ctx context.Context, tenantID, tripID uuid.UUID) (*models.TripLive, []models.PendingStop, error) {
	var live models.TripLive
	var stops, position, pending []byte
	err := r.dbService.QueryRow(ctx, `SELECT * FROM tracking.sp_trip_live(p_tenant_id := $1, p_trip_id := $2)`, tenantID, tripID).
		Scan(append(scanTrip(&live.Trip), &stops, &live.Polyline, &live.DistanceKm, &live.TraceSource, &position, &pending)...)
	if err != nil {
		return nil, nil, mapTripError(err, trackingErrors.TripNotFound)
	}
	if err := json.Unmarshal(stops, &live.Stops); err != nil {
		return nil, nil, err
	}
	var pendingStops []models.PendingStop
	if err := json.Unmarshal(pending, &pendingStops); err != nil {
		return nil, nil, err
	}
	if position != nil {
		live.Position = position
	}
	live.Eta = []models.StopEta{}
	return &live, pendingStops, nil
}

// RiderLive answers the rider's trip in progress with the rider's stops and the vehicle's position,
// and the trip's pending stops the ETA runs through. Scope as GetRiderStatus: out of it, not-found.
func (r *TrackingRepository) RiderLive(ctx context.Context, tenantID, riderID uuid.UUID, scopeUserID, guardianUserID *uuid.UUID) (*models.RiderLive, []models.PendingStop, error) {
	var live models.RiderLive
	var stops, position, pending []byte
	err := r.dbService.QueryRow(ctx,
		`SELECT * FROM tracking.sp_rider_live(p_tenant_id := $1, p_rider_id := $2, p_scope_user_id := $3, p_guardian_user_id := $4)`,
		tenantID, riderID, scopeUserID, guardianUserID).
		Scan(&live.RiderID, &live.TripID, &live.RouteName, &live.LicensePlate, &stops, &position, &pending)
	if err != nil {
		return nil, nil, scopedErr(err, trackingErrors.TrackingInternalError)
	}
	if err := json.Unmarshal(stops, &live.Stops); err != nil {
		return nil, nil, err
	}
	var pendingStops []models.PendingStop
	if err := json.Unmarshal(pending, &pendingStops); err != nil {
		return nil, nil, err
	}
	if position != nil {
		live.Position = position
	}
	live.Eta = []models.StopEta{}
	return &live, pendingStops, nil
}

// CanSubscribe answers whether the user may listen to channel; guardian is the portal level.
func (r *TrackingRepository) CanSubscribe(ctx context.Context, tenantID, userID uuid.UUID, guardian bool, channel string) (bool, error) {
	var ok bool
	err := r.dbService.QueryRow(ctx,
		`SELECT tracking.sp_can_subscribe(p_tenant_id := $1, p_user_id := $2, p_guardian := $3, p_channel := $4)`,
		tenantID, userID, guardian, channel).Scan(&ok)
	return ok, err
}

// TripAudience answers the trip's riders not yet off it — plus those with a task on subjectID, the
// task or stop that moved (uuid.Nil: none) — and their organizations: who its changes are published to.
func (r *TrackingRepository) TripAudience(ctx context.Context, tenantID, tripID, subjectID uuid.UUID) (riders, organizations []uuid.UUID, err error) {
	var subject *uuid.UUID
	if subjectID != uuid.Nil {
		subject = &subjectID
	}
	err = r.dbService.QueryRow(ctx,
		`SELECT rider_ids, organization_ids FROM tracking.sp_trip_audience(p_tenant_id := $1, p_trip_id := $2, p_subject_id := $3)`,
		tenantID, tripID, subject).Scan(&riders, &organizations)
	return riders, organizations, err
}

// DayAudience is one driver a day_changed goes to, and the user it is linked to (nil: none).
type DayAudience struct {
	DriverID uuid.UUID
	UserID   *uuid.UUID
}

// DriverAudience answers the drivers with a trip today on routeID (within from..to) or on tripID —
// either may be nil.
func (r *TrackingRepository) DriverAudience(ctx context.Context, tenantID uuid.UUID, routeID, tripID *uuid.UUID, from, to *time.Time) ([]DayAudience, error) {
	rows, err := r.dbService.Query(ctx,
		`SELECT driver_id, user_id FROM tracking.sp_driver_audience(p_tenant_id := $1, p_route_id := $2, p_trip_id := $3, p_date_from := $4, p_date_to := $5)`,
		tenantID, routeID, tripID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DayAudience
	for rows.Next() {
		var a DayAudience
		if err := rows.Scan(&a.DriverID, &a.UserID); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
