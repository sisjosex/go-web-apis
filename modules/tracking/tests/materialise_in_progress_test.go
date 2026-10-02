//go:build integration
// +build integration

package tracking_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"josex/web/modules/core/testhelpers"
	"josex/web/modules/tracking/models"
)

// ============================================================================
// ASSIGNMENTS REACH THE RUNNING TRIP (TRACK-031)
// ============================================================================

// lateRider is a rider assigned to the trip's route from the first stop to the second after the trip
// started, with the materialiser run as the route.changed handler would.
func lateRider(t *testing.T, helper *testhelpers.ApiTestHelper, trip models.TripDetail) string {
	t.Helper()
	riderID := CreateTestRider(t, helper)
	assignRider(t, helper, riderID, trip.RouteID.String())
	today := dayOffset(t, "UTC", 0)
	materialiseRoute(t, helper, trip.RouteID.String(), today, today)
	return riderID
}

// TestMaterialise_InProgressAddsRiderAtCurrentStop - D1: the bus arrived at stop 1 and has not left
// for stop 2; a rider assigned there now shows on the driver's day with a pending pickup at stop 1,
// and the pickup the driver already marked done stays done.
func TestMaterialise_InProgressAddsRiderAtCurrentStop(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	trip := driverTrip(t, helper, 1, true)
	driver := SetupDriverTest(t)
	defer driver.Close()
	first := trip.Stops[0]
	at := time.Now().UTC().Add(-10 * time.Minute)
	sync(t, driver,
		op(models.DriverOpStopArrive, models.DriverSyncArgs{TripStopID: &first.ID}, at),
		taskOp(models.DriverOpTaskDone, first.Tasks[0].ID, at.Add(time.Minute)))

	riderID := lateRider(t, helper, trip)

	w := driver.DoRequest("GET", "/mobile/driver/today", nil, map[string]string{})
	if !assert.Equal(t, http.StatusOK, w.Code, w.Body.String()) {
		return
	}
	var day struct {
		Trips []struct {
			Stops []struct {
				Sequence int32 `json:"sequence"`
				Tasks    []struct {
					Kind   string `json:"kind"`
					Status string `json:"status"`
					Rider  *struct {
						ID string `json:"id"`
					} `json:"rider"`
				} `json:"tasks"`
			} `json:"stops"`
		} `json:"trips"`
	}
	decodeBody(t, w, &day)
	late := []string{}
	if assert.Len(t, day.Trips, 1) {
		for _, stop := range day.Trips[0].Stops {
			for _, task := range stop.Tasks {
				if task.Rider != nil && task.Rider.ID == riderID {
					late = append(late, fmt.Sprintf("%d:%s:%s", stop.Sequence, task.Kind, task.Status))
				}
			}
		}
	}
	assert.Equal(t, []string{"1:pickup:pending", "2:dropoff:pending"}, late)
	assert.Equal(t, []string{"1:pickup:done", "2:dropoff:pending"},
		RiderTasks(t, helper, trip.ID.String(), first.Tasks[0].SubjectID.String()))
}

// TestMaterialise_InProgressSkipsPassedStop - D1: once the bus arrived at stop 2, a rider assigned at
// stop 1 does not enter this trip.
func TestMaterialise_InProgressSkipsPassedStop(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	trip := driverTrip(t, helper, 1, true)
	driver := SetupDriverTest(t)
	defer driver.Close()
	at := time.Now().UTC().Add(-10 * time.Minute)
	sync(t, driver,
		op(models.DriverOpStopArrive, models.DriverSyncArgs{TripStopID: &trip.Stops[0].ID}, at),
		op(models.DriverOpStopArrive, models.DriverSyncArgs{TripStopID: &trip.Stops[1].ID}, at.Add(5*time.Minute)))

	riderID := lateRider(t, helper, trip)

	assert.Empty(t, RiderTasks(t, helper, trip.ID.String(), riderID))
}

// TestMaterialise_InProgressRemovedRiders - unassigned mid-trip: the rider already picked up keeps
// their dropoff, the rider still waiting loses both tasks.
func TestMaterialise_InProgressRemovedRiders(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	trip := driverTrip(t, helper, 2, true)
	driver := SetupDriverTest(t)
	defer driver.Close()
	first := trip.Stops[0]
	onBoard, waiting := first.Tasks[0].SubjectID.String(), first.Tasks[1].SubjectID.String()
	sync(t, driver, taskOp(models.DriverOpTaskDone, first.Tasks[0].ID, time.Now().UTC().Add(-time.Minute)))

	w := helper.DoRequest("GET", "/tracking/assignments?route_id="+trip.RouteID.String(), nil, map[string]string{})
	var list models.ListAssignmentsResponse
	decodeBody(t, w, &list)
	if !assert.Len(t, list.Assignments, 2) {
		return
	}
	for _, a := range list.Assignments {
		w = helper.DoRequest("DELETE", "/tracking/assignments/"+a.ID.String(), nil, map[string]string{})
		assert.Equal(t, http.StatusNoContent, w.Code, w.Body.String())
	}
	today := dayOffset(t, "UTC", 0)
	materialiseRoute(t, helper, trip.RouteID.String(), today, today)

	assert.Equal(t, []string{"1:pickup:done", "2:dropoff:pending"}, RiderTasks(t, helper, trip.ID.String(), onBoard))
	assert.Empty(t, RiderTasks(t, helper, trip.ID.String(), waiting))
}
