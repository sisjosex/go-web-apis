//go:build integration
// +build integration

package tracking_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"

	"josex/web/modules/core/jobs"
	"josex/web/modules/core/testhelpers"
	trackingJobs "josex/web/modules/tracking/jobs"
	"josex/web/modules/tracking/models"
)

// ============================================================================
// TRIP ENDPOINTS (TRACK-020)
// ============================================================================

// todaysTrip builds a route that runs every day at 07:00 with one rider, materialises today and
// answers the route, the rider and today's trip.
func todaysTrip(t *testing.T, helper *testhelpers.ApiTestHelper) (string, string, string) {
	t.Helper()
	routeID, riderID := createTripRoute(t, helper, "07:00")
	today := dayOffset(t, "UTC", 0)
	materialiseRoute(t, helper, routeID, today, today)
	tripID, _ := tripOn(t, helper, routeID, today, "07:00")
	return routeID, riderID, tripID
}

// tripRequest sends a trip request and decodes the detail it answers, failing on any other status.
func tripRequest(t *testing.T, helper *testhelpers.ApiTestHelper, method, path string, body interface{}, want int) models.TripDetail {
	t.Helper()
	w := helper.DoRequest(method, path, body, map[string]string{})
	if w.Code != want {
		t.Fatalf("%s %s: expected %d, got %d: %s", method, path, want, w.Code, w.Body.String())
	}
	var detail models.TripDetail
	if want == http.StatusOK {
		decodeBody(t, w, &detail)
	}
	return detail
}

// taskOf answers the id of the rider's task of kind on the detail.
func taskOf(t *testing.T, detail models.TripDetail, riderID, kind string) string {
	t.Helper()
	for _, stop := range detail.Stops {
		for _, task := range stop.Tasks {
			if task.SubjectID.String() == riderID && task.Kind == kind {
				return task.ID.String()
			}
		}
	}
	t.Fatalf("no %s task for rider %s on trip %s", kind, riderID, detail.ID)
	return ""
}

func decodeBody(t *testing.T, w *httptest.ResponseRecorder, into interface{}) {
	t.Helper()
	if err := json.Unmarshal(w.Body.Bytes(), into); err != nil {
		t.Fatalf("decode %s: %v", w.Body.String(), err)
	}
}

func countRows(t *testing.T, helper *testhelpers.ApiTestHelper, query string, args ...interface{}) int {
	t.Helper()
	var n int
	if err := helper.DB().QueryRow(context.Background(), query, args...).Scan(&n); err != nil {
		t.Fatalf("count %q: %v", query, err)
	}
	return n
}

func tripChangedRows(t *testing.T, helper *testhelpers.ApiTestHelper, tripID, eventType string) int {
	t.Helper()
	return countRows(t, helper, `SELECT count(*) FROM tracking.outbox
		WHERE topic = 'trip.changed' AND payload->>'trip_id' = $1 AND payload->>'type' = $2`, tripID, eventType)
}

// ---- step 1: reads ----

// TestListTrips_DayRows - today's list of one route: its one trip, the rider's two tasks, the route
// and stop counts; another organization's filter drops it, a malformed status is a 400.
func TestListTrips_DayRows(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	routeID, _, tripID := todaysTrip(t, helper)

	w := helper.DoRequest("GET", "/tracking/trips?route_id="+routeID, nil, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var list models.ListTripsResponse
	decodeBody(t, w, &list)
	if assert.Len(t, list.Trips, 1) {
		trip := list.Trips[0]
		assert.Equal(t, tripID, trip.ID.String())
		assert.Equal(t, int64(1), list.TotalCount)
		assert.Equal(t, 2, int(trip.TasksTotal), "one rider: a pickup and a dropoff")
		assert.Equal(t, 0, int(trip.TasksDone))
		assert.Equal(t, 2, int(trip.StopsCount))
		assert.Equal(t, "outbound", trip.Direction)
		assert.NotEmpty(t, trip.RouteName)
		assert.Equal(t, "planned", trip.Status)
	}

	w = helper.DoRequest("GET", "/tracking/trips?route_id="+routeID+"&date="+dayOffset(t, "UTC", 0)+"&organization_id="+MainSchoolID, nil, map[string]string{})
	decodeBody(t, w, &list)
	assert.Len(t, list.Trips, 1, "the rider is MainSchool's")

	w = helper.DoRequest("GET", "/tracking/trips?route_id="+routeID+"&organization_id="+EmptyEmployerID, nil, map[string]string{})
	decodeBody(t, w, &list)
	assert.Len(t, list.Trips, 0, "no rider of that organization is on it")

	w = helper.DoRequest("GET", "/tracking/trips?status=late", nil, map[string]string{})
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
}

// TestGetTrip_StopsAndTasks - the detail's first stop carries the rider's pickup with their name;
// an unknown id is a 404.
func TestGetTrip_StopsAndTasks(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	_, riderID, tripID := todaysTrip(t, helper)

	detail := tripRequest(t, helper, "GET", "/tracking/trips/"+tripID, nil, http.StatusOK)

	if assert.Len(t, detail.Stops, 2) {
		first := detail.Stops[0]
		assert.Equal(t, "Central Station", first.StopName)
		assert.Equal(t, int32(1), first.Sequence)
		if assert.Len(t, first.Tasks, 1) {
			assert.Equal(t, "pickup", first.Tasks[0].Kind)
			assert.Equal(t, riderID, first.Tasks[0].SubjectID.String())
			if assert.NotNil(t, first.Tasks[0].RiderName) {
				assert.NotEmpty(t, *first.Tasks[0].RiderName)
			}
		}
		assert.Equal(t, "dropoff", detail.Stops[1].Tasks[0].Kind)
	}

	w := helper.DoRequest("GET", "/tracking/trips/"+uuid.NewString(), nil, map[string]string{})
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "tracking.trip.not-found")
	w = helper.DoRequest("GET", "/tracking/trips/"+uuid.NewString()+"/status", nil, map[string]string{})
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
}

// TestTripStatus_MatchesRouteStatus - over the route's current trip, the trip status counts what the
// route status counts; the next stop is the first pending one and the delay is never negative.
func TestTripStatus_MatchesRouteStatus(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	routeID, riderID, tripID := todaysTrip(t, helper)
	detail := tripRequest(t, helper, "POST", "/tracking/trips/"+tripID+"/start", nil, http.StatusOK)
	tripRequest(t, helper, "POST", "/tracking/trip-stop-tasks/"+taskOf(t, detail, riderID, "pickup")+"/done",
		map[string]interface{}{"client_op_id": uuid.NewString()}, http.StatusOK)
	tripRequest(t, helper, "POST", "/tracking/trip-stops/"+detail.Stops[0].ID.String()+"/arrive", nil, http.StatusOK)

	w := helper.DoRequest("GET", "/tracking/trips/"+tripID+"/status", nil, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var status models.TripStatus
	decodeBody(t, w, &status)
	route := routeStatus(t, helper, routeID)

	assert.Equal(t, "in_progress", status.Status)
	assert.Equal(t, route["total_riders"], float64(status.TotalRiders))
	assert.Equal(t, route["boarded_count"], float64(status.BoardedCount))
	assert.Equal(t, route["arrived_count"], float64(status.ArrivedCount))
	assert.Equal(t, route["no_show_count"], float64(status.NoShowCount))
	assert.Equal(t, route["pending_count"], float64(status.PendingCount))
	assert.Equal(t, int32(1), status.BoardedCount)
	if assert.NotNil(t, status.NextStop) {
		assert.Equal(t, detail.Stops[1].ID, status.NextStop.ID, "the first stop is arrived: the next is the second")
	}
	if assert.NotNil(t, status.DelaySeconds) {
		assert.GreaterOrEqual(t, *status.DelaySeconds, int32(0))
	}
}

// TestTrips_OrganizationDenied - the board is the operator's (D3).
func TestTrips_OrganizationDenied(t *testing.T) {
	helper := SetupOrganizationTest(t)
	defer helper.Close()

	w := helper.DoRequest("GET", "/tracking/trips", nil, map[string]string{})
	assert.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
}

// ---- step 2: transitions and override ----

// TestTripTaskDone_IdempotentOnClientOp - done twice with one client_op_id is one transition: one
// trip_events row, one trip.changed row, boarded_count 1. The same id on another task is a 409.
func TestTripTaskDone_IdempotentOnClientOp(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	_, riderID, tripID := todaysTrip(t, helper)
	detail := tripRequest(t, helper, "POST", "/tracking/trips/"+tripID+"/start", nil, http.StatusOK)
	pickup := taskOf(t, detail, riderID, "pickup")
	op := map[string]interface{}{"client_op_id": uuid.NewString(), "proof": map[string]interface{}{"notes": "on time"}}

	first := tripRequest(t, helper, "POST", "/tracking/trip-stop-tasks/"+pickup+"/done", op, http.StatusOK)
	again := tripRequest(t, helper, "POST", "/tracking/trip-stop-tasks/"+pickup+"/done", op, http.StatusOK)

	assert.Equal(t, "done", first.Stops[0].Tasks[0].Status)
	assert.Equal(t, int32(1), first.TasksDone)
	assert.Equal(t, first.TasksDone, again.TasksDone)
	assert.Equal(t, 1, countRows(t, helper, `SELECT count(*) FROM tracking.trip_events WHERE trip_id = $1 AND type = 'done'`, tripID))
	assert.Equal(t, 1, tripChangedRows(t, helper, tripID, "done"))
	assert.Equal(t, float64(1), routeStatus(t, helper, detail.RouteID.String())["boarded_count"])

	w := helper.DoRequest("POST", "/tracking/trip-stop-tasks/"+taskOf(t, detail, riderID, "dropoff")+"/done", op, map[string]string{})
	assert.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "tracking.trip.task.op-conflict")

	w = helper.DoRequest("POST", "/tracking/trip-stop-tasks/"+pickup+"/done", map[string]interface{}{}, map[string]string{})
	assert.Equal(t, http.StatusBadRequest, w.Code, "client_op_id is required: %s", w.Body.String())
}

// TestTripTransitions_WrongState - a task on a planned trip, a second start, an unknown action.
func TestTripTransitions_WrongState(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	_, riderID, tripID := todaysTrip(t, helper)
	detail := tripRequest(t, helper, "GET", "/tracking/trips/"+tripID, nil, http.StatusOK)

	w := helper.DoRequest("POST", "/tracking/trip-stop-tasks/"+taskOf(t, detail, riderID, "pickup")+"/done",
		map[string]interface{}{"client_op_id": uuid.NewString()}, map[string]string{})
	assert.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "tracking.trip.task.trip-not-active")

	w = helper.DoRequest("POST", "/tracking/trip-stops/"+detail.Stops[0].ID.String()+"/arrive", nil, map[string]string{})
	assert.Equal(t, http.StatusConflict, w.Code, "a stop of a planned trip: %s", w.Body.String())

	tripRequest(t, helper, "POST", "/tracking/trips/"+tripID+"/start", nil, http.StatusOK)
	w = helper.DoRequest("POST", "/tracking/trips/"+tripID+"/start", nil, map[string]string{})
	assert.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "tracking.trip.invalid-transition")

	w = helper.DoRequest("POST", "/tracking/trips/"+tripID+"/teleport", nil, map[string]string{})
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())

	completed := tripRequest(t, helper, "POST", "/tracking/trips/"+tripID+"/complete", nil, http.StatusOK)
	assert.Equal(t, "completed", completed.Status)
	assert.NotNil(t, completed.EndedAt)
}

// TestTripCancel_KeptByMaterialiser - D3: a manual cancel cancels the pending tasks and marks the trip
// overridden, and a materialiser run leaves it cancelled.
func TestTripCancel_KeptByMaterialiser(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	routeID, _, tripID := todaysTrip(t, helper)

	cancelled := tripRequest(t, helper, "POST", "/tracking/trips/"+tripID+"/cancel", nil, http.StatusOK)
	assert.Equal(t, "cancelled", cancelled.Status)
	assert.True(t, cancelled.IsOverridden)
	assert.Equal(t, int32(0), cancelled.TasksTotal, "every task is cancelled")
	assert.Equal(t, 1, tripChangedRows(t, helper, tripID, "cancel"))

	today := dayOffset(t, "UTC", 0)
	materialiseRoute(t, helper, routeID, today, today)
	_, status := tripOn(t, helper, routeID, today, "07:00")
	assert.Equal(t, "cancelled", status)
	assert.Equal(t, []string{"dropoff:cancelled", "pickup:cancelled"}, taskStatuses(t, helper, tripID))
}

// TestTripStopSkip_SettlesItsTasks - D3: skipping the first stop turns its pickup into a no_show;
// a task at a skipped stop can no longer be marked.
func TestTripStopSkip_SettlesItsTasks(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	_, riderID, tripID := todaysTrip(t, helper)
	detail := tripRequest(t, helper, "POST", "/tracking/trips/"+tripID+"/start", nil, http.StatusOK)

	skipped := tripRequest(t, helper, "POST", "/tracking/trip-stops/"+detail.Stops[0].ID.String()+"/skip", nil, http.StatusOK)

	assert.Equal(t, "skipped", skipped.Stops[0].Status)
	assert.Equal(t, "no_show", skipped.Stops[0].Tasks[0].Status)
	assert.Equal(t, "pending", skipped.Stops[1].Tasks[0].Status, "the dropoff is at the other stop")
	w := helper.DoRequest("POST", "/tracking/trip-stop-tasks/"+taskOf(t, detail, riderID, "pickup")+"/done",
		map[string]interface{}{"client_op_id": uuid.NewString()}, map[string]string{})
	assert.Equal(t, http.StatusConflict, w.Code, w.Body.String())
}

// TestUpdateTrip_Override - a new start marks the trip overridden and moves its stops with it; a
// vehicle of another carrier is refused on its field; an ended trip cannot be overridden.
func TestUpdateTrip_Override(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	_, _, tripID := todaysTrip(t, helper)
	before := tripRequest(t, helper, "GET", "/tracking/trips/"+tripID, nil, http.StatusOK)
	later := before.PlannedStart.Add(time.Hour)

	after := tripRequest(t, helper, "PATCH", "/tracking/trips/"+tripID,
		map[string]interface{}{"planned_start": later.Format(time.RFC3339)}, http.StatusOK)

	assert.True(t, after.IsOverridden)
	assert.True(t, later.Equal(after.PlannedStart))
	assert.True(t, before.Stops[1].PlannedAt.Add(time.Hour).Equal(*after.Stops[1].PlannedAt), "the stops move with the start")
	assert.Equal(t, 1, tripChangedRows(t, helper, tripID, "override"))

	foreignVehicle := CreateTestVehicleOf(t, helper, SecondaryCompanyID)
	w := helper.DoRequest("PATCH", "/tracking/trips/"+tripID, map[string]interface{}{"vehicle_id": foreignVehicle}, map[string]string{})
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "tracking.route.company-mismatch")
	assert.Contains(t, w.Body.String(), "vehicle_id")

	tripRequest(t, helper, "POST", "/tracking/trips/"+tripID+"/cancel", nil, http.StatusOK)
	w = helper.DoRequest("PATCH", "/tracking/trips/"+tripID, map[string]interface{}{"planned_start": later.Format(time.RFC3339)}, map[string]string{})
	assert.Equal(t, http.StatusConflict, w.Code, w.Body.String())
}

// CreateTestVehicleOf creates a vehicle of companyID and returns its id.
func CreateTestVehicleOf(t *testing.T, helper *testhelpers.ApiTestHelper, companyID string) string {
	t.Helper()
	w := helper.DoRequest("POST", "/tracking/vehicles", VehicleDtoForCompany(uuid.MustParse(companyID)), map[string]string{})
	if w.Code != http.StatusCreated {
		t.Fatalf("create vehicle: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	return ExtractID(t, ParseResponse(t, w.Body.Bytes()))
}

// ---- step 3: trip.changed (D1) ----

// TestTripChanged_PublishedAndAcknowledged - a start leaves one trip.changed row; the relay publishes
// it as outbox:trip.changed and stamps published_at, and the handler acknowledges the task.
func TestTripChanged_PublishedAndAcknowledged(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	ctx := context.Background()
	_, _, tripID := todaysTrip(t, helper)
	tenantID := uuid.NewString()
	relay, inspector := startTestRelay(t, helper, tenantID)
	defer relay.Shutdown()
	defer inspector.Close()

	tripRequest(t, helper, "POST", "/tracking/trips/"+tripID+"/start", nil, http.StatusOK)

	assert.Equal(t, 1, tripChangedRows(t, helper, tripID, "start"))
	var id int64
	if err := helper.DB().QueryRow(ctx, `SELECT id FROM tracking.outbox
		WHERE topic = 'trip.changed' AND payload->>'trip_id' = $1`, tripID).Scan(&id); err != nil {
		t.Fatalf("read the trip.changed row: %v", err)
	}
	var info *asynq.TaskInfo
	var err error
	waitFor(t, 2*time.Second, "the trip.changed task", func() bool {
		info, err = inspector.GetTaskInfo(jobs.QueueDefault, tenantID+":"+strconv.FormatInt(id, 10))
		return err == nil
	})
	assert.Equal(t, trackingJobs.TaskTripChanged, info.Type)
	waitFor(t, 2*time.Second, "published_at on the trip.changed row", func() bool {
		var published bool
		err := helper.DB().QueryRow(ctx, `SELECT published_at IS NOT NULL FROM tracking.outbox WHERE id = $1`, id).Scan(&published)
		return err == nil && published
	})

	var logged bytes.Buffer
	log.SetOutput(&logged)
	defer log.SetOutput(os.Stderr)
	err = trackingJobs.TripChangedHandler()(ctx, asynq.NewTask(info.Type, info.Payload))

	assert.NoError(t, err)
	assert.Contains(t, logged.String(), "trip.changed "+tripID+" start acknowledged")
}
