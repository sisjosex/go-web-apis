//go:build integration
// +build integration

package tracking_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"josex/web/modules/core/testhelpers"
)

// TRACK-038: routes anchored on their destination, outbound and return created as a pair.

// createPair creates a route to the seeded school (pinned first) ending at its stop, with three new stops, on vehicleID
// when set, and answers the outbound and the return.
func createPair(t *testing.T, helper *testhelpers.ApiTestHelper, vehicleID string) (map[string]interface{}, map[string]interface{}) {
	t.Helper()
	w := helper.DoRequest("PATCH", "/tracking/organizations/"+MainSchoolID, map[string]interface{}{
		"latitude": -17.3935, "longitude": -66.1570, "address": "Av. Ballivián 500",
	}, map[string]string{})
	expectStatus(t, w, http.StatusOK)

	body := map[string]interface{}{
		"company_id":                MainCompanyID,
		"organization_id":           MainSchoolID,
		"route_name":                "Ruta " + uuid.New().String()[:6],
		"arrival_time":              "07:45",
		"departure_time":            "13:00",
		"stops":                     pairStops(),
		"destination_stop_place_id": ParseResponse(t, w.Body.Bytes())["stop_place_id"],
	}
	if vehicleID != "" {
		body["vehicle_id"] = vehicleID
	}
	w = helper.DoRequest("POST", "/tracking/routes", body, map[string]string{})
	expectStatus(t, w, http.StatusCreated)
	created := ParseResponse(t, w.Body.Bytes())
	ret, _ := created["return_route"].(map[string]interface{})
	return created, ret
}

// pairStops are three new places, a fresh spot each run so no earlier place within 30 m is reused.
func pairStops() []map[string]interface{} {
	jitter := float64(uuid.New().ID()%1000) / 1e5
	stops := []map[string]interface{}{}
	for i, name := range []string{"Parada A", "Parada B", "Parada C"} {
		stops = append(stops, map[string]interface{}{
			"name": name, "latitude": -17.30 - jitter - float64(i)*0.005, "longitude": -66.10 - jitter,
		})
	}
	return stops
}

// TestRoutePair_CreatedMirrored - one create makes the outbound (stops, then the school) and its
// return (the school, then the stops reversed), linked both ways, each with one schedule.
func TestRoutePair_CreatedMirrored(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	out, ret := createPair(t, helper, "")
	if !assert.NotNil(t, ret, "the return comes back beside the outbound") {
		return
	}
	outID, retID := ExtractID(t, out), ExtractID(t, ret)
	assert.Equal(t, retID, out["paired_route_id"])
	assert.Equal(t, outID, ret["paired_route_id"])
	assert.Equal(t, "outbound", out["direction"])
	assert.Equal(t, "inbound", ret["direction"])
	assert.Equal(t, MainSchoolID, out["organization_id"])
	assert.Equal(t, "America/La_Paz", out["timezone"], "the deployment's zone when the form sends none")

	// The school is the outbound's last stop and the return's first (its own place, or one already
	// within 30 m of it); the pickups run in reverse.
	outStops, retStops := StopNames(ListRouteStops(t, helper, outID, "")), StopNames(ListRouteStops(t, helper, retID, ""))
	if assert.Len(t, outStops, 4) && assert.Len(t, retStops, 4) {
		assert.Equal(t, []string{"Parada A", "Parada B", "Parada C"}, outStops[:3])
		assert.Equal(t, []string{"Parada C", "Parada B", "Parada A"}, retStops[1:])
		assert.Equal(t, outStops[3], retStops[0])
	}

	outSchedules, retSchedules := ListRouteSchedules(t, helper, outID), ListRouteSchedules(t, helper, retID)
	if assert.Len(t, outSchedules, 1) && assert.Len(t, retSchedules, 1) {
		assert.Equal(t, "13:00", retSchedules[0]["start_time"])
		assert.Less(t, outSchedules[0]["start_time"], "07:45", "the outbound leaves before it must arrive")
		assert.EqualValues(t, WeekdaysMask, outSchedules[0]["days_of_week"])
	}

	w := helper.DoRequest("GET", "/tracking/routes?organization_id="+MainSchoolID+"&page_size=100", nil, map[string]string{})
	expectStatus(t, w, http.StatusOK)
	listed := ParseResponse(t, w.Body.Bytes())["routes"].([]interface{})
	ids := []string{}
	for _, row := range listed {
		ids = append(ids, row.(map[string]interface{})["id"].(string))
	}
	assert.Contains(t, ids, outID)
	assert.Contains(t, ids, retID)
}

// TestRoutePair_AssignmentLegs - "both" writes the rider on the return too, "outbound" only on the
// outbound; deleting the outbound row takes its twin.
func TestRoutePair_AssignmentLegs(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	out, ret := createPair(t, helper, "")
	outID, retID := ExtractID(t, out), ExtractID(t, ret)

	both := CreateHomedRider(t, helper)
	w := helper.DoRequest("POST", "/tracking/assignments", AssignmentBody(both, outID, WeekdaysMask), map[string]string{})
	expectStatus(t, w, http.StatusCreated)
	onlyOut := AssignmentBody(CreateHomedRider(t, helper), outID, WeekdaysMask)
	onlyOut["legs"] = "outbound"
	expectStatus(t, helper.DoRequest("POST", "/tracking/assignments", onlyOut, map[string]string{}), http.StatusCreated)

	retRows := listAssignments(t, helper, "route_id="+retID)["assignments"].([]interface{})
	if assert.Len(t, retRows, 1, "only the 'both' rider rides back") {
		assert.Equal(t, both, retRows[0].(map[string]interface{})["rider_id"])
	}
	assert.Len(t, listAssignments(t, helper, "route_id="+outID)["assignments"], 2)

	outRows := listAssignments(t, helper, fmt.Sprintf("route_id=%s&rider_id=%s", outID, both))["assignments"].([]interface{})
	expectStatus(t, helper.DoRequest("DELETE", "/tracking/assignments/"+outRows[0].(map[string]interface{})["id"].(string), nil, map[string]string{}),
		http.StatusNoContent)
	assert.Equal(t, 0, assignmentCount(t, helper, both), "the twin goes with it")
}

// TestRoutePair_SeatsShort - a route carrying more riders than a vehicle's seats is named when that
// vehicle is checked for it.
func TestRoutePair_SeatsShort(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	out, _ := createPair(t, helper, "")
	outID := ExtractID(t, out)
	for i := 0; i < 3; i++ {
		body := AssignmentBody(CreateHomedRider(t, helper), outID, WeekdaysMask)
		body["valid_from"] = "2026-01-01"
		expectStatus(t, helper.DoRequest("POST", "/tracking/assignments", body, map[string]string{}), http.StatusCreated)
	}

	dto := VehicleDtoForCompany(uuid.MustParse(MainCompanyID))
	dto.Capacity = 2
	w := helper.DoRequest("POST", "/tracking/vehicles", dto, map[string]string{})
	expectStatus(t, w, http.StatusCreated)
	small := ExtractID(t, ParseResponse(t, w.Body.Bytes()))

	w = helper.DoRequest("GET", fmt.Sprintf("/tracking/vehicles/%s/conflicts?route_id=%s", small, outID), nil, map[string]string{})
	expectStatus(t, w, http.StatusOK)
	short := ParseResponse(t, w.Body.Bytes())["seats_short"].([]interface{})
	if assert.Len(t, short, 1) {
		row := short[0].(map[string]interface{})
		assert.EqualValues(t, 3, row["assigned"])
		assert.EqualValues(t, 2, row["seats"])
	}
}
