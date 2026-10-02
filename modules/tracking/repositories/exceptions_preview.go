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

// The exception and preview slice of TRACK-018.

// ==================== EXCEPTIONS ====================

// scanRouteException is the one scan order every exception read shares.
func scanRouteException(e *models.RouteException) []any {
	return []any{
		&e.ID, &e.RouteID, &e.DateFrom, &e.DateTo, &e.Kind, &e.Payload, &e.Reason,
		&e.CreatedBy, &e.CreatedAt, &e.UpdatedAt,
	}
}

// mapExceptionError turns the SP's own codes into module codes, falling back to fallbackCode. The
// three id raises are the payload naming something outside the tenant.
func mapExceptionError(err error, fallbackCode string) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		if pgErr.ConstraintName == "chk_route_exception_range" {
			return &trackingErrors.TrackingError{Code: trackingErrors.ExceptionRange, Err: pgErr}
		}
		switch pgErr.Message {
		case "route.not-found":
			return &trackingErrors.TrackingError{Code: trackingErrors.RouteNotFound, Err: pgErr}
		case "exception.not-found":
			return &trackingErrors.TrackingError{Code: trackingErrors.ExceptionNotFound, Err: pgErr}
		case "exception.payload":
			return &trackingErrors.TrackingError{Code: trackingErrors.ExceptionPayload, Err: pgErr}
		case "route.preview-range":
			return &trackingErrors.TrackingError{Code: trackingErrors.RoutePreviewRange, Err: pgErr}
		case "vehicle.not-found":
			return &trackingErrors.TrackingError{Code: trackingErrors.VehicleNotFound, Err: pgErr}
		case "driver.not-found":
			return &trackingErrors.TrackingError{Code: trackingErrors.DriverNotFound, Err: pgErr}
		case "stop-place.not-found":
			return &trackingErrors.TrackingError{Code: trackingErrors.StopPlaceNotFound, Err: pgErr}
		}
	}
	return &trackingErrors.TrackingError{Code: fallbackCode, Err: err}
}

func (r *TrackingRepository) ListRouteExceptions(ctx context.Context, tenantID uuid.UUID, routeID uuid.UUID, dateFrom, dateTo *string) ([]*models.RouteException, error) {
	rows, err := r.dbService.Query(ctx, `
		SELECT * FROM tracking.sp_list_route_exceptions(
			p_tenant_id := $1,
			p_route_id  := $2,
			p_date_from := $3,
			p_date_to   := $4
		)
	`, tenantID, routeID, dateFrom, dateTo)
	if err != nil {
		return nil, mapExceptionError(err, trackingErrors.ExceptionListFailed)
	}
	defer rows.Close()
	exceptions := []*models.RouteException{}
	for rows.Next() {
		var exception models.RouteException
		if err := rows.Scan(scanRouteException(&exception)...); err != nil {
			return nil, err
		}
		exceptions = append(exceptions, &exception)
	}
	if err := rows.Err(); err != nil {
		return nil, mapExceptionError(err, trackingErrors.ExceptionListFailed)
	}
	return exceptions, nil
}

func (r *TrackingRepository) CreateRouteException(ctx context.Context, tenantID uuid.UUID, routeID uuid.UUID, dto *models.CreateRouteExceptionDto, createdBy *uuid.UUID) (*models.RouteException, error) {
	payload, err := json.Marshal(dto.Payload)
	if err != nil {
		return nil, &trackingErrors.TrackingError{Code: trackingErrors.ExceptionCreateFailed, Err: err}
	}
	var exception models.RouteException
	err = r.dbService.QueryRow(ctx, `
		SELECT * FROM tracking.sp_create_route_exception(
			p_tenant_id  := $1,
			p_route_id   := $2,
			p_date_from  := $3,
			p_date_to    := $4,
			p_kind       := $5,
			p_payload    := $6,
			p_reason     := $7,
			p_created_by := $8
		)
	`, tenantID, routeID, time.Time(dto.DateFrom), time.Time(dto.DateTo), dto.Kind, payload,
		dto.Reason, createdBy,
	).Scan(scanRouteException(&exception)...)
	if err != nil {
		return nil, mapExceptionError(err, trackingErrors.ExceptionCreateFailed)
	}
	return &exception, nil
}

func (r *TrackingRepository) DeleteRouteException(ctx context.Context, tenantID uuid.UUID, exceptionID uuid.UUID) error {
	var deleted bool
	err := r.dbService.QueryRow(ctx, `SELECT tracking.sp_delete_route_exception($1, $2)`, tenantID, exceptionID).Scan(&deleted)
	if err != nil {
		return mapExceptionError(err, trackingErrors.ExceptionDeleteFailed)
	}
	if !deleted {
		return &trackingErrors.TrackingError{Code: trackingErrors.ExceptionDeleteFailed}
	}
	return nil
}

// ==================== PREVIEW ====================

// PreviewRoute reads the plan for a window. The SP is STABLE and writes nothing — TRACK-008
// materialises trips from this same query, so opening the calendar screen leaves no trail.
func (r *TrackingRepository) PreviewRoute(ctx context.Context, tenantID uuid.UUID, routeID uuid.UUID, from, to string) ([]*models.RoutePreviewDay, error) {
	rows, err := r.dbService.Query(ctx, `
		SELECT * FROM tracking.sp_preview_route(
			p_tenant_id := $1,
			p_route_id  := $2,
			p_from      := $3,
			p_to        := $4
		)
	`, tenantID, routeID, from, to)
	if err != nil {
		return nil, mapExceptionError(err, trackingErrors.RoutePreviewFailed)
	}
	defer rows.Close()
	days := []*models.RoutePreviewDay{}
	for rows.Next() {
		var day models.RoutePreviewDay
		if err := rows.Scan(&day.ServiceDate, &day.ScheduleID, &day.StartTime, &day.VersionID,
			&day.VehicleID, &day.DriverID, &day.Status, &day.Exceptions); err != nil {
			return nil, err
		}
		days = append(days, &day)
	}
	if err := rows.Err(); err != nil {
		return nil, mapExceptionError(err, trackingErrors.RoutePreviewFailed)
	}
	return days, nil
}
