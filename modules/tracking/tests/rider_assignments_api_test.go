//go:build integration
// +build integration

package tracking_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"josex/web/modules/core/testhelpers"
)

// ============================================================================
// RIDER ROUTE ASSIGNMENTS (TRACK-009)
// ============================================================================

const (
	MonToWedMask = 7
	ThuToFriMask = 24
	// AThursday is inside every assignment these tests make (all start 2026-01-01, open-ended).
	AThursday = "2026-10-01"
)

// expectStatus stops the test when a call a later step depends on did not answer want.
func expectStatus(t *testing.T, w *httptest.ResponseRecorder, want int) {
	t.Helper()
	if w.Code != want {
		t.Fatalf("expected %d, got %d: %s", want, w.Code, w.Body.String())
	}
}

// listAssignments reads GET /tracking/assignments with a raw query string.
func listAssignments(t *testing.T, helper *testhelpers.ApiTestHelper, query string) map[string]interface{} {
	t.Helper()
	w := helper.DoRequest("GET", "/tracking/assignments?"+query, nil, map[string]string{})
	expectStatus(t, w, http.StatusOK)
	return ParseResponse(t, w.Body.Bytes())
}

// assignmentCount is how many assignment rows the database holds for a rider.
func assignmentCount(t *testing.T, helper *testhelpers.ApiTestHelper, riderID string) int {
	t.Helper()
	var n int
	err := helper.DB().QueryRow(context.Background(),
		`SELECT count(*) FROM tracking.rider_route_assignments WHERE rider_id = $1`, riderID).Scan(&n)
	if err != nil {
		t.Fatalf("count assignments: %v", err)
	}
	return n
}

// TestAssignments_DaysSplitAcrossRoutes - A Mon–Wed then B Thu–Fri → both 201, a Thursday lists B only
func TestAssignments_DaysSplitAcrossRoutes(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	riderID := CreateTestRider(t, helper)
	routeA := CreateTestRoute(t, helper)
	routeB := CreateTestRoute(t, helper)

	a := CreateAssignment(t, helper, AssignmentBody(riderID, routeA, MonToWedMask))
	b := CreateAssignment(t, helper, AssignmentBody(riderID, routeB, ThuToFriMask))
	assert.Equal(t, "outbound", a["direction"])
	assert.NotEmpty(t, b["rider_name"])
	assert.NotEmpty(t, b["route_name"])

	page := listAssignments(t, helper, fmt.Sprintf("rider_id=%s&date=%s", riderID, AThursday))
	rows := page["assignments"].([]interface{})
	if !assert.Len(t, rows, 1) {
		t.FailNow()
	}
	assert.Equal(t, routeB, rows[0].(map[string]interface{})["route_id"])
	assert.EqualValues(t, 1, page["total_count"])
	assert.EqualValues(t, 1, page["page"])
	assert.EqualValues(t, 20, page["page_size"])

	all := listAssignments(t, helper, "rider_id="+riderID)
	assert.EqualValues(t, 2, all["total_count"])
}

// TestAssignments_Overlap - A Mon–Wed then A Mon–Fri → 409; the other direction the same days → 201
func TestAssignments_Overlap(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	riderID := CreateTestRider(t, helper)
	routeA := CreateTestRoute(t, helper)
	CreateAssignment(t, helper, AssignmentBody(riderID, routeA, MonToWedMask))

	w := helper.DoRequest("POST", "/tracking/assignments", AssignmentBody(riderID, routeA, WeekdaysMask), map[string]string{})
	expectStatus(t, w, http.StatusConflict)
	assert.Equal(t, "tracking.assignment.overlap", ParseResponse(t, w.Body.Bytes())["error"])

	// Ended before the first one starts: the ranges do not meet.
	past := AssignmentBody(riderID, routeA, WeekdaysMask)
	past["valid_from"], past["valid_until"] = "2025-01-01", "2025-12-31"
	CreateAssignment(t, helper, past)

	routeBack := CreateTestRoute(t, helper)
	SetRouteDirection(t, helper, routeBack, "inbound")
	CreateAssignment(t, helper, AssignmentBody(riderID, routeBack, WeekdaysMask))
}

// TestAssignments_CapacityWarnings - capacity 2, three riders every day → one warning per service_date
func TestAssignments_CapacityWarnings(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	vehicle := ValidVehicleDto()
	w := helper.DoRequest("POST", "/tracking/vehicles", map[string]interface{}{
		"company_id": vehicle.CompanyID, "plate_number": vehicle.PlateNumber, "vehicle_type": vehicle.VehicleType,
		"capacity": 2, "status": "active",
	}, map[string]string{})
	expectStatus(t, w, http.StatusCreated)
	vehicleID := ExtractID(t, ParseResponse(t, w.Body.Bytes()))

	routeID := CreateTestRoute(t, helper)
	w = helper.DoRequest("PATCH", "/tracking/routes/"+routeID, map[string]interface{}{"vehicle_id": vehicleID}, map[string]string{})
	expectStatus(t, w, http.StatusOK)
	// Two departures a day on the same vehicle still make one warning per day.
	for _, start := range []string{"07:00", "15:00"} {
		CreateRouteSchedule(t, helper, routeID, map[string]interface{}{
			"days_of_week": EveryDayMask, "start_time": start, "valid_from": "2026-01-01",
		})
	}

	var body map[string]interface{}
	for i := 0; i < 3; i++ {
		w = helper.DoRequest("POST", "/tracking/assignments",
			AssignmentBody(CreateTestRider(t, helper), routeID, EveryDayMask), map[string]string{})
		expectStatus(t, w, http.StatusCreated)
		body = ParseResponse(t, w.Body.Bytes())
		if i < 2 {
			assert.Empty(t, body["warnings"], "within capacity")
		}
	}

	warnings := body["warnings"].([]interface{})
	if !assert.Len(t, warnings, 15, "today..today+14") {
		t.FailNow()
	}
	days := map[string]bool{}
	for _, raw := range warnings {
		warning := raw.(map[string]interface{})
		days[warning["service_date"].(string)] = true
		assert.Equal(t, routeID, warning["route_id"])
		assert.Equal(t, vehicleID, warning["vehicle_id"])
		assert.EqualValues(t, 2, warning["capacity"])
		assert.EqualValues(t, 3, warning["assigned"])
	}
	assert.Len(t, days, 15)
}

// TestAssignments_BulkRollsBackOnOverlap - the second item overlaps the first → 409 index 1, nothing inserted
func TestAssignments_BulkRollsBackOnOverlap(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	riderID := CreateTestRider(t, helper)
	routeID := CreateTestRoute(t, helper)
	items := []map[string]interface{}{
		AssignmentBody(riderID, routeID, MonToWedMask),
		AssignmentBody(riderID, routeID, WeekdaysMask),
	}

	w := helper.DoRequest("POST", "/tracking/assignments/bulk", items, map[string]string{})
	expectStatus(t, w, http.StatusConflict)
	body := ParseResponse(t, w.Body.Bytes())
	assert.Equal(t, "tracking.assignment.overlap", body["error"])
	assert.EqualValues(t, 1, body["detail"].(map[string]interface{})["index"])
	assert.Equal(t, 0, assignmentCount(t, helper, riderID))

	items[1]["days_of_week"] = ThuToFriMask
	w = helper.DoRequest("POST", "/tracking/assignments/bulk", items, map[string]string{})
	expectStatus(t, w, http.StatusCreated)
	assert.Len(t, ParseResponse(t, w.Body.Bytes())["assignments"], 2)
	assert.Equal(t, 2, assignmentCount(t, helper, riderID))
}

// TestAssignments_StopNotOnRoute - a stop the route's version does not call at → 400
func TestAssignments_StopNotOnRoute(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	body := AssignmentBody(CreateTestRider(t, helper), MorningRouteID, WeekdaysMask)
	body["pickup_stop_place_id"] = CentralStationID
	body["dropoff_stop_place_id"] = SchoolAStopID
	a := CreateAssignment(t, helper, body)
	assert.Equal(t, "Central Station", a["pickup_stop_name"])
	assert.Equal(t, "School A", a["dropoff_stop_name"])

	body = AssignmentBody(CreateTestRider(t, helper), AfternoonRouteID, WeekdaysMask)
	body["dropoff_stop_place_id"] = DowntownTerminalID
	w := helper.DoRequest("POST", "/tracking/assignments", body, map[string]string{})
	expectStatus(t, w, http.StatusBadRequest)
	assert.Equal(t, "tracking.assignment.stop-not-on-route", ParseResponse(t, w.Body.Bytes())["error"])
}

// TestAssignments_UpdateAndDelete - PATCH keeps what it is not sent and re-checks overlap; DELETE → 204
func TestAssignments_UpdateAndDelete(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	riderID := CreateTestRider(t, helper)
	routeID := CreateTestRoute(t, helper)
	a := CreateAssignment(t, helper, AssignmentBody(riderID, routeID, MonToWedMask))
	CreateAssignment(t, helper, AssignmentBody(riderID, routeID, ThuToFriMask))
	path := "/tracking/assignments/" + a["id"].(string)

	w := helper.DoRequest("PATCH", path, map[string]interface{}{"valid_until": "2026-12-31"}, map[string]string{})
	expectStatus(t, w, http.StatusOK)
	updated := ParseResponse(t, w.Body.Bytes())
	assert.Equal(t, "2026-12-31", updated["assignment"].(map[string]interface{})["valid_until"])
	assert.EqualValues(t, MonToWedMask, updated["assignment"].(map[string]interface{})["days_of_week"])
	assert.NotNil(t, updated["warnings"])

	w = helper.DoRequest("PATCH", path, map[string]interface{}{"days_of_week": WeekdaysMask}, map[string]string{})
	assert.Equal(t, http.StatusConflict, w.Code, w.Body.String())

	w = helper.DoRequest("PATCH", path, map[string]interface{}{"valid_until": "2025-01-01"}, map[string]string{})
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Equal(t, "tracking.assignment.range", ParseResponse(t, w.Body.Bytes())["error"])

	w = helper.DoRequest("DELETE", path, nil, map[string]string{})
	assert.Equal(t, http.StatusNoContent, w.Code, w.Body.String())
	w = helper.DoRequest("DELETE", path, nil, map[string]string{})
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
}

// TestAssignments_ListOtherTenant - another tenant's assignment on the same route is not listed
func TestAssignments_ListOtherTenant(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	routeID := CreateTestRoute(t, helper)
	CreateAssignment(t, helper, AssignmentBody(CreateTestRider(t, helper), routeID, WeekdaysMask))
	if _, err := helper.DB().Execute(context.Background(), `
		WITH o AS (
			INSERT INTO tracking.organizations (tenant_id, kind, name) VALUES ($1, 'school', 'Elsewhere') RETURNING id
		), r AS (
			INSERT INTO tracking.riders (organization_id, first_name, last_name) SELECT o.id, 'Foreign', 'Rider' FROM o RETURNING id
		)
		INSERT INTO tracking.rider_route_assignments (rider_id, route_id, days_of_week, valid_from)
		SELECT r.id, $2, 127, DATE '2026-01-01' FROM r`, uuid.New(), routeID); err != nil {
		t.Fatalf("seed a foreign assignment: %v", err)
	}

	page := listAssignments(t, helper, "route_id="+routeID)

	assert.EqualValues(t, 1, page["total_count"])
	assert.Len(t, page["assignments"], 1)
}

// TestAssignments_NotFoundAndValidation - unknown rider or route → 404; a malformed body or query → 400
func TestAssignments_NotFoundAndValidation(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	w := helper.DoRequest("POST", "/tracking/assignments", AssignmentBody(uuid.New().String(), MorningRouteID, WeekdaysMask), map[string]string{})
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())

	w = helper.DoRequest("POST", "/tracking/assignments", AssignmentBody(TestRiderJohnID, uuid.New().String(), WeekdaysMask), map[string]string{})
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())

	w = helper.DoRequest("POST", "/tracking/assignments", AssignmentBody(TestRiderJohnID, MorningRouteID, 0), map[string]string{})
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())

	w = helper.DoRequest("PATCH", "/tracking/assignments/"+uuid.New().String(), map[string]interface{}{"days_of_week": 1}, map[string]string{})
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())

	w = helper.DoRequest("GET", "/tracking/assignments?date=not-a-date", nil, map[string]string{})
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())

	w = helper.DoRequest("POST", "/tracking/assignments/bulk", []map[string]interface{}{}, map[string]string{})
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
}
