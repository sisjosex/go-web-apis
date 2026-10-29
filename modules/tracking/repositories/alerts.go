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
	"github.com/jackc/pgx/v5/pgconn"
)

// The alerts slice of TRACK-004 — list, resolve — and a trip's event log: one slice, one file.

// alertCodes turns the alert SPs' own refusals into module codes.
var alertCodes = map[string]string{
	"alert.not-found":        trackingErrors.AlertNotFound,
	"alert.already-resolved": trackingErrors.AlertAlreadyResolved,
	"alert.settings-invalid": trackingErrors.AlertSettingsInvalid,
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

// alertRow scans fn_route_alert_rows' shape. The list's empty page answers one row with every alert
// column NULL (only the summary set), so the columns an alert always has scan through pointers first.
type alertRow struct {
	alert     models.RouteAlert
	id        *uuid.UUID
	alertType *string
	title     *string
	status    *string
	createdAt *time.Time
	notified  *int32
}

func (s *alertRow) targets() []any {
	a := &s.alert
	return []any{
		&s.id, &a.RouteID, &a.RouteName, &a.VehicleID, &a.LicensePlate, &a.TripID, &s.alertType, &s.title,
		&a.Message, &a.Severity, &a.EstimatedDelayMinutes, &s.status, &a.Lat, &a.Lng, &a.CreatedBy,
		&a.CreatedByName, &s.createdAt, &a.ResolvedAt, &a.ResolvedBy, &a.ResolvedByName,
		&a.OrganizationName, &a.OrganizationKind, &a.DriverName, &a.DriverPhone, &s.notified,
	}
}

// row is the scanned alert, or nil for the empty page's summary-only row.
func (s *alertRow) row() *models.RouteAlert {
	if s.id == nil {
		return nil
	}
	a := s.alert
	a.ID = *s.id
	a.AlertType, a.Title, a.Status, a.CreatedAt = deref(s.alertType), deref(s.title), deref(s.status), *s.createdAt
	if s.notified != nil {
		a.NotifiedCount = *s.notified
	}
	return &a
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// ListRouteAlerts returns one page of the tenant's alerts, newest first, the total the same filters
// match and the board's summary over every filter but status. status "all" reads every status.
func (r *TrackingRepository) ListRouteAlerts(ctx context.Context, tenantID uuid.UUID, query models.ListAlertsQuery) ([]*models.RouteAlert, int64, models.AlertsSummary, error) {
	var summary models.AlertsSummary
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
			p_page_size := $6,
			p_search    := $7
		)
	`, tenantID, query.RouteID, status, query.Severity, query.Page, query.PageSize, query.Search)
	if err != nil {
		return nil, 0, summary, &trackingErrors.TrackingError{Code: trackingErrors.AlertListFailed, Err: err}
	}
	defer rows.Close()
	alerts := []*models.RouteAlert{}
	var totalCount int64
	for rows.Next() {
		var scan alertRow
		var raw []byte
		if err := rows.Scan(append(scan.targets(), &totalCount, &raw)...); err != nil {
			return nil, 0, summary, &trackingErrors.TrackingError{Code: trackingErrors.AlertListFailed, Err: err}
		}
		if err := json.Unmarshal(raw, &summary); err != nil {
			return nil, 0, summary, &trackingErrors.TrackingError{Code: trackingErrors.AlertListFailed, Err: err}
		}
		if a := scan.row(); a != nil {
			alerts = append(alerts, a)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, 0, summary, &trackingErrors.TrackingError{Code: trackingErrors.AlertListFailed, Err: err}
	}
	return alerts, totalCount, summary, nil
}

// AlertSettings answers the tenant's alert thresholds, the defaults when it never set them (TRACK-071).
func (r *TrackingRepository) AlertSettings(ctx context.Context, tenantID uuid.UUID) (*models.AlertSettings, error) {
	var s models.AlertSettings
	err := r.dbService.QueryRow(ctx, `SELECT * FROM tracking.sp_alert_settings_get(p_tenant_id := $1)`, tenantID).
		Scan(&s.DeviceSilentMin, &s.OffRouteM, &s.OffRouteFixes)
	if err != nil {
		return nil, mapAlertError(err, trackingErrors.AlertSettingsFailed)
	}
	return &s, nil
}

// SaveAlertSettings stores the tenant's alert thresholds and answers them.
func (r *TrackingRepository) SaveAlertSettings(ctx context.Context, tenantID uuid.UUID, in models.AlertSettings) (*models.AlertSettings, error) {
	var s models.AlertSettings
	err := r.dbService.QueryRow(ctx,
		`SELECT * FROM tracking.sp_alert_settings_put(p_tenant_id := $1, p_device_silent_min := $2, p_off_route_m := $3, p_off_route_fixes := $4)`,
		tenantID, in.DeviceSilentMin, in.OffRouteM, in.OffRouteFixes).
		Scan(&s.DeviceSilentMin, &s.OffRouteM, &s.OffRouteFixes)
	if err != nil {
		return nil, mapAlertError(err, trackingErrors.AlertSettingsFailed)
	}
	return &s, nil
}

// ResolveRouteAlert resolves an active alert and answers it as the list does.
func (r *TrackingRepository) ResolveRouteAlert(ctx context.Context, tenantID, alertID uuid.UUID, userID *uuid.UUID) (*models.RouteAlert, error) {
	var scan alertRow
	err := r.dbService.QueryRow(ctx,
		`SELECT * FROM tracking.sp_resolve_route_alert(p_tenant_id := $1, p_alert_id := $2, p_user_id := $3)`,
		tenantID, alertID, userID,
	).Scan(scan.targets()...)
	if err != nil {
		return nil, mapAlertError(err, trackingErrors.AlertResolveFailed)
	}
	return scan.row(), nil
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

// RaiseDeviceSilentAlerts is the minute sweep (TRACK-071 D2): one device_silent alert per trip in
// progress whose vehicle went quiet past the tenant's threshold; answers the alerts raised.
func (r *TrackingRepository) RaiseDeviceSilentAlerts(ctx context.Context, tenantID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := r.dbService.Query(ctx, `SELECT alert_id FROM tracking.sp_raise_device_silent_alerts(p_tenant_id := $1)`, tenantID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
}

// RouteAlert answers one alert in the list's row shape, nil when the tenant has none by that id.
// AlertClientUsers answers who of the client hears a driver's incident, with its kind, minutes and route
// (MOBILE-037 D2); none when the driver switched notify_families off.
func (r *TrackingRepository) AlertClientUsers(ctx context.Context, tenantID, alertID uuid.UUID) ([]models.AlertClientUser, error) {
	rows, err := r.dbService.Query(ctx, `SELECT * FROM tracking.sp_alert_client_users(p_tenant_id := $1, p_alert_id := $2)`, tenantID, alertID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []models.AlertClientUser
	for rows.Next() {
		var u models.AlertClientUser
		if err := rows.Scan(&u.UserID, &u.AlertType, &u.DelayMin, &u.RouteName); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (r *TrackingRepository) RouteAlert(ctx context.Context, tenantID, alertID uuid.UUID) (*models.RouteAlert, error) {
	var scan alertRow
	err := r.dbService.QueryRow(ctx, `SELECT * FROM tracking.fn_route_alert_rows(p_tenant_id := $1, p_ids := ARRAY[$2::UUID])`,
		tenantID, alertID).Scan(scan.targets()...)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return scan.row(), nil
}
