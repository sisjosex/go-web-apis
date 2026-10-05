//go:build integration
// +build integration

package tracking_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"josex/web/modules/core/testhelpers"
)

// TRACK-037: guardians come with the rider in one request, and the home is the default stop.

func riderWithGuardians(guardians []map[string]interface{}) map[string]interface{} {
	dto := ValidRiderDto()
	return map[string]interface{}{
		"organization_id": dto.OrganizationID,
		"rider_type":      dto.RiderType,
		"first_name":      "Lucía",
		"last_name":       "Mamani",
		"guardians":       guardians,
	}
}

// TestCreateRider_WithGuardians - an existing account, a new email and a contact without one are all
// linked by the create; a refused item names its index and leaves no rider and no membership.
func TestCreateRider_WithGuardians(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	// An account that already exists: a guardian of another rider.
	existingEmail := newAccessEmail()
	w := helper.DoRequest("POST", fmt.Sprintf("/tracking/riders/%s/guardians", createAccessRider(t, helper)),
		accessPerson(existingEmail), map[string]string{})
	expectStatus(t, w, http.StatusCreated)
	existingID := ParseResponse(t, w.Body.Bytes())["user_id"].(string)

	newEmail := newAccessEmail()
	w = helper.DoRequest("POST", "/tracking/riders", riderWithGuardians([]map[string]interface{}{
		{"user_id": existingID},
		accessPerson(newEmail),
		{"first_name": "Rosa", "phone": "+59171111111"},
	}), map[string]string{})
	expectStatus(t, w, http.StatusCreated)
	created := ParseResponse(t, w.Body.Bytes())
	riderID := ExtractID(t, created)
	assert.Len(t, created["guardians"], 2, "the two accounts come back with their notices")

	w = helper.DoRequest("GET", fmt.Sprintf("/tracking/riders/%s/guardians", riderID), nil, map[string]string{})
	expectStatus(t, w, http.StatusOK)
	listed := ParseListResponse(t, w.Body.Bytes())
	if assert.Len(t, listed, 3) {
		assert.NotNil(t, listed[0]["user_id"])
		assert.NotNil(t, listed[1]["user_id"])
		assert.Nil(t, listed[2]["user_id"], "the contact without an account lists last")
		assert.Equal(t, "Rosa", listed[2]["first_name"])
	}

	// A contact with no phone is refused at index 1, after index 0 was granted: nothing stays.
	refusedEmail := newAccessEmail()
	before := ListRiders(t, helper, "").TotalCount
	w = helper.DoRequest("POST", "/tracking/riders", riderWithGuardians([]map[string]interface{}{
		accessPerson(refusedEmail),
		{"first_name": "Sin teléfono"},
	}), map[string]string{})
	expectStatus(t, w, http.StatusBadRequest)
	body := ParseResponse(t, w.Body.Bytes())
	assert.Equal(t, "tracking.rider.guardian-invalid", body["error"])
	assert.EqualValues(t, 1, body["detail"].(map[string]interface{})["index"])
	assert.Equal(t, before, ListRiders(t, helper, "").TotalCount, "no rider was created")
	if refusedID := UserIDByEmail(t, helper, refusedEmail); refusedID != "" {
		_, active := TenantMembership(t, helper, refusedID)
		assert.False(t, active, "the membership granted before the refusal is taken back")
	}
}

func homedRider(t *testing.T, helper *testhelpers.ApiTestHelper, lat, lng float64) string {
	t.Helper()
	body := riderWithGuardians(nil)
	delete(body, "guardians")
	body["home_latitude"], body["home_longitude"] = lat, lng
	w := helper.DoRequest("POST", "/tracking/riders", body, map[string]string{})
	expectStatus(t, w, http.StatusCreated)
	return ExtractID(t, ParseResponse(t, w.Body.Bytes()))
}

// TestAssignments_HomeIsDefaultStop - an assignment with no pickup puts the home on the route where it
// lengthens it least, splitting the version that started earlier; a neighbour 10 m away reuses it.
func TestAssignments_HomeIsDefaultStop(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	routeID := CreateTestRoute(t, helper)
	PublishRouteVersion(t, helper, routeID, "2026-01-01", []map[string]interface{}{
		{"sequence": 1, "name": "Plaza Norte", "latitude": -17.3700, "longitude": -66.1500},
		{"sequence": 2, "name": "Plaza Sur", "latitude": -17.4100, "longitude": -66.1600},
	})

	first := homedRider(t, helper, -17.3900, -66.1550)
	a := CreateAssignment(t, helper, AssignmentBody(first, routeID, WeekdaysMask))
	assert.NotNil(t, a["pickup_stop_place_id"], "the home is the pickup")
	assert.Nil(t, a["dropoff_stop_place_id"])

	stops := ListRouteStops(t, helper, routeID, "")
	if assert.Len(t, stops, 3) {
		assert.Equal(t, "Lucía Mamani", stops[1]["stop_name"], "between the two plazas, where the detour is shortest")
		assert.Equal(t, a["pickup_stop_place_id"], stops[1]["stop_place_id"])
	}
	assert.Len(t, ListRouteVersions(t, helper, routeID), 2, "the version from January keeps its days; today on is a new one")
	assert.Len(t, ListRouteStops(t, helper, routeID, "?date=2026-02-02"), 2, "a past day is unchanged")

	// ~10 m north: the same stop, no new version.
	second := homedRider(t, helper, -17.38991, -66.1550)
	b := CreateAssignment(t, helper, AssignmentBody(second, routeID, WeekdaysMask))
	assert.Equal(t, a["pickup_stop_place_id"], b["pickup_stop_place_id"])
	assert.Len(t, ListRouteStops(t, helper, routeID, ""), 3)
	assert.Len(t, ListRouteVersions(t, helper, routeID), 2)

	// A rider without a home pin cannot be placed (TRACK-044 D3).
	w := helper.DoRequest("POST", "/tracking/assignments", AssignmentBody(CreateTestRider(t, helper), routeID, WeekdaysMask), map[string]string{})
	expectStatus(t, w, http.StatusUnprocessableEntity)
	assert.Contains(t, w.Body.String(), "tracking.rider.location-required")
}
