package repositories

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	trackingErrors "josex/web/modules/tracking/errors"
	"josex/web/modules/tracking/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
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

// The trip endpoints of TRACK-020, beside the materialiser: one slice, one file.

// scanTrip is the one scan order every trip read and write shares.
func scanTrip(t *models.Trip) []any {
	return []any{
		&t.ID, &t.RouteID, &t.RouteName, &t.Direction, &t.RouteScheduleID, &t.ServiceDate, &t.Timezone,
		&t.PlannedStart, &t.VehicleID, &t.LicensePlate, &t.DriverID, &t.DriverName, &t.Status,
		&t.StartedAt, &t.EndedAt, &t.IsOverridden, &t.StopsCount, &t.TasksTotal, &t.TasksDone,
		&t.CreatedAt, &t.UpdatedAt,
	}
}

// tripCodes turns the trip SPs' own refusals into module codes.
var tripCodes = map[string]string{
	"trip.not-found":            trackingErrors.TripNotFound,
	"trip.stop.not-found":       trackingErrors.TripStopNotFound,
	"trip.task.not-found":       trackingErrors.TripTaskNotFound,
	"trip.invalid-transition":   trackingErrors.TripInvalidTransition,
	"trip.task.trip-not-active": trackingErrors.TripTaskTripNotActive,
	"trip.task.op-conflict":     trackingErrors.TripTaskOpConflict,
	"route.company-mismatch":    trackingErrors.RouteCompanyMismatch,
}

// mapTripError maps an SP refusal to its code, falling back to fallbackCode. company-mismatch names
// the offending field as DETAIL, which becomes the error's Detail.
func mapTripError(err error, fallbackCode string) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		if code, ok := tripCodes[pgErr.Message]; ok {
			mapped := &trackingErrors.TrackingError{Code: code, Err: pgErr}
			if pgErr.Detail != "" {
				mapped.Detail = pgErr.Detail
			}
			return mapped
		}
	}
	return &trackingErrors.TrackingError{Code: fallbackCode, Err: err}
}

// queryTripDetail runs an SP that answers sp_get_trip's shape — the detail read and every write.
func (r *TrackingRepository) queryTripDetail(ctx context.Context, fallbackCode, query string, args ...any) (*models.TripDetail, error) {
	var detail models.TripDetail
	var stops []byte
	if err := r.dbService.QueryRow(ctx, query, args...).Scan(append(scanTrip(&detail.Trip), &stops)...); err != nil {
		return nil, mapTripError(err, fallbackCode)
	}
	if err := json.Unmarshal(stops, &detail.Stops); err != nil {
		return nil, &trackingErrors.TrackingError{Code: fallbackCode, Err: err}
	}
	return &detail, nil
}

// ListTrips returns one page of a day's trips plus the total the same filters match.
func (r *TrackingRepository) ListTrips(ctx context.Context, tenantID uuid.UUID, query models.ListTripsQuery) ([]*models.TripListRow, int64, error) {
	rows, err := r.dbService.Query(ctx, `
		SELECT * FROM tracking.sp_list_trips(
			p_tenant_id       := $1,
			p_date            := $2::DATE,
			p_route_id        := $3::UUID,
			p_status          := $4,
			p_organization_id := $5::UUID,
			p_page            := $6,
			p_page_size       := $7
		)
	`, tenantID, query.Date, query.RouteID, query.Status, query.OrganizationID, query.Page, query.PageSize)
	if err != nil {
		return nil, 0, mapTripError(err, trackingErrors.TripListFailed)
	}
	defer rows.Close()
	trips := []*models.TripListRow{}
	var totalCount int64
	for rows.Next() {
		var t models.TripListRow
		if err := rows.Scan(append(scanTrip(&t.Trip), &t.DelaySeconds, &totalCount)...); err != nil {
			return nil, 0, err
		}
		trips = append(trips, &t)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, mapTripError(err, trackingErrors.TripListFailed)
	}
	return trips, totalCount, nil
}

// GetTrip is the detail plus the driven path stored at trip close.
func (r *TrackingRepository) GetTrip(ctx context.Context, tenantID, tripID uuid.UUID) (*models.TripDetail, error) {
	var detail models.TripDetail
	var stops []byte
	err := r.dbService.QueryRow(ctx, `SELECT * FROM tracking.sp_get_trip_traced(p_tenant_id := $1, p_trip_id := $2)`, tenantID, tripID).
		Scan(append(scanTrip(&detail.Trip), &stops, &detail.Polyline, &detail.DistanceKm, &detail.TraceSource)...)
	if err != nil {
		return nil, mapTripError(err, trackingErrors.TripNotFound)
	}
	if err := json.Unmarshal(stops, &detail.Stops); err != nil {
		return nil, &trackingErrors.TrackingError{Code: trackingErrors.TripNotFound, Err: err}
	}
	return &detail, nil
}

func (r *TrackingRepository) GetTripStatus(ctx context.Context, tenantID, tripID uuid.UUID) (*models.TripStatus, error) {
	var s models.TripStatus
	var nextStop []byte
	err := r.dbService.QueryRow(ctx, `SELECT * FROM tracking.sp_get_trip_status(p_tenant_id := $1, p_trip_id := $2)`, tenantID, tripID).Scan(
		&s.TripID, &s.RouteID, &s.Status, &s.PlannedStart, &s.StartedAt, &s.EndedAt, &nextStop, &s.DelaySeconds,
		&s.VehicleID, &s.CurrentLatitude, &s.CurrentLongitude, &s.CurrentSpeed, &s.LocationAgeSeconds,
		&s.TotalRiders, &s.BoardedCount, &s.ArrivedCount, &s.NoShowCount, &s.PendingCount,
	)
	if err != nil {
		return nil, mapTripError(err, trackingErrors.TripNotFound)
	}
	if nextStop != nil {
		if err := json.Unmarshal(nextStop, &s.NextStop); err != nil {
			return nil, &trackingErrors.TrackingError{Code: trackingErrors.TripNotFound, Err: err}
		}
	}
	return &s, nil
}

// UpdateTrip is the operator's override; the fields left out keep their values.
func (r *TrackingRepository) UpdateTrip(ctx context.Context, tenantID, tripID uuid.UUID, dto *models.UpdateTripDto, userID *uuid.UUID) (*models.TripDetail, error) {
	changes, err := json.Marshal(dto)
	if err != nil {
		return nil, &trackingErrors.TrackingError{Code: trackingErrors.TripUpdateFailed, Err: err}
	}
	return r.queryTripDetail(ctx, trackingErrors.TripUpdateFailed, `
		SELECT * FROM tracking.sp_update_trip(
			p_tenant_id := $1,
			p_trip_id   := $2,
			p_changes   := $3,
			p_user_id   := $4
		)
	`, tenantID, tripID, changes, userID)
}

// TransitionTrip starts, completes or cancels a trip.
func (r *TrackingRepository) TransitionTrip(ctx context.Context, tenantID, tripID uuid.UUID, action string, userID *uuid.UUID) (*models.TripDetail, error) {
	return r.queryTripDetail(ctx, trackingErrors.TripTransitionFailed, `
		SELECT * FROM tracking.sp_trip_transition(
			p_tenant_id := $1,
			p_trip_id   := $2,
			p_action    := $3,
			p_user_id   := $4
		)
	`, tenantID, tripID, action, userID)
}

// TransitionTripStop marks a stop arrived or skipped.
func (r *TrackingRepository) TransitionTripStop(ctx context.Context, tenantID, stopID uuid.UUID, action string, userID *uuid.UUID) (*models.TripDetail, error) {
	return r.queryTripDetail(ctx, trackingErrors.TripTransitionFailed, `
		SELECT * FROM tracking.sp_trip_stop_transition(
			p_tenant_id := $1,
			p_stop_id   := $2,
			p_action    := $3,
			p_user_id   := $4
		)
	`, tenantID, stopID, action, userID)
}

// TransitionTripTask marks a task done or no_show, once per client_op_id.
func (r *TrackingRepository) TransitionTripTask(ctx context.Context, tenantID, taskID uuid.UUID, action string, dto *models.TripTaskTransitionDto, userID *uuid.UUID) (*models.TripDetail, error) {
	var proof []byte
	if dto.Proof != nil {
		var err error
		if proof, err = json.Marshal(dto.Proof); err != nil {
			return nil, &trackingErrors.TrackingError{Code: trackingErrors.TripTransitionFailed, Err: err}
		}
	}
	return r.queryTripDetail(ctx, trackingErrors.TripTransitionFailed, `
		SELECT * FROM tracking.sp_trip_task_transition(
			p_tenant_id    := $1,
			p_task_id      := $2,
			p_action       := $3,
			p_user_id      := $4,
			p_client_op_id := $5,
			p_proof        := $6::JSONB
		)
	`, tenantID, taskID, action, userID, dto.ClientOpID, proof)
}
