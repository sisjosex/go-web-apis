package repositories

import (
	"context"
	"errors"

	trackingErrors "josex/web/modules/tracking/errors"
	"josex/web/modules/tracking/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

// The alerts slice of TRACK-004 — list, resolve — and a trip's event log: one slice, one file.

// alertCodes turns the alert SPs' own refusals into module codes.
var alertCodes = map[string]string{
	"alert.not-found":        trackingErrors.AlertNotFound,
	"alert.already-resolved": trackingErrors.AlertAlreadyResolved,
}

func mapAlertError(err error, fallbackCode string) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		if code, ok := alertCodes[pgErr.Message]; ok {
			return &trackingErrors.TrackingError{Code: code, Err: pgErr}
		}
	}
	return &trackingErrors.TrackingError{Code: fallbackCode, Err: err}
}

// scanAlert is the one scan order fn_route_alert_rows' shape answers in.
func scanAlert(a *models.RouteAlert) []any {
	return []any{
		&a.ID, &a.RouteID, &a.RouteName, &a.VehicleID, &a.LicensePlate, &a.TripID, &a.AlertType, &a.Title,
		&a.Message, &a.Severity, &a.EstimatedDelayMinutes, &a.Status, &a.Lat, &a.Lng, &a.CreatedBy,
		&a.CreatedByName, &a.CreatedAt, &a.ResolvedAt, &a.ResolvedBy, &a.ResolvedByName,
	}
}

// ListRouteAlerts returns one page of the tenant's alerts, newest first, plus the total the same
// filters match. status "all" reads every status.
func (r *TrackingRepository) ListRouteAlerts(ctx context.Context, tenantID uuid.UUID, query models.ListAlertsQuery) ([]*models.RouteAlert, int64, error) {
	var status *string
	if query.Status != "all" {
		status = &query.Status
	}
	rows, err := r.dbService.Query(ctx, `
		SELECT * FROM tracking.sp_list_route_alerts(
			p_tenant_id := $1,
			p_route_id  := $2::UUID,
			p_status    := $3,
			p_severity  := $4,
			p_page      := $5,
			p_page_size := $6
		)
	`, tenantID, query.RouteID, status, query.Severity, query.Page, query.PageSize)
	if err != nil {
		return nil, 0, &trackingErrors.TrackingError{Code: trackingErrors.AlertListFailed, Err: err}
	}
	defer rows.Close()
	alerts := []*models.RouteAlert{}
	var totalCount int64
	for rows.Next() {
		var a models.RouteAlert
		if err := rows.Scan(append(scanAlert(&a), &totalCount)...); err != nil {
			return nil, 0, &trackingErrors.TrackingError{Code: trackingErrors.AlertListFailed, Err: err}
		}
		alerts = append(alerts, &a)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, &trackingErrors.TrackingError{Code: trackingErrors.AlertListFailed, Err: err}
	}
	return alerts, totalCount, nil
}

// ResolveRouteAlert resolves an active alert and answers it as the list does.
func (r *TrackingRepository) ResolveRouteAlert(ctx context.Context, tenantID, alertID uuid.UUID, userID *uuid.UUID) (*models.RouteAlert, error) {
	var a models.RouteAlert
	err := r.dbService.QueryRow(ctx,
		`SELECT * FROM tracking.sp_resolve_route_alert(p_tenant_id := $1, p_alert_id := $2, p_user_id := $3)`,
		tenantID, alertID, userID,
	).Scan(scanAlert(&a)...)
	if err != nil {
		return nil, mapAlertError(err, trackingErrors.AlertResolveFailed)
	}
	return &a, nil
}

// ListTripEvents answers a trip's events oldest first.
func (r *TrackingRepository) ListTripEvents(ctx context.Context, tenantID, tripID uuid.UUID) ([]*models.TripEvent, error) {
	rows, err := r.dbService.Query(ctx,
		`SELECT * FROM tracking.sp_list_trip_events(p_tenant_id := $1, p_trip_id := $2)`, tenantID, tripID)
	if err != nil {
		return nil, mapTripError(err, trackingErrors.TripEventsFailed)
	}
	defer rows.Close()
	events := []*models.TripEvent{}
	for rows.Next() {
		var e models.TripEvent
		if err := rows.Scan(&e.ID, &e.Type, &e.Payload, &e.CreatedAt, &e.CreatedByName); err != nil {
			return nil, mapTripError(err, trackingErrors.TripEventsFailed)
		}
		events = append(events, &e)
	}
	if err := rows.Err(); err != nil {
		return nil, mapTripError(err, trackingErrors.TripEventsFailed)
	}
	return events, nil
}
