//go:build integration
// +build integration

package tracking_test

import (
	"bytes"
	"context"
	"encoding/csv"
	"net/http"
	"testing"
	"time"

	coreJobs "josex/web/modules/core/jobs"
	"josex/web/modules/core/testhelpers"
	tenancyRepos "josex/web/modules/tenancy/repositories"
	trackingJobs "josex/web/modules/tracking/jobs"
	trackingPerms "josex/web/modules/tracking/permissions"
	trackingServices "josex/web/modules/tracking/services"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TRACK-013: the reports' aggregates, rolled at trip close and nightly, and the reads over them.

// endedTrip is a live trip with GPS points, its rider boarded, completed and traced and summarised as
// the trip.changed handler does it. It answers the trip, its route, its rider and its vehicle.
func endedTrip(t *testing.T, helper *testhelpers.ApiTestHelper) (tripID, routeID, riderID, vehicleID string) {
	t.Helper()
	tripID, vehicleID = liveTrip(t, helper)
	StartTripMinutesAgo(t, helper, tripID, 10)
	ingest(t, helper, pointsJSON(t, vehicleID, time.Now().UTC().Add(-5*time.Minute),
		[2]float64{-17.3935, -66.1570}, [2]float64{-17.3940, -66.1560}, [2]float64{-17.3950, -66.1550}, [2]float64{-17.3962, -66.1541}))
	detail := tripRequest(t, helper, "GET", "/tracking/trips/"+tripID, nil, http.StatusOK)
	routeID = detail.RouteID.String()
	for _, stop := range detail.Stops {
		for _, task := range stop.Tasks {
			if task.Kind == "pickup" {
				riderID = task.SubjectID.String()
			}
		}
	}
	require.NotEmpty(t, riderID, "the trip has a rider")
	tripRequest(t, helper, "POST", "/tracking/trip-stop-tasks/"+taskOf(t, detail, riderID, "pickup")+"/done",
		map[string]interface{}{"client_op_id": uuid.NewString()}, http.StatusOK)
	tripRequest(t, helper, "POST", "/tracking/trips/"+tripID+"/complete", nil, http.StatusOK)

	ctx := context.Background()
	_, err := trackingJobs.ApplyTripTrace(ctx, helper.DB(), nil, testTenant(), uuid.MustParse(tripID))
	require.NoError(t, err)
	require.NoError(t, trackingJobs.SummariseTrip(ctx, helper.DB(), testTenant(), uuid.MustParse(tripID)))
	return tripID, routeID, riderID, vehicleID
}

// TestReports_SummaryAtCloseAndRollups - a completed trip is summarised with its traced km, and again
// to the same row; the nightly pass, run twice over "yesterday" = today, leaves one usage row for the
// tenant counting the vehicle that sent points.
func TestReports_SummaryAtCloseAndRollups(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	tripID, _, _, _ := endedTrip(t, helper)
	km, pickups, rolledAt := TripSummaryRow(t, helper, tripID)
	assert.Greater(t, km, 0.0, "the summary carries the trace's km")
	assert.Equal(t, 1, pickups)

	require.NoError(t, trackingJobs.SummariseTrip(context.Background(), helper.DB(), testTenant(), uuid.MustParse(tripID)))
	km2, pickups2, rolledAt2 := TripSummaryRow(t, helper, tripID)
	assert.Equal(t, km, km2)
	assert.Equal(t, pickups, pickups2)
	assert.True(t, rolledAt2.After(rolledAt), "a second roll rewrites the row")
	assert.Equal(t, 1, TripAttendanceCount(t, helper, tripID, "boarded"))

	usage := tenancyRepos.NewUsageRepository(helper.DB())
	tomorrow := time.Now().UTC().Add(24 * time.Hour)
	for range 2 {
		for _, err := range trackingJobs.RunReportRollupsPass(context.Background(), helper.DB(), usage, []coreJobs.Tenant{testTenant()}, tomorrow) {
			require.NoError(t, err)
		}
	}
	rows, vehicles := UsageDayRow(t, helper, TestTenantID, time.Now().UTC())
	assert.Equal(t, 1, rows)
	assert.GreaterOrEqual(t, vehicles, 1)
}

// reportRows reads a report's rows and summary for today, failing on anything but 200.
func reportRows(t *testing.T, helper *testhelpers.ApiTestHelper, report, filters string) ([]map[string]interface{}, map[string]interface{}) {
	t.Helper()
	today := dayOffset(t, "UTC", 0)
	w := helper.DoRequest("GET", "/tracking/reports/"+report+"?from="+today+"&to="+today+filters, nil, map[string]string{})
	expectStatus(t, w, http.StatusOK)
	var body struct {
		Rows    []map[string]interface{} `json:"rows"`
		Summary map[string]interface{}   `json:"summary"`
	}
	decodeBody(t, w, &body)
	return body.Rows, body.Summary
}

// TestReports_Reads - after one ended trip with an incident, each report names it: its rider boarded
// once, its route with one trip, its vehicle with the trip's km, the breakdown counted by type.
func TestReports_Reads(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	tripID, routeID, riderID, vehicleID := endedTrip(t, helper)
	raiseTripAlert(t, helper, routeID, tripID, "breakdown", "Flat tyre")
	km, _, _ := TripSummaryRow(t, helper, tripID)

	rows, summary := reportRows(t, helper, "attendance", "&route_id="+routeID)
	if assert.Len(t, rows, 1) {
		assert.Equal(t, riderID, rows[0]["rider_id"])
		assert.Equal(t, float64(1), rows[0]["boarded"])
		assert.Equal(t, float64(100), rows[0]["attendance_pct"])
	}
	assert.Equal(t, float64(1), summary["riders"])

	rows, summary = reportRows(t, helper, "punctuality", "&route_id="+routeID)
	if assert.Len(t, rows, 1) {
		assert.Equal(t, routeID, rows[0]["route_id"])
		assert.Equal(t, float64(1), rows[0]["trips"])
	}
	assert.Equal(t, float64(1), summary["trips"])

	rows, summary = reportRows(t, helper, "fleet-usage", "&route_id="+routeID)
	if assert.Len(t, rows, 1) {
		assert.Equal(t, vehicleID, rows[0]["id"])
		assert.Equal(t, km, rows[0]["km"])
		assert.Equal(t, float64(1), rows[0]["riders_boarded"])
	}
	assert.Equal(t, float64(1), summary["vehicles"])
	rows, _ = reportRows(t, helper, "fleet-usage", "&route_id="+routeID+"&group_by=driver")
	assert.Len(t, rows, 1, "grouped by the trip's driver")

	rows, summary = reportRows(t, helper, "incidents", "&route_id="+routeID)
	if assert.Len(t, rows, 1) {
		assert.Equal(t, "Flat tyre", rows[0]["title"])
	}
	assert.Equal(t, float64(1), summary["by_type"].(map[string]interface{})["breakdown"])
	assert.Equal(t, float64(1), summary["open"])
}

// TestReports_Refusals - a range over 366 days or ending before it starts is 400, a bad group_by 400.
func TestReports_Refusals(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	w := helper.DoRequest("GET", "/tracking/reports/attendance?from=2025-01-01&to=2026-06-01", nil, map[string]string{})
	expectStatus(t, w, http.StatusBadRequest)
	assert.Contains(t, w.Body.String(), "tracking.reports.range-too-wide")
	w = helper.DoRequest("GET", "/tracking/reports/punctuality?from=2026-06-02&to=2026-06-01", nil, map[string]string{})
	expectStatus(t, w, http.StatusBadRequest)
	assert.Contains(t, w.Body.String(), "tracking.reports.range-invalid")
	w = helper.DoRequest("GET", "/tracking/reports/fleet-usage?group_by=route", nil, map[string]string{})
	expectStatus(t, w, http.StatusBadRequest)
}

// TestReports_OrganizationScope - without the permission the organization user is refused; with it, it
// sees the trip of its own school's rider and not one whose rider belongs to another organization.
func TestReports_OrganizationScope(t *testing.T) {
	admin := SetupTrackingTest(t)
	defer admin.Close()
	org := SetupOrganizationTest(t)
	defer org.Close()

	w := org.DoRequest("GET", "/tracking/reports/attendance", nil, map[string]string{})
	expectStatus(t, w, http.StatusForbidden)

	_, ownRoute, ownRider, _ := endedTrip(t, admin)
	otherTrip, otherRoute, otherRider, _ := endedTrip(t, admin)
	MoveRiderToOrganization(t, admin, otherRider, EmptyEmployerID)
	require.NoError(t, trackingJobs.SummariseTrip(context.Background(), admin.DB(), testTenant(), uuid.MustParse(otherTrip)))
	GrantOrganizationRole(t, admin, trackingPerms.ReportsRead)

	rows, _ := reportRows(t, org, "attendance", "&route_id="+ownRoute)
	if assert.Len(t, rows, 1) {
		assert.Equal(t, ownRider, rows[0]["rider_id"])
	}
	rows, _ = reportRows(t, org, "attendance", "&route_id="+otherRoute)
	assert.Empty(t, rows, "another organization's rider")
	rows, _ = reportRows(t, org, "punctuality", "&route_id="+otherRoute)
	assert.Empty(t, rows)
	rows, _ = reportRows(t, org, "fleet-usage", "&route_id="+ownRoute)
	assert.Len(t, rows, 1)
}

// TestReports_Export - the attendance export is a UTF-8 CSV with a BOM, a header in the request's
// language and one line per rider; past the cap it is 413 before any row, and an unknown report 404.
func TestReports_Export(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	loadTrackingTranslations(t)
	_, routeID, _, _ := endedTrip(t, helper)
	today := dayOffset(t, "UTC", 0)
	path := "/tracking/reports/attendance/export?from=" + today + "&to=" + today + "&route_id=" + routeID
	w := helper.DoRequest("GET", path, nil, map[string]string{"Accept-Language": "es"})
	expectStatus(t, w, http.StatusOK)
	assert.Equal(t, "text/csv; charset=utf-8", w.Header().Get("Content-Type"))
	assert.Equal(t, `attachment; filename="attendance-`+today+`-`+today+`.csv"`, w.Header().Get("Content-Disposition"))
	raw := w.Body.Bytes()
	require.True(t, bytes.HasPrefix(raw, []byte{0xEF, 0xBB, 0xBF}), "starts with the BOM")
	records, err := csv.NewReader(bytes.NewReader(raw[3:])).ReadAll()
	require.NoError(t, err)
	if assert.Len(t, records, 2, "header and the one rider") {
		assert.Equal(t, "Pasajero", records[0][0])
		assert.Equal(t, "1", records[1][3], "boarded")
	}

	for _, report := range []string{"punctuality", "fleet-usage", "incidents"} {
		w = helper.DoRequest("GET", "/tracking/reports/"+report+"/export?from="+today+"&to="+today, nil, map[string]string{})
		expectStatus(t, w, http.StatusOK)
	}

	defer func(max int) { trackingServices.ReportExportMax = max }(trackingServices.ReportExportMax)
	trackingServices.ReportExportMax = 0
	w = helper.DoRequest("GET", path, nil, map[string]string{})
	expectStatus(t, w, http.StatusRequestEntityTooLarge)
	assert.Contains(t, w.Body.String(), "tracking.reports.export-too-large")

	w = helper.DoRequest("GET", "/tracking/reports/routes/export", nil, map[string]string{})
	expectStatus(t, w, http.StatusNotFound)
}
