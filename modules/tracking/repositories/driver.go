package repositories

import (
	"context"

	trackingErrors "josex/web/modules/tracking/errors"
	"josex/web/modules/tracking/models"

	"github.com/google/uuid"
)

// The driver's phone of TRACK-011: one SP per endpoint, the account resolved to its driver inside each.

// DriverToday answers sp_driver_today's JSONB as the database wrote it: the bytes the ETag hashes.
func (r *TrackingRepository) DriverToday(ctx context.Context, tenantID, userID uuid.UUID) ([]byte, error) {
	var day []byte
	err := r.dbService.QueryRow(ctx, `SELECT tracking.sp_driver_today(p_tenant_id := $1, p_user_id := $2)`, tenantID, userID).Scan(&day)
	if err != nil {
		return nil, mapTripError(err, trackingErrors.DriverTodayFailed)
	}
	return day, nil
}

// DriverStartTrip starts one of the driver's own trips.
func (r *TrackingRepository) DriverStartTrip(ctx context.Context, tenantID, userID, tripID uuid.UUID) (*models.TripDetail, error) {
	return r.queryTripDetail(ctx, trackingErrors.TripTransitionFailed, `
		SELECT * FROM tracking.sp_driver_start_trip(
			p_tenant_id := $1,
			p_user_id   := $2,
			p_trip_id   := $3
		)
	`, tenantID, userID, tripID)
}

// DriverSync applies a queue of ops, one result each; Code is the SP's own code.
func (r *TrackingRepository) DriverSync(ctx context.Context, tenantID, userID uuid.UUID, ops []byte) ([]models.DriverOpResult, error) {
	rows, err := r.dbService.Query(ctx, `
		SELECT client_op_id, status, code, trip_id
		FROM tracking.sp_driver_sync(p_tenant_id := $1, p_user_id := $2, p_ops := $3::JSONB)
	`, tenantID, userID, ops)
	if err != nil {
		return nil, mapTripError(err, trackingErrors.DriverSyncFailed)
	}
	defer rows.Close()
	results := []models.DriverOpResult{}
	for rows.Next() {
		var res models.DriverOpResult
		if err := rows.Scan(&res.ClientOpID, &res.Status, &res.Code, &res.TripID); err != nil {
			return nil, err
		}
		results = append(results, res)
	}
	if err := rows.Err(); err != nil {
		return nil, mapTripError(err, trackingErrors.DriverSyncFailed)
	}
	return results, nil
}

// The rider card and the incident report of TRACK-027.

// RiderQRToken reads a rider's card code, or replaces it first when rotate.
func (r *TrackingRepository) RiderQRToken(ctx context.Context, tenantID, riderID uuid.UUID, rotate bool, scopeUserID *uuid.UUID) (string, error) {
	var token string
	err := r.dbService.QueryRow(ctx, `
		SELECT tracking.sp_rider_qr_token(p_tenant_id := $1, p_rider_id := $2, p_rotate := $3, p_scope_user_id := $4)
	`, tenantID, riderID, rotate, scopeUserID).Scan(&token)
	if err != nil {
		return "", scopedErr(err, trackingErrors.RiderQRTokenFailed)
	}
	return token, nil
}

// DriverResolveRider answers the rider a card code names; an unknown code is rider.not-found.
func (r *TrackingRepository) DriverResolveRider(ctx context.Context, tenantID, userID uuid.UUID, code string) (*models.DriverRider, error) {
	riders, err := r.queryDriverRiders(ctx, `
		SELECT * FROM tracking.sp_driver_resolve_rider(p_tenant_id := $1, p_user_id := $2, p_token := $3)
	`, tenantID, userID, code)
	if err != nil {
		return nil, err
	}
	return &riders[0], nil
}

// DriverFindRiders answers the riders on the driver's trips in progress matching a name or a code.
func (r *TrackingRepository) DriverFindRiders(ctx context.Context, tenantID, userID uuid.UUID, query string) ([]models.DriverRider, error) {
	return r.queryDriverRiders(ctx, `
		SELECT * FROM tracking.sp_driver_find_riders(p_tenant_id := $1, p_user_id := $2, p_query := $3)
	`, tenantID, userID, query)
}

// queryDriverRiders scans the rider-and-pending-pickup rows both lookups answer.
func (r *TrackingRepository) queryDriverRiders(ctx context.Context, query string, args ...any) ([]models.DriverRider, error) {
	rows, err := r.dbService.Query(ctx, query, args...)
	if err != nil {
		return nil, mapTripError(err, trackingErrors.DriverRiderFailed)
	}
	defer rows.Close()
	riders := []models.DriverRider{}
	for rows.Next() {
		var rider models.DriverRider
		var taskID, tripID, stopID *uuid.UUID
		var kind, status *string
		if err := rows.Scan(&rider.Rider.ID, &rider.Rider.Name, &taskID, &tripID, &stopID, &kind, &status); err != nil {
			return nil, err
		}
		if taskID != nil {
			rider.Task = &models.DriverRiderTask{ID: *taskID, TripID: *tripID, TripStopID: *stopID, Kind: *kind, Status: *status}
		}
		riders = append(riders, rider)
	}
	if err := rows.Err(); err != nil {
		return nil, mapTripError(err, trackingErrors.DriverRiderFailed)
	}
	return riders, nil
}

// DriverIncident reports an incident on one of the driver's trips.
func (r *TrackingRepository) DriverIncident(ctx context.Context, tenantID, userID uuid.UUID, dto *models.DriverIncidentDto) (*models.DriverIncident, error) {
	var a models.DriverIncident
	err := r.dbService.QueryRow(ctx, `
		SELECT * FROM tracking.sp_driver_incident(
			p_tenant_id  := $1,
			p_user_id    := $2,
			p_trip_id    := $3,
			p_alert_type := $4,
			p_message    := $5,
			p_lat        := $6,
			p_lng        := $7
		)
	`, tenantID, userID, dto.TripID, dto.AlertType, dto.Message, dto.Lat, dto.Lng).Scan(
		&a.ID, &a.RouteID, &a.VehicleID, &a.TripID, &a.AlertType, &a.Title, &a.Message, &a.Severity,
		&a.Status, &a.Lat, &a.Lng, &a.CreatedBy, &a.CreatedAt)
	if err != nil {
		return nil, mapTripError(err, trackingErrors.DriverIncidentFailed)
	}
	return &a, nil
}
