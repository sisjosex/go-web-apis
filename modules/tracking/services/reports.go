package services

import (
	"bytes"
	"context"
	"encoding/json"
	"time"

	trackingErrors "josex/web/modules/tracking/errors"
	"josex/web/modules/tracking/models"

	"github.com/google/uuid"
)

// The reports of TRACK-013.

// ReportExportMax is the most rows an export writes (D2); a variable so a test can lower it.
var ReportExportMax = 10000

const (
	reportDefaultDays = 30
	reportMaxDays     = 366
)

// reportRange is the query's from..to, the last 30 days (UTC) up to today by default; a range ending
// before it starts or spanning more than 366 days is refused.
func reportRange(q models.ReportQuery, now time.Time) (string, string, error) {
	to := now.UTC().Truncate(24 * time.Hour)
	if q.To != nil {
		to, _ = time.Parse(time.DateOnly, *q.To)
	}
	from := to.AddDate(0, 0, 1-reportDefaultDays)
	if q.From != nil {
		from, _ = time.Parse(time.DateOnly, *q.From)
	}
	switch {
	case to.Before(from):
		return "", "", &trackingErrors.TrackingError{Code: trackingErrors.ReportsRangeInvalid}
	case to.Sub(from) > reportMaxDays*24*time.Hour:
		return "", "", &trackingErrors.TrackingError{Code: trackingErrors.ReportsRangeTooWide}
	}
	return from.Format(time.DateOnly), to.Format(time.DateOnly), nil
}

// Report reads one page of a report and decodes it into the report's own shape.
func (s *TrackingService) Report(ctx context.Context, tenantID uuid.UUID, report string, q models.ReportQuery, scopeUserID *uuid.UUID) (any, error) {
	from, to, err := reportRange(q, time.Now())
	if err != nil {
		return nil, err
	}
	page, err := s.trackingRepo.Report(ctx, tenantID, report, q, from, to, scopeUserID, q.Page, q.PageSize)
	if err != nil {
		return nil, err
	}
	switch report {
	case models.ReportAttendance:
		out := &models.AttendanceReport{TotalCount: page.TotalCount, Page: q.Page, PageSize: q.PageSize}
		return out, decodeReport(page, &out.Rows, &out.Summary)
	case models.ReportPunctuality:
		out := &models.PunctualityReport{TotalCount: page.TotalCount, Page: q.Page, PageSize: q.PageSize}
		return out, decodeReport(page, &out.Rows, &out.Summary)
	case models.ReportFleetUsage:
		out := &models.FleetUsageReport{TotalCount: page.TotalCount, Page: q.Page, PageSize: q.PageSize}
		return out, decodeReport(page, &out.Rows, &out.Summary)
	default:
		out := &models.IncidentsReport{TotalCount: page.TotalCount, Page: q.Page, PageSize: q.PageSize}
		return out, decodeReport(page, &out.Rows, &out.Summary)
	}
}

// ReportExport reads every row a report's filters match, as the SP wrote them (numbers kept exact),
// with the range they cover; more than ReportExportMax rows is refused before any is decoded.
func (s *TrackingService) ReportExport(ctx context.Context, tenantID uuid.UUID, report string, q models.ReportQuery, scopeUserID *uuid.UUID) ([]map[string]any, string, string, error) {
	from, to, err := reportRange(q, time.Now())
	if err != nil {
		return nil, "", "", err
	}
	page, err := s.trackingRepo.Report(ctx, tenantID, report, q, from, to, scopeUserID, 1, ReportExportMax+1)
	if err != nil {
		return nil, "", "", err
	}
	if page.TotalCount > int64(ReportExportMax) {
		return nil, "", "", &trackingErrors.TrackingError{Code: trackingErrors.ReportsExportTooLarge}
	}
	var rows []map[string]any
	dec := json.NewDecoder(bytes.NewReader(page.Rows))
	dec.UseNumber()
	if err := dec.Decode(&rows); err != nil {
		return nil, "", "", &trackingErrors.TrackingError{Code: trackingErrors.ReportsReadFailed, Err: err}
	}
	return rows, from, to, nil
}

func decodeReport(page *models.ReportPage, rows, summary any) error {
	if err := json.Unmarshal(page.Rows, rows); err != nil {
		return &trackingErrors.TrackingError{Code: trackingErrors.ReportsReadFailed, Err: err}
	}
	if err := json.Unmarshal(page.Summary, summary); err != nil {
		return &trackingErrors.TrackingError{Code: trackingErrors.ReportsReadFailed, Err: err}
	}
	return nil
}
