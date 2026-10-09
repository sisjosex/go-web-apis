//go:build integration
// +build integration

package tracking_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

// ============================================================================
// THE OUTBOUND'S ARRIVAL ON ITS SCHEDULE (TRACK-055 D2)
// ============================================================================

// TestSchedules_ArrivalKept - the arrival a pair was created for is on its outbound schedule; a home
// added moves the departure and keeps the arrival; the return has none.
func TestSchedules_ArrivalKept(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	outID, retID := createEmptyPair(t, helper)
	out := ListRouteSchedules(t, helper, outID)[0]
	assert.Equal(t, "07:45", out["arrival_time"])
	assert.Equal(t, "07:40", out["start_time"])
	assert.Nil(t, ListRouteSchedules(t, helper, retID)[0]["arrival_time"], "the return leaves at a time of its own")

	CreateAssignment(t, helper, AssignmentBody(homedRider(t, helper, -17.3700, -66.1570), outID, WeekdaysMask))
	out = ListRouteSchedules(t, helper, outID)[0]
	assert.Equal(t, "07:45", out["arrival_time"])
	assert.Equal(t, "07:35", out["start_time"])
}

// TestSchedules_PatchArrival - a new arrival moves the departure by the same minutes; a departure set
// by hand drops the arrival; sending the arrival again puts it back on the estimate.
func TestSchedules_PatchArrival(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	outID, _ := createEmptyPair(t, helper)
	path := "/tracking/routes/" + outID + "/schedules/" + ListRouteSchedules(t, helper, outID)[0]["id"].(string)

	w := helper.DoRequest("PATCH", path, map[string]interface{}{"arrival_time": "08:15"}, map[string]string{})
	expectStatus(t, w, http.StatusOK)
	body := ParseResponse(t, w.Body.Bytes())
	assert.Equal(t, "08:15", body["arrival_time"])
	assert.Equal(t, "08:10", body["start_time"])

	w = helper.DoRequest("PATCH", path, map[string]interface{}{"start_time": "07:00"}, map[string]string{})
	expectStatus(t, w, http.StatusOK)
	assert.Nil(t, ParseResponse(t, w.Body.Bytes())["arrival_time"])

	w = helper.DoRequest("PATCH", path, map[string]interface{}{"arrival_time": "07:45"}, map[string]string{})
	expectStatus(t, w, http.StatusOK)
	assert.Equal(t, "07:40", ParseResponse(t, w.Body.Bytes())["start_time"])
}

// TestSchedules_ArrivalRefused - both times at once, neither on a create, or an arrival on a return.
func TestSchedules_ArrivalRefused(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	outID, retID := createEmptyPair(t, helper)
	body := map[string]interface{}{"days_of_week": WeekdaysMask, "valid_from": "2026-01-01"}

	w := helper.DoRequest("POST", "/tracking/routes/"+outID+"/schedules", body, map[string]string{})
	expectStatus(t, w, http.StatusBadRequest)
	assert.Equal(t, "tracking.schedule.time-choice", ParseResponse(t, w.Body.Bytes())["error"])

	body["start_time"], body["arrival_time"] = "06:00", "06:30"
	w = helper.DoRequest("POST", "/tracking/routes/"+outID+"/schedules", body, map[string]string{})
	expectStatus(t, w, http.StatusBadRequest)

	delete(body, "start_time")
	w = helper.DoRequest("POST", "/tracking/routes/"+retID+"/schedules", body, map[string]string{})
	expectStatus(t, w, http.StatusBadRequest)
	assert.Equal(t, "tracking.route.arrival-outbound-only", ParseResponse(t, w.Body.Bytes())["error"])

	w = helper.DoRequest("POST", "/tracking/routes/"+outID+"/schedules", body, map[string]string{})
	expectStatus(t, w, http.StatusCreated)
	created := ParseResponse(t, w.Body.Bytes())
	assert.Equal(t, "06:30", created["arrival_time"])
	assert.Equal(t, "06:25", created["start_time"])
}
