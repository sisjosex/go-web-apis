package repositories

import (
	"context"
	"time"

	trackingErrors "josex/web/modules/tracking/errors"
	"josex/web/modules/tracking/models"

	"github.com/google/uuid"
)

// The reports slice of TRACK-013: the aggregates the jobs roll and the reads over them.

// SummariseTrip writes an ended trip's summary and attendance rows; false when it has not ended.
func (r *TrackingRepository) SummariseTrip(ctx context.Context, tenantID, tripID uuid.UUID) (bool, error) {
	var done bool
	err := r.dbService.QueryRow(ctx, `SELECT tracking.sp_trip_summarise(p_tenant_id := $1, p_trip_id := $2)`,
		tenantID, tripID).Scan(&done)
	return done, err
}

// SweepTripSummaries summarises the trips ended since since that have no summary or changed after it.
func (r *TrackingRepository) SweepTripSummaries(ctx context.Context, tenantID uuid.UUID, since time.Time) (int, error) {
	var n int
	err := r.dbService.QueryRow(ctx, `SELECT tracking.sp_trip_summaries_sweep(p_tenant_id := $1, p_since := $2::DATE)`,
		tenantID, since.Format(time.DateOnly)).Scan(&n)
	return n, err
}

// RollStopDelayStats recomputes the per-stop lateness over the last weeks of completed trips.
func (r *TrackingRepository) RollStopDelayStats(ctx context.Context, tenantID uuid.UUID, weeks int) (int, error) {
	var n int
	err := r.dbService.QueryRow(ctx, `SELECT tracking.sp_roll_stop_delay_stats(p_tenant_id := $1, p_weeks := $2)`,
		tenantID, weeks).Scan(&n)
	return n, err
}

// UsageForDay counts what the tenant used on day (D3).
func (r *TrackingRepository) UsageForDay(ctx context.Context, tenantID uuid.UUID, day time.Time) (models.DayUsage, error) {
	var u models.DayUsage
	err := r.dbService.QueryRow(ctx, `SELECT * FROM tracking.sp_usage_for_day(p_tenant_id := $1, p_day := $2::DATE)`,
		tenantID, day.Format(time.DateOnly)).Scan(&u.VehiclesActive, &u.TripsCompleted, &u.Positions, &u.RidersBoarded)
	return u, err
}

// reportSQL is each report's SP call; every one answers a single row (rows, summary, total_count).
var reportSQL = map[string]string{
	models.ReportAttendance: `SELECT * FROM tracking.sp_report_attendance(
		p_tenant_id := $1, p_from := $2::DATE, p_to := $3::DATE, p_organization_id := $4::UUID,
		p_route_id := $5::UUID, p_rider_id := $6::UUID, p_scope_user_id := $7, p_page := $8, p_page_size := $9)`,
	models.ReportPunctuality: `SELECT * FROM tracking.sp_report_punctuality(
		p_tenant_id := $1, p_from := $2::DATE, p_to := $3::DATE, p_route_id := $4::UUID,
		p_organization_id := $5::UUID, p_scope_user_id := $6, p_page := $7, p_page_size := $8)`,
	models.ReportFleetUsage: `SELECT * FROM tracking.sp_report_fleet_usage(
		p_tenant_id := $1, p_from := $2::DATE, p_to := $3::DATE, p_route_id := $4::UUID,
		p_organization_id := $5::UUID, p_group_by := $6, p_scope_user_id := $7, p_page := $8, p_page_size := $9)`,
	models.ReportIncidents: `SELECT * FROM tracking.sp_report_incidents(
		p_tenant_id := $1, p_from := $2::DATE, p_to := $3::DATE, p_route_id := $4::UUID,
		p_organization_id := $5::UUID, p_alert_type := $6, p_scope_user_id := $7, p_page := $8, p_page_size := $9)`,
}

// Report reads one page of a report between from and to (YYYY-MM-DD), narrowed to scopeUserID's
// organizations when set, in one round-trip.
func (r *TrackingRepository) Report(ctx context.Context, tenantID uuid.UUID, report string, q models.ReportQuery, from, to string, scopeUserID *uuid.UUID, page, pageSize int) (*models.ReportPage, error) {
	var args []any
	switch report {
	case models.ReportAttendance:
		args = []any{tenantID, from, to, q.OrganizationID, q.RouteID, q.RiderID, scopeUserID, page, pageSize}
	case models.ReportPunctuality:
		args = []any{tenantID, from, to, q.RouteID, q.OrganizationID, scopeUserID, page, pageSize}
	case models.ReportFleetUsage:
		args = []any{tenantID, from, to, q.RouteID, q.OrganizationID, q.GroupBy, scopeUserID, page, pageSize}
	case models.ReportIncidents:
		args = []any{tenantID, from, to, q.RouteID, q.OrganizationID, q.AlertType, scopeUserID, page, pageSize}
	}
	var p models.ReportPage
	if err := r.dbService.QueryRow(ctx, reportSQL[report], args...).Scan(&p.Rows, &p.Summary, &p.TotalCount); err != nil {
		return nil, scopedErr(err, trackingErrors.ReportsReadFailed)
	}
	return &p, nil
}
