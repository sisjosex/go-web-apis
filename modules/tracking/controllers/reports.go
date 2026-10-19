package controllers

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"net/http"
	"strings"

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

func reportErrorResponse(c *gin.Context, err error) {
	if scopeRefused(c, err) {
		return
	}
	var trackingErr *trackingErrors.TrackingError
	if errors.As(err, &trackingErr) {
		if status, ok := reportStatus[trackingErr.Code]; ok {
			c.JSON(status, coreErrors.BuildErrorSingle(c, trackingErr.Code))
			return
		}
	}
	c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
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

// report binds a report's query and answers its page; every read is narrowed to an organization
// user's organizations (D4).
func (ctrl *TrackingController) report(c *gin.Context, report string) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}
	var query models.ReportQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.ReportsReadFailed, utils.ExtractValidationError(c, err)))
		return
	}
	out, err := ctrl.trackingService.Report(c.Request.Context(), tenantID, report, query, ctrl.scopeUserID(c))
	if err != nil {
		reportErrorResponse(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

// ReportAttendance godoc
// @Summary Attendance report
// @Description Riders with their boarded, no-show, absent and cancelled rides between from and to (default the last 30 days, at most 366), least attended first. attendance_pct is boarded over boarded + no-shows + absent; null when none
// @Tags Tracking - Reports
// @Produce json
// @Security BearerAuth
// @Param from query string false "First day (YYYY-MM-DD)"
// @Param to query string false "Last day (YYYY-MM-DD)"
// @Param organization_id query string false "Organization ID (UUID)"
// @Param route_id query string false "Route ID (UUID)"
// @Param rider_id query string false "Rider ID (UUID)"
// @Param page query int false "Page number (default 1)"
// @Param page_size query int false "Page size (default 20, max 100)"
// @Success 200 {object} models.AttendanceReport
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 403 {object} coreErrors.ErrorResponse
// @Router /tracking/reports/attendance [get]
func (ctrl *TrackingController) ReportAttendance(c *gin.Context) {
	ctrl.report(c, models.ReportAttendance)
}

// ReportPunctuality godoc
// @Summary Punctuality report
// @Description Routes with their completed trips on time (started within 5 minutes of plan and no stop over 5 minutes late), start and stop delays and the stop reached latest by p90, least punctual first
// @Tags Tracking - Reports
// @Produce json
// @Security BearerAuth
// @Param from query string false "First day (YYYY-MM-DD)"
// @Param to query string false "Last day (YYYY-MM-DD)"
// @Param route_id query string false "Route ID (UUID)"
// @Param organization_id query string false "Organization ID (UUID)"
// @Param page query int false "Page number (default 1)"
// @Param page_size query int false "Page size (default 20, max 100)"
// @Success 200 {object} models.PunctualityReport
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 403 {object} coreErrors.ErrorResponse
// @Router /tracking/reports/punctuality [get]
func (ctrl *TrackingController) ReportPunctuality(c *gin.Context) {
	ctrl.report(c, models.ReportPunctuality)
}

// ReportFleetUsage godoc
// @Summary Fleet usage report
// @Description Vehicles (group_by=vehicle, label the plate) or drivers (driver, label the name) with their completed trips, km, hours, riders boarded, days out and mean peak occupancy, most km first
// @Tags Tracking - Reports
// @Produce json
// @Security BearerAuth
// @Param from query string false "First day (YYYY-MM-DD)"
// @Param to query string false "Last day (YYYY-MM-DD)"
// @Param route_id query string false "Route ID (UUID)"
// @Param organization_id query string false "Organization ID (UUID)"
// @Param group_by query string false "vehicle (default) or driver"
// @Param page query int false "Page number (default 1)"
// @Param page_size query int false "Page size (default 20, max 100)"
// @Success 200 {object} models.FleetUsageReport
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 403 {object} coreErrors.ErrorResponse
// @Router /tracking/reports/fleet-usage [get]
func (ctrl *TrackingController) ReportFleetUsage(c *gin.Context) {
	ctrl.report(c, models.ReportFleetUsage)
}

// ReportIncidents godoc
// @Summary Incidents report
// @Description The alerts raised between from and to (UTC days), newest first, in GET /tracking/alerts' row shape, with counts by status and type and the mean minutes to resolve
// @Tags Tracking - Reports
// @Produce json
// @Security BearerAuth
// @Param from query string false "First day (YYYY-MM-DD)"
// @Param to query string false "Last day (YYYY-MM-DD)"
// @Param route_id query string false "Route ID (UUID)"
// @Param organization_id query string false "Organization ID (UUID)"
// @Param alert_type query string false "delay, breakdown, cancellation, emergency or other"
// @Param page query int false "Page number (default 1)"
// @Param page_size query int false "Page size (default 20, max 100)"
// @Success 200 {object} models.IncidentsReport
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 403 {object} coreErrors.ErrorResponse
// @Router /tracking/reports/incidents [get]
func (ctrl *TrackingController) ReportIncidents(c *gin.Context) {
	ctrl.report(c, models.ReportIncidents)
}

// ExportReport godoc
// @Summary Export a report as CSV
// @Description Every row the report's filters match, as UTF-8 CSV with a BOM and headers in the request's language; numbers unformatted, dates ISO. Over 10 000 rows is 413 (D2)
// @Tags Tracking - Reports
// @Produce text/csv
// @Security BearerAuth
// @Param report path string true "attendance, punctuality, fleet-usage or incidents"
// @Success 200 {file} string
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 413 {object} coreErrors.ErrorResponse
// @Router /tracking/reports/{report}/export [get]
func (ctrl *TrackingController) ExportReport(c *gin.Context) {
	report := c.Param("report")
	columns, ok := reportColumns[report]
	if !ok {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}
	var query models.ReportQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.ReportsReadFailed, utils.ExtractValidationError(c, err)))
		return
	}
	rows, from, to, err := ctrl.trackingService.ReportExport(c.Request.Context(), tenantID, report, query, ctrl.scopeUserID(c))
	if err != nil {
		reportErrorResponse(c, err)
		return
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
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s-%s-%s.csv"`, report, from, to))
	c.Data(http.StatusOK, "text/csv; charset=utf-8", buf.Bytes())
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
