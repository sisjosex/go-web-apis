//go:build integration
// +build integration

package tracking_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"josex/web/modules/core/testhelpers"
	"josex/web/modules/tracking/models"
)

// ============================================================================
// STOPLESS ROUTES AND RIDERLESS TRIPS (TRACK-053)
// ============================================================================

// clearStopsFrom publishes a version with no stop from date on, as removing the route's last stop does.
func clearStopsFrom(t *testing.T, helper *testhelpers.ApiTestHelper, routeID, date string) {
	t.Helper()
	PublishRouteVersion(t, helper, routeID, date, []map[string]interface{}{})
}

// listTrips reads the board of date for one route, with the given extra query.
func listTrips(t *testing.T, helper *testhelpers.ApiTestHelper, routeID, date, extra string) models.ListTripsResponse {
	t.Helper()
	w := helper.DoRequest("GET", "/tracking/trips?date="+date+"&route_id="+routeID+extra, nil, map[string]string{})
	if w.Code != http.StatusOK {
		t.Fatalf("list trips: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var body models.ListTripsResponse
	decodeBody(t, w, &body)
	return body
}

// TestMaterialise_StoplessVersionMakesNoTrips - D3: a route that runs every day on a version without
// a single stop materialises no trip.
func TestMaterialise_StoplessVersionMakesNoTrips(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	routeID := CreateTestRoute(t, helper)
	PublishRouteVersion(t, helper, routeID, "2026-01-01", []map[string]interface{}{})
	CreateRouteSchedule(t, helper, routeID, map[string]interface{}{
		"days_of_week": EveryDayMask, "start_time": "07:00", "valid_from": "2026-01-01",
	})

	today := dayOffset(t, "UTC", 0)
	materialiseRoute(t, helper, routeID, today, today)

	assert.Equal(t, 0, routeTripShape(t, helper, routeID).Trips)
}

// TestMaterialise_LastStopRemovedCancelsPlanned - D3: emptying the version cancels the planned trip
// in the same pass.
func TestMaterialise_LastStopRemovedCancelsPlanned(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	routeID, _ := createTripRoute(t, helper, "07:00")
	today := dayOffset(t, "UTC", 0)
	materialiseRoute(t, helper, routeID, today, today)

	clearStopsFrom(t, helper, routeID, today)
	materialiseRoute(t, helper, routeID, today, today)

	_, status := tripOn(t, helper, routeID, today, "07:00")
	assert.Equal(t, "cancelled", status)
}

// TestMaterialise_LastStopRemovedLeavesRunningTrip - D3: a trip already on the road is never touched.
func TestMaterialise_LastStopRemovedLeavesRunningTrip(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	trip := driverTrip(t, helper, 1, true)
	today := dayOffset(t, "UTC", 0)

	clearStopsFrom(t, helper, trip.RouteID.String(), today)
	materialiseRoute(t, helper, trip.RouteID.String(), today, today)

	detail := tripRequest(t, helper, "GET", "/tracking/trips/"+trip.ID.String(), nil, http.StatusOK)
	assert.Equal(t, "in_progress", detail.Status)
}

// TestListTrips_RiderlessOnlyWhenAsked - D1: a trip nobody rides is left off the board unless
// include_empty is true; without the parameter every trip shows, as before.
func TestListTrips_RiderlessOnlyWhenAsked(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	routeID := CreateTestRoute(t, helper)
	PublishRouteVersion(t, helper, routeID, "2026-01-01", []map[string]interface{}{
		{"stop_place_id": CentralStationID, "sequence": 1, "planned_offset_min": 0},
		{"stop_place_id": SchoolAStopID, "sequence": 2, "planned_offset_min": 30},
	})
	CreateRouteSchedule(t, helper, routeID, map[string]interface{}{
		"days_of_week": EveryDayMask, "start_time": "07:00", "valid_from": "2026-01-01",
	})
	today := dayOffset(t, "UTC", 0)
	materialiseRoute(t, helper, routeID, today, today)

	assert.Equal(t, int64(0), listTrips(t, helper, routeID, today, "&include_empty=false").TotalCount)
	assert.Equal(t, int64(1), listTrips(t, helper, routeID, today, "&include_empty=true").TotalCount)
	assert.Equal(t, int64(1), listTrips(t, helper, routeID, today, "").TotalCount)
}

// TestDriverToday_HidesRiderlessUntilStarted - D2: once the last rider is unassigned the trip leaves
// the driver's day; started, it would stay.
func TestDriverToday_HidesRiderlessUntilStarted(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	trip := driverTrip(t, helper, 1, false)
	driver := SetupDriverTest(t)
	defer driver.Close()

	w := helper.DoRequest("GET", "/tracking/assignments?route_id="+trip.RouteID.String(), nil, map[string]string{})
	var list models.ListAssignmentsResponse
	decodeBody(t, w, &list)
	for _, a := range list.Assignments {
		w = helper.DoRequest("DELETE", "/tracking/assignments/"+a.ID.String(), nil, map[string]string{})
		assert.Equal(t, http.StatusNoContent, w.Code, w.Body.String())
	}
	today := dayOffset(t, "UTC", 0)
	materialiseRoute(t, helper, trip.RouteID.String(), today, today)

	w = driver.DoRequest("GET", "/mobile/driver/today", nil, map[string]string{})
	if !assert.Equal(t, http.StatusOK, w.Code, w.Body.String()) {
		return
	}
	var day struct {
		Trips []struct {
			ID string `json:"id"`
		} `json:"trips"`
	}
	decodeBody(t, w, &day)
	for _, row := range day.Trips {
		assert.NotEqual(t, trip.ID.String(), row.ID)
	}
}
