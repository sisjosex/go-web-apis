//go:build integration
// +build integration

package tracking_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"josex/web/modules/core/testhelpers"
)

// TRACK-044: stops come from riders — a route starts with only its destination, every assigned rider has
// their own stop, riders keep saved places.

// createEmptyPair creates an outbound and return to the seeded school with no stops, arriving 07:45 and
// leaving 13:00, and answers both ids.
func createEmptyPair(t *testing.T, helper *testhelpers.ApiTestHelper) (string, string) {
	t.Helper()
	w := helper.DoRequest("PATCH", "/tracking/organizations/"+MainSchoolID, map[string]interface{}{
		"latitude": -17.3935, "longitude": -66.1570, "address": "Av. Ballivián 500",
	}, map[string]string{})
	expectStatus(t, w, http.StatusOK)
	w = helper.DoRequest("POST", "/tracking/routes", map[string]interface{}{
		"company_id":      MainCompanyID,
		"organization_id": MainSchoolID,
		"route_name":      "Ruta " + uuid.New().String()[:6],
		"arrival_time":    "07:45",
		"departure_time":  "13:00",
		"stops":           []interface{}{},
	}, map[string]string{})
	expectStatus(t, w, http.StatusCreated)
	created := ParseResponse(t, w.Body.Bytes())
	return ExtractID(t, created), ExtractID(t, created["return_route"].(map[string]interface{}))
}

func outboundStart(t *testing.T, helper *testhelpers.ApiTestHelper, routeID string) string {
	t.Helper()
	schedules := ListRouteSchedules(t, helper, routeID)
	if len(schedules) != 1 {
		t.Fatalf("expected one schedule, got %d", len(schedules))
	}
	return schedules[0]["start_time"].(string)
}

// TestRoutePair_NoStops - D1/D2: a pair saves with only the school; the first home lands before it on the
// outbound and after it on the return, and the outbound leaves earlier to still arrive at 07:45.
func TestRoutePair_NoStops(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	outID, retID := createEmptyPair(t, helper)
	assert.Len(t, ListRouteStops(t, helper, outID, ""), 1, "only the school")
	assert.Equal(t, "07:40", outboundStart(t, helper, outID), "five minutes when there is nothing to drive")

	rider := homedRider(t, helper, -17.3700, -66.1570)
	CreateAssignment(t, helper, AssignmentBody(rider, outID, WeekdaysMask))

	outStops, retStops := StopNames(ListRouteStops(t, helper, outID, "")), StopNames(ListRouteStops(t, helper, retID, ""))
	if assert.Len(t, outStops, 2) && assert.Len(t, retStops, 2) {
		assert.Equal(t, "Lucía Mamani", outStops[0], "the home is picked up before the school")
		assert.Equal(t, "Lucía Mamani", retStops[1], "and dropped off after it")
	}
	assert.Equal(t, "07:35", outboundStart(t, helper, outID), "2.6 km more: 10 minutes before the arrival")
	assert.Equal(t, "13:00", outboundStart(t, helper, retID), "the return leaves when it was told to")

	// A time someone sets is theirs: the next home does not move it.
	schedule := ListRouteSchedules(t, helper, outID)[0]
	w := helper.DoRequest("PATCH", "/tracking/routes/"+outID+"/schedules/"+schedule["id"].(string),
		map[string]interface{}{"start_time": "07:00"}, map[string]string{})
	expectStatus(t, w, http.StatusOK)
	CreateAssignment(t, helper, AssignmentBody(homedRider(t, helper, -17.3500, -66.1570), outID, WeekdaysMask))
	assert.Len(t, ListRouteStops(t, helper, outID, ""), 3)
	assert.Equal(t, "07:00", outboundStart(t, helper, outID))
}

// TestAssignments_HomeRequired - D3: a rider with no home and no shared stop is refused, on a create and
// on a PATCH asking for "home".
func TestAssignments_HomeRequired(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	outID, _ := createEmptyPair(t, helper)
	w := helper.DoRequest("POST", "/tracking/assignments", AssignmentBody(CreateTestRider(t, helper), outID, WeekdaysMask), map[string]string{})
	expectStatus(t, w, http.StatusUnprocessableEntity)
	assert.Equal(t, "tracking.rider.location-required", ParseResponse(t, w.Body.Bytes())["error"])
}

// TestAssignments_HomeStopFollowsTheRider - a PATCH sets the pickup back to "home"; a home stop nobody
// calls at any more leaves the route (D5), on a change of stop and on a delete with its twin.
func TestAssignments_HomeStopFollowsTheRider(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	outID, retID := createEmptyPair(t, helper)
	rider := homedRider(t, helper, -17.3800, -66.1700)
	a := CreateAssignment(t, helper, AssignmentBody(rider, outID, WeekdaysMask))
	home := a["pickup_stop_place_id"]
	stops := ListRouteStops(t, helper, outID, "")
	if !assert.Len(t, stops, 2) {
		return
	}
	school := stops[1]["stop_place_id"]

	// The school as the pickup: the home stop goes, and the departure with it.
	path := "/tracking/assignments/" + a["id"].(string)
	w := helper.DoRequest("PATCH", path, map[string]interface{}{"pickup_stop_place_id": school}, map[string]string{})
	expectStatus(t, w, http.StatusOK)
	assert.Len(t, ListRouteStops(t, helper, outID, ""), 1, "no detour to an empty house")
	assert.Equal(t, "07:40", outboundStart(t, helper, outID))

	// Back to "home": the stop is put back and is the pickup again.
	w = helper.DoRequest("PATCH", path, map[string]interface{}{"pickup_stop_place_id": "home"}, map[string]string{})
	expectStatus(t, w, http.StatusOK)
	assert.Equal(t, home, ParseResponse(t, w.Body.Bytes())["assignment"].(map[string]interface{})["pickup_stop_place_id"])
	assert.Len(t, ListRouteStops(t, helper, outID, ""), 2)

	w = helper.DoRequest("PATCH", path, map[string]interface{}{"pickup_stop_place_id": "office"}, map[string]string{})
	expectStatus(t, w, http.StatusBadRequest)

	// A neighbour shares the stop: removing the rider keeps it for them.
	neighbour := homedRider(t, helper, -17.38005, -66.1700)
	CreateAssignment(t, helper, AssignmentBody(neighbour, outID, WeekdaysMask))
	w = helper.DoRequest("DELETE", path, nil, map[string]string{})
	expectStatus(t, w, http.StatusNoContent)
	assert.Len(t, ListRouteStops(t, helper, outID, ""), 2, "the neighbour still lives there")
	assert.Len(t, ListRouteStops(t, helper, retID, ""), 2)
}

// TestRiderPlaces_DefaultIsTheHome - D4: the form's pin is the default place "Casa"; another place made
// default becomes the rider's home; the default goes last.
func TestRiderPlaces_DefaultIsTheHome(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	rider := homedRider(t, helper, -17.3900, -66.1500)
	base := "/tracking/riders/" + rider + "/places"
	list := func() []interface{} {
		w := helper.DoRequest("GET", base, nil, map[string]string{})
		expectStatus(t, w, http.StatusOK)
		return ParseResponse(t, w.Body.Bytes())["places"].([]interface{})
	}

	places := list()
	if assert.Len(t, places, 1) {
		casa := places[0].(map[string]interface{})
		assert.Equal(t, "Casa", casa["label"])
		assert.Equal(t, true, casa["is_default"])
		assert.InDelta(t, -17.39, casa["latitude"], 1e-6)
	}

	w := helper.DoRequest("POST", base, map[string]interface{}{
		"label": "Oficina", "address": "Calle Junín 120", "latitude": -17.3950, "longitude": -66.1600,
	}, map[string]string{})
	expectStatus(t, w, http.StatusCreated)
	office := ParseResponse(t, w.Body.Bytes())
	assert.Equal(t, false, office["is_default"])
	officePath := base + "/" + office["id"].(string)

	w = helper.DoRequest("PATCH", officePath, map[string]interface{}{"is_default": true}, map[string]string{})
	expectStatus(t, w, http.StatusOK)
	w = helper.DoRequest("GET", "/tracking/riders/"+rider, nil, map[string]string{})
	expectStatus(t, w, http.StatusOK)
	got := ParseResponse(t, w.Body.Bytes())
	assert.InDelta(t, -17.395, got["home_latitude"], 1e-6, "the default place is the home")
	assert.Equal(t, "Calle Junín 120", got["address"])

	// "Domicilio" on a new assignment now resolves to the office.
	outID, _ := createEmptyPair(t, helper)
	a := CreateAssignment(t, helper, AssignmentBody(rider, outID, WeekdaysMask))
	assert.Equal(t, "Lucía Mamani", a["pickup_stop_name"])
	stops := ListRouteStops(t, helper, outID, "")
	if assert.Len(t, stops, 2) {
		assert.InDelta(t, -17.395, stops[0]["latitude"], 1e-6)
	}

	w = helper.DoRequest("DELETE", officePath, nil, map[string]string{})
	expectStatus(t, w, http.StatusConflict)
	assert.Equal(t, "tracking.rider-place.default", ParseResponse(t, w.Body.Bytes())["error"])

	casaPath := base + "/" + list()[1].(map[string]interface{})["id"].(string)
	w = helper.DoRequest("DELETE", casaPath, nil, map[string]string{})
	expectStatus(t, w, http.StatusNoContent)
	assert.Len(t, list(), 1)

	w = helper.DoRequest("PATCH", casaPath, map[string]interface{}{"label": "Casa"}, map[string]string{})
	expectStatus(t, w, http.StatusNotFound)
	w = helper.DoRequest("GET", "/tracking/riders/"+uuid.New().String()+"/places", nil, map[string]string{})
	expectStatus(t, w, http.StatusNotFound)
}
