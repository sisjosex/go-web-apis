//go:build integration
// +build integration

package tracking_test

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"josex/web/modules/core/testhelpers"
)

// TRACK-070: a pair listed as one row, and the return of an outbound created without one.

// listRouteRows answers the rows of GET /tracking/routes for the query.
func listRouteRows(t *testing.T, helper *testhelpers.ApiTestHelper, query string) []map[string]interface{} {
	t.Helper()
	w := helper.DoRequest("GET", "/tracking/routes?page_size=100&"+query, nil, map[string]string{})
	expectStatus(t, w, http.StatusOK)
	rows := []map[string]interface{}{}
	for _, row := range ParseResponse(t, w.Body.Bytes())["routes"].([]interface{}) {
		rows = append(rows, row.(map[string]interface{}))
	}
	return rows
}

// TestRouteList_GroupPairs - grouped, a pair is its outbound with the return inside and its times; a
// return whose outbound the filter drops lists alone; vehicle_id=none keeps the routes without one.
func TestRouteList_GroupPairs(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	w := helper.DoRequest("POST", "/tracking/vehicles", VehicleDtoForCompany(uuid.MustParse(MainCompanyID)), map[string]string{})
	expectStatus(t, w, http.StatusCreated)
	vehicleID := ExtractID(t, ParseResponse(t, w.Body.Bytes()))
	out, ret := createPair(t, helper, vehicleID)
	outID, retID := ExtractID(t, out), ExtractID(t, ret)
	search := "search=" + url.QueryEscape(out["route_name"].(string))

	assert.Len(t, listRouteRows(t, helper, search), 2, "ungrouped, two rows")
	rows := listRouteRows(t, helper, search+"&group_pairs=true")
	if assert.Len(t, rows, 1, "grouped, one row per service") {
		assert.Equal(t, outID, rows[0]["id"])
		assert.NotEmpty(t, rows[0]["arrival_time"], "the outbound reads its arrival")
		inner, _ := rows[0]["return_route"].(map[string]interface{})
		if assert.NotNil(t, inner) {
			assert.Equal(t, retID, inner["id"])
			assert.Equal(t, "13:00", inner["start_time"])
			assert.Equal(t, rows[0]["license_plate"], inner["license_plate"])
		}
	}

	SetRouteVehicle(t, helper, retID, "")
	rows = listRouteRows(t, helper, search+"&group_pairs=true&vehicle_id="+vehicleID)
	if assert.Len(t, rows, 1) {
		assert.Equal(t, outID, rows[0]["id"])
		assert.Nil(t, rows[0]["return_route"], "the return the filter drops is not inside")
	}
	rows = listRouteRows(t, helper, search+"&group_pairs=true&vehicle_id=none")
	if assert.Len(t, rows, 1, "the return whose outbound is filtered out lists alone") {
		assert.Equal(t, retID, rows[0]["id"])
	}
}

// TestRouteReturn_Created - "Crear retorno" on a lone outbound with three riders, one who contracted
// only the outbound: the return runs the stops reversed, carries the other two and is linked both
// ways; a second call is refused.
func TestRouteReturn_Created(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	body := pairBody(t, helper, "")
	body["with_return"] = false
	w := helper.DoRequest("POST", "/tracking/routes", body, map[string]string{})
	expectStatus(t, w, http.StatusCreated)
	out := ParseResponse(t, w.Body.Bytes())
	outID := ExtractID(t, out)
	assert.Nil(t, out["paired_route_id"])

	riders := []string{CreateHomedRider(t, helper), CreateHomedRider(t, helper),
		createRider(t, helper, map[string]interface{}{"home_latitude": -17.35, "home_longitude": -66.2, "service_legs": "outbound"})}
	for _, rider := range riders {
		expectStatus(t, helper.DoRequest("POST", "/tracking/assignments", AssignmentBody(rider, outID, WeekdaysMask), map[string]string{}),
			http.StatusCreated)
	}

	w = helper.DoRequest("POST", "/tracking/routes/"+outID+"/return", map[string]interface{}{"departure_time": "13:15"}, map[string]string{})
	expectStatus(t, w, http.StatusCreated)
	ret := ParseResponse(t, w.Body.Bytes())
	retID := ExtractID(t, ret)
	assert.Equal(t, "inbound", ret["direction"])
	assert.Equal(t, outID, ret["paired_route_id"])
	assert.Equal(t, retID, getRoute(t, helper, outID)["paired_route_id"])

	outStops, retStops := StopNames(ListRouteStops(t, helper, outID, "")), StopNames(ListRouteStops(t, helper, retID, ""))
	if assert.Equal(t, len(outStops), len(retStops)) {
		for i := range outStops {
			assert.Equal(t, outStops[i], retStops[len(retStops)-1-i], "the stops run in reverse")
		}
	}
	schedules := ListRouteSchedules(t, helper, retID)
	if assert.Len(t, schedules, 1) {
		assert.Equal(t, "13:15", schedules[0]["start_time"])
		assert.EqualValues(t, WeekdaysMask, schedules[0]["days_of_week"])
	}

	retRows := listAssignments(t, helper, "route_id="+retID)["assignments"].([]interface{})
	if assert.Len(t, retRows, 2, "the outbound-only rider stays off the return") {
		for _, row := range retRows {
			assert.NotEqual(t, riders[2], row.(map[string]interface{})["rider_id"])
		}
	}

	w = helper.DoRequest("POST", "/tracking/routes/"+outID+"/return", map[string]interface{}{"departure_time": "13:15"}, map[string]string{})
	expectStatus(t, w, http.StatusConflict)
	w = helper.DoRequest("POST", "/tracking/routes/"+retID+"/return", map[string]interface{}{"departure_time": "13:15"}, map[string]string{})
	expectStatus(t, w, http.StatusConflict)
}

// getRoute answers GET /tracking/routes/:id.
func getRoute(t *testing.T, helper *testhelpers.ApiTestHelper, routeID string) map[string]interface{} {
	t.Helper()
	w := helper.DoRequest("GET", "/tracking/routes/"+routeID, nil, map[string]string{})
	expectStatus(t, w, http.StatusOK)
	return ParseResponse(t, w.Body.Bytes())
}
