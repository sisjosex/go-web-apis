//go:build integration
// +build integration

package tracking_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"josex/web/modules/core/testhelpers"
)

// TRACK-070: a vehicle that changes company, a rider removed from one leg, a client's gates.

// createTestCompany creates a carrier of the test tenant and answers its id.
func createTestCompany(t *testing.T, helper *testhelpers.ApiTestHelper) string {
	t.Helper()
	w := helper.DoRequest("POST", "/tracking/companies", ValidCompanyDto(), map[string]string{})
	expectStatus(t, w, http.StatusCreated)
	return ExtractID(t, ParseResponse(t, w.Body.Bytes()))
}

// createTestVehicle creates a vehicle of the seeded carrier and answers its id.
func createTestVehicle(t *testing.T, helper *testhelpers.ApiTestHelper) string {
	t.Helper()
	w := helper.DoRequest("POST", "/tracking/vehicles", VehicleDtoForCompany(uuid.MustParse(MainCompanyID)), map[string]string{})
	expectStatus(t, w, http.StatusCreated)
	return ExtractID(t, ParseResponse(t, w.Body.Bytes()))
}

func moveVehicle(t *testing.T, helper *testhelpers.ApiTestHelper, vehicleID, companyID string, moveRoutes bool) map[string]interface{} {
	t.Helper()
	w := helper.DoRequest("POST", "/tracking/vehicles/"+vehicleID+"/company",
		map[string]interface{}{"company_id": companyID, "move_routes": moveRoutes}, map[string]string{})
	expectStatus(t, w, http.StatusOK)
	return ParseResponse(t, w.Body.Bytes())
}

// TestVehicleMove_WithRoutes - the vehicle takes its routes and their pairs to the new company; the
// pair running another bus loses it and the old carrier's usual driver is dropped.
func TestVehicleMove_WithRoutes(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	target := createTestCompany(t, helper)
	vehicleID, otherBus := createTestVehicle(t, helper), createTestVehicle(t, helper)
	out, ret := createPair(t, helper, vehicleID)
	outID, retID := ExtractID(t, out), ExtractID(t, ret)
	SetRouteDefaultDriver(t, helper, outID, CreateTestDriver(t, helper))
	SetRouteVehicle(t, helper, retID, otherBus)

	moved := moveVehicle(t, helper, vehicleID, target, true)
	assert.EqualValues(t, 2, moved["routes_moved"])
	assert.EqualValues(t, 1, moved["routes_unassigned"])
	assert.Equal(t, target, moved["vehicle"].(map[string]interface{})["company_id"])

	outRoute, retRoute := getRoute(t, helper, outID), getRoute(t, helper, retID)
	assert.Equal(t, target, outRoute["company_id"])
	assert.Equal(t, vehicleID, outRoute["vehicle_id"])
	assert.Nil(t, outRoute["default_driver_id"], "the old carrier's driver is dropped")
	assert.Equal(t, target, retRoute["company_id"], "the pair goes with it")
	assert.Nil(t, retRoute["vehicle_id"], "the other bus stays behind")

	w := helper.DoRequest("POST", "/tracking/vehicles/"+vehicleID+"/company",
		map[string]interface{}{"company_id": uuid.New().String(), "move_routes": true}, map[string]string{})
	expectStatus(t, w, http.StatusNotFound)
}

// TestVehicleMove_WithoutRoutes - the routes stay and lose the vehicle: a planned trip is rebuilt
// without it, a completed one keeps it, and vehicle_id=none lists them.
func TestVehicleMove_WithoutRoutes(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	target := createTestCompany(t, helper)
	vehicleID := createTestVehicle(t, helper)
	out, ret := createPair(t, helper, vehicleID)
	outID, retID := ExtractID(t, out), ExtractID(t, ret)

	from := time.Now().Format("2006-01-02")
	to := time.Now().AddDate(0, 0, 7).Format("2006-01-02")
	materialiseRoute(t, helper, outID, from, to)
	trips := RouteTripIDs(t, helper, outID)
	if !assert.GreaterOrEqual(t, len(trips), 2, "a week of weekdays") {
		return
	}
	CompleteTrip(t, helper, trips[0])

	moved := moveVehicle(t, helper, vehicleID, target, false)
	assert.EqualValues(t, 0, moved["routes_moved"])
	assert.EqualValues(t, 2, moved["routes_unassigned"])
	for _, id := range []string{outID, retID} {
		route := getRoute(t, helper, id)
		assert.Equal(t, MainCompanyID, route["company_id"], "the route stays with its carrier")
		assert.Nil(t, route["vehicle_id"])
	}

	materialiseRoute(t, helper, outID, from, to)
	assert.Equal(t, vehicleID, TripVehicleID(t, helper, trips[0]), "a completed trip keeps its vehicle")
	assert.Equal(t, "", TripVehicleID(t, helper, trips[1]), "a planned trip runs without one")

	rows := listRouteRows(t, helper, "vehicle_id=none&organization_id="+MainSchoolID)
	ids := []string{}
	for _, row := range rows {
		ids = append(ids, row["id"].(string))
	}
	assert.Contains(t, ids, outID)
	assert.Contains(t, ids, retID)
}

// TestAssignmentDelete_OneLeg - twin=false removes one leg and leaves the other; the default still
// removes both.
func TestAssignmentDelete_OneLeg(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	out, ret := createPair(t, helper, "")
	outID, retID := ExtractID(t, out), ExtractID(t, ret)
	for i := 0; i < 2; i++ {
		expectStatus(t, helper.DoRequest("POST", "/tracking/assignments",
			AssignmentBody(CreateHomedRider(t, helper), outID, WeekdaysMask), map[string]string{}), http.StatusCreated)
	}
	retRows := listAssignments(t, helper, "route_id="+retID)["assignments"].([]interface{})
	if !assert.Len(t, retRows, 2) {
		return
	}

	first := retRows[0].(map[string]interface{})
	expectStatus(t, helper.DoRequest("DELETE", "/tracking/assignments/"+first["id"].(string)+"?twin=false", nil, map[string]string{}),
		http.StatusNoContent)
	rider := first["rider_id"].(string)
	assert.Equal(t, 1, assignmentCount(t, helper, rider), "the outbound twin stays")

	second := retRows[1].(map[string]interface{})
	expectStatus(t, helper.DoRequest("DELETE", "/tracking/assignments/"+second["id"].(string), nil, map[string]string{}),
		http.StatusNoContent)
	assert.Equal(t, 0, assignmentCount(t, helper, second["rider_id"].(string)), "by default the twin goes too")
}

// TestStopPlaces_ClientGates - organization_id lists one client's places only.
func TestStopPlaces_ClientGates(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	w := helper.DoRequest("POST", "/tracking/organizations", OrganizationBody(ValidOrganizationDto()), map[string]string{})
	expectStatus(t, w, http.StatusCreated)
	clientID := ExtractID(t, ParseResponse(t, w.Body.Bytes()))
	gates := map[string]bool{}
	for i := 0; i < 2; i++ {
		body := ValidStopPlaceBody()
		body["organization_id"] = clientID
		w = helper.DoRequest("POST", "/tracking/stop-places", body, map[string]string{})
		expectStatus(t, w, http.StatusCreated)
		gates[ExtractID(t, ParseResponse(t, w.Body.Bytes()))] = true
	}
	other := CreateTestStopPlace(t, helper)

	// The client's own stop (its pin, TRACK-045 D3) is a gate too.
	listed := ListStopPlaces(t, helper, "?organization_id="+clientID+"&page_size=100")
	ids := map[string]bool{}
	for _, place := range listed.StopPlaces {
		ids[place.ID.String()] = true
		if assert.NotNil(t, place.OrganizationID) {
			assert.Equal(t, clientID, place.OrganizationID.String())
		}
	}
	for gate := range gates {
		assert.True(t, ids[gate], "both gates are listed")
	}
	assert.False(t, ids[other], "a place of no client is not")
}
