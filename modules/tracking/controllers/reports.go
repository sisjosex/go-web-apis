package controllers

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"josex/web/modules/core/api"
	coreErrors "josex/web/modules/core/errors"
	"josex/web/modules/core/utils"
	trackingErrors "josex/web/modules/tracking/errors"
	"josex/web/modules/tracking/models"

	"github.com/gin-gonic/gin"
)

// The report handlers of TRACK-013: one slice, one file.

// reportStatus is the status each report code answers; a code not listed is a 500.
var reportStatus = map[string]int{
	trackingErrors.ReportsRangeTooWide:   http.StatusBadRequest,
	trackingErrors.ReportsRangeInvalid:   http.StatusBadRequest,
	trackingErrors.ReportsExportTooLarge: http.StatusRequestEntityTooLarge,
}

func reportErrorResponse(c *gin.Context, err error) error {
	if err := scopeRefused(c, err); err != nil {
		return err
	}
	var trackingErr *trackingErrors.TrackingError
	if errors.As(err, &trackingErr) {
		if status, ok := reportStatus[trackingErr.Code]; ok {
			return api.Fail(status, coreErrors.BuildErrorSingle(c, trackingErr.Code))
		}
	}
	return api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
}

// reportColumn is one CSV column: the row's key (a dot reaches into an object) and the suffix of its
// header's lang key, tracking.reports.csv.<header>.
type reportColumn struct{ key, header string }

// reportColumns is the export's allow-list: a :report not listed is a 404.
var reportColumns = map[string][]reportColumn{
	models.ReportAttendance: {
		{"rider_name", "rider_name"}, {"organization_name", "organization_name"}, {"days", "days"},
		{"boarded", "boarded"}, {"no_shows", "no_shows"}, {"absent", "absent"}, {"cancelled", "cancelled"},
		{"attendance_pct", "attendance_pct"},
	},
	models.ReportPunctuality: {
		{"route_name", "route_name"}, {"direction", "direction"}, {"organization_name", "organization_name"},
		{"trips", "trips"}, {"on_time", "on_time"}, {"on_time_pct", "on_time_pct"},
		{"avg_start_delay_min", "avg_start_delay_min"}, {"max_start_delay_min", "max_start_delay_min"},
		{"avg_stop_delay_min", "avg_stop_delay_min"}, {"worst_stop.stop_name", "worst_stop_name"},
		{"worst_stop.p90_delay_min", "worst_stop_p90_delay_min"},
	},
	models.ReportFleetUsage: {
		{"label", "vehicle"}, {"company_name", "company_name"}, {"trips", "trips"}, {"km", "km"},
		{"hours", "hours"}, {"riders_boarded", "riders_boarded"}, {"days_active", "days_active"},
		{"avg_occupancy_pct", "avg_occupancy_pct"},
	},
	models.ReportIncidents: {
		{"created_at", "created_at"}, {"route_name", "route_name"}, {"license_plate", "license_plate"},
		{"alert_type", "alert_type"}, {"severity", "severity"}, {"title", "title"}, {"message", "message"},
		{"status", "status"}, {"estimated_delay_minutes", "estimated_delay_minutes"},
		{"resolved_at", "resolved_at"}, {"created_by_name", "created_by_name"},
		{"resolved_by_name", "resolved_by_name"},
	},
}

// ReportInput is a report's request: its filters and page.
type ReportInput struct {
	Query api.Query[models.ReportQuery]
}

// report binds a report's query and answers its page; every read is narrowed to an organization
// user's organizations (D4). T is the report's shape, which the service decodes it into.
func report[T any](ctx context.Context, ctrl *TrackingController, in *ReportInput, name string) (*api.Out[T], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}
	var query models.ReportQuery
	if err := in.Query.Into(ctx, &query); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.ReportsReadFailed, utils.ExtractValidationError(c, err)))
	}
	out, err := ctrl.trackingService.Report(c.Request.Context(), tenantID, name, query, ctrl.scopeUserID(c))
	if err != nil {
		return nil, reportErrorResponse(c, err)
	}
	return api.OK(out.(T))
}

// ReportAttendance is riders with their boarded, no-show, absent and cancelled rides between from
// and to, least attended first.
func (ctrl *TrackingController) ReportAttendance(ctx context.Context, in *ReportInput) (*api.Out[*models.AttendanceReport], error) {
	return report[*models.AttendanceReport](ctx, ctrl, in, models.ReportAttendance)
}

// ReportPunctuality is routes with their completed trips on time, least punctual first.
func (ctrl *TrackingController) ReportPunctuality(ctx context.Context, in *ReportInput) (*api.Out[*models.PunctualityReport], error) {
	return report[*models.PunctualityReport](ctx, ctrl, in, models.ReportPunctuality)
}

// ReportFleetUsage is vehicles or drivers with their trips, km, hours and occupancy, most km first.
func (ctrl *TrackingController) ReportFleetUsage(ctx context.Context, in *ReportInput) (*api.Out[*models.FleetUsageReport], error) {
	return report[*models.FleetUsageReport](ctx, ctrl, in, models.ReportFleetUsage)
}

// ReportIncidents is the alerts raised in the range, newest first, with counts by status and type.
func (ctrl *TrackingController) ReportIncidents(ctx context.Context, in *ReportInput) (*api.Out[*models.IncidentsReport], error) {
	return report[*models.IncidentsReport](ctx, ctrl, in, models.ReportIncidents)
}

// ExportReportInput is ExportReport's request.
type ExportReportInput struct {
	Report string `path:"report" doc:"attendance, punctuality, fleet-usage or incidents"`
	Query  api.Query[models.ReportQuery]
}

// ExportReport answers every row the report's filters match, as UTF-8 CSV with a BOM and headers in
// the request's language; numbers unformatted, dates ISO. Over 10 000 rows is 413 (D2).
func (ctrl *TrackingController) ExportReport(ctx context.Context, in *ExportReportInput) (*api.File, error) {
	c := api.Gin(ctx)
	report := in.Report
	columns, ok := reportColumns[report]
	if !ok {
		return nil, api.Fail(http.StatusNotFound, nil)
	}
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}
	var query models.ReportQuery
	if err := in.Query.Into(ctx, &query); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.ReportsReadFailed, utils.ExtractValidationError(c, err)))
	}
	rows, from, to, err := ctrl.trackingService.ReportExport(c.Request.Context(), tenantID, report, query, ctrl.scopeUserID(c))
	if err != nil {
		return nil, reportErrorResponse(c, err)
	}
	lang := c.GetString("lang")
	header := make([]string, len(columns))
	for i, col := range columns {
		if col.key == "label" {
			col.header = query.GroupBy
		}
		header[i] = utils.GetTranslation(lang, "tracking.reports.csv."+col.header)
	}
	var buf bytes.Buffer
	// The BOM makes Excel read the accents as UTF-8.
	buf.WriteString("\xEF\xBB\xBF")
	w := csv.NewWriter(&buf)
	_ = w.Write(header)
	record := make([]string, len(columns))
	for _, row := range rows {
		for i, col := range columns {
			record[i] = csvValue(row, col.key)
		}
		_ = w.Write(record)
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return &api.File{
		ContentType:        "text/csv; charset=utf-8",
		ContentDisposition: fmt.Sprintf(`attachment; filename="%s-%s-%s.csv"`, report, from, to),
		Body:               buf.Bytes(),
	}, nil
}

// csvValue is the cell of key in row, a dot reaching into an object; empty for null or absent.
func csvValue(row map[string]any, key string) string {
	var v any = row
	for _, part := range strings.Split(key, ".") {
		obj, ok := v.(map[string]any)
		if !ok {
			return ""
		}
		v = obj[part]
	}
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}
