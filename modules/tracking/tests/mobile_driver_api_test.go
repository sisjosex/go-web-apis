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
	"josex/web/modules/tracking/models"
)

// ============================================================================
// DRIVER'S PHONE (TRACK-011)
// ============================================================================

// driverTrip is today's trip of a fresh route carrying riders riders, driven by the seeded driver
// account on a fresh vehicle; started, it began an hour ago so an op stamped within that hour keeps
// its own time (D2). It answers the trip's detail as the operator reads it. Its riders go at cleanup:
// left behind, 40 of them would push other tests' riders off the rider list's first page.
func driverTrip(t *testing.T, helper *testhelpers.ApiTestHelper, riders int, started bool) models.TripDetail {
	t.Helper()
	routeID, riderID := createTripRoute(t, helper, "07:00")
	riderIDs := []string{riderID}
	for i := 1; i < riders; i++ {
		riderIDs = append(riderIDs, CreateTestRider(t, helper))
		assignRider(t, helper, riderIDs[i], routeID)
	}
	t.Cleanup(func() { execSQL(t, helper, `DELETE FROM tracking.riders WHERE id = ANY($1::UUID[])`, riderIDs) })
	today := dayOffset(t, "UTC", 0)
	materialiseRoute(t, helper, routeID, today, today)
	tripID, _ := tripOn(t, helper, routeID, today, "07:00")

	vehicleID := CreateTestVehicleOf(t, helper, MainCompanyID)
	driverID := CreateTestDriver(t, helper)
	unlinkDriverAccount(t, helper)
	t.Cleanup(func() { unlinkDriverAccount(t, helper) })
	execSQL(t, helper, `UPDATE tracking.drivers SET user_id = (SELECT id FROM auth.users WHERE email = 'driver@test.local') WHERE id = $1`, driverID)
	execSQL(t, helper, `UPDATE tracking.trips SET vehicle_id = $2, driver_id = $3 WHERE id = $1`, tripID, vehicleID, driverID)
	if started {
		tripRequest(t, helper, "POST", "/tracking/trips/"+tripID+"/start", nil, http.StatusOK)
		execSQL(t, helper, `UPDATE tracking.trips SET started_at = now() - INTERVAL '1 hour' WHERE id = $1`, tripID)
	}
	return tripRequest(t, helper, "GET", "/tracking/trips/"+tripID, nil, http.StatusOK)
}

// op is one queued op, done at at.
func op(kind string, args models.DriverSyncArgs, at time.Time) models.DriverSyncOp {
	id := uuid.New()
	return models.DriverSyncOp{ClientOpID: &id, Op: kind, Args: args, ClientRecordedAt: &at}
}

func taskOp(kind string, taskID uuid.UUID, at time.Time) models.DriverSyncOp {
	return op(kind, models.DriverSyncArgs{TripStopTaskID: &taskID}, at)
}

// sync posts ops as the driver and answers the results, failing on anything but 200.
func sync(t *testing.T, driver *testhelpers.ApiTestHelper, ops ...models.DriverSyncOp) []models.DriverOpResult {
	t.Helper()
	w := driver.DoRequest("POST", "/mobile/driver/sync", models.DriverSyncDto{Ops: ops}, map[string]string{})
	if w.Code != http.StatusOK {
		t.Fatalf("sync: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var body models.DriverSyncResponse
	decodeBody(t, w, &body)
	return body.Results
}

func statusesOf(results []models.DriverOpResult) []string {
	statuses := make([]string, len(results))
	for i, r := range results {
		statuses[i] = r.Status
	}
	return statuses
}

// ---- today ----

// TestDriverToday_ETag - the driver's day with 40 riders: one trip, both stops with coordinates, 80
// tasks each naming its rider, under 50 KB; the ETag again is a 304, and a start changes it.
func TestDriverToday_ETag(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	trip := driverTrip(t, helper, 40, false)
	driver := SetupDriverTest(t)
	defer driver.Close()

	w := driver.DoRequest("GET", "/mobile/driver/today", nil, map[string]string{})
	if !assert.Equal(t, http.StatusOK, w.Code, w.Body.String()) {
		return
	}
	assert.Less(t, w.Body.Len(), 50*1024)
	var day struct {
		Date   string           `json:"date"`
		Driver models.DriverRef `json:"driver"`
		Trips  []struct {
			ID           uuid.UUID `json:"id"`
			LicensePlate *string   `json:"license_plate"`
			Timezone     string    `json:"timezone"`
			Stops        []struct {
				Lat   *float64 `json:"lat"`
				Tasks []struct {
					Rider *struct {
						Name string `json:"name"`
					} `json:"rider"`
				} `json:"tasks"`
			} `json:"stops"`
		} `json:"trips"`
		GeneratedAt time.Time `json:"generated_at"`
	}
	decodeBody(t, w, &day)
	assert.Equal(t, dayOffset(t, "UTC", 0), day.Date)
	assert.Equal(t, trip.DriverID.String(), day.Driver.ID.String())
	assert.False(t, day.GeneratedAt.IsZero())
	if assert.Len(t, day.Trips, 1) && assert.Len(t, day.Trips[0].Stops, 2) {
		assert.Equal(t, trip.ID, day.Trips[0].ID)
		assert.NotNil(t, day.Trips[0].LicensePlate)
		assert.Equal(t, "UTC", day.Trips[0].Timezone)
		tasks := 0
		for _, stop := range day.Trips[0].Stops {
			assert.NotNil(t, stop.Lat)
			for _, task := range stop.Tasks {
				if assert.NotNil(t, task.Rider) {
					assert.NotEmpty(t, task.Rider.Name)
				}
				tasks++
			}
		}
		assert.Equal(t, 80, tasks)
	}

	etag := w.Header().Get("ETag")
	assert.NotEmpty(t, etag)
	again := driver.DoRequest("GET", "/mobile/driver/today", nil, map[string]string{"If-None-Match": etag})
	assert.Equal(t, http.StatusNotModified, again.Code)
	assert.Zero(t, again.Body.Len())

	driver.DoRequest("POST", "/mobile/driver/trips/"+trip.ID.String()+"/start", nil, map[string]string{})
	changed := driver.DoRequest("GET", "/mobile/driver/today", nil, map[string]string{"If-None-Match": etag})
	assert.Equal(t, http.StatusOK, changed.Code)
	assert.NotEqual(t, etag, changed.Header().Get("ETag"))
}

// TestDriverToday_NotLinked - an account linked to no driver record → 403 driver.not-linked
func TestDriverToday_NotLinked(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	unlinkDriverAccount(t, helper)
	driver := SetupDriverTest(t)
	defer driver.Close()

	w := driver.DoRequest("GET", "/mobile/driver/today", nil, map[string]string{})

	assert.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "tracking.driver.not-linked")
}

// ---- start ----

// TestDriverStartTrip - the driver starts their planned trip → 200 in_progress; another driver's trip
// → 404 trip.not-found, left planned.
func TestDriverStartTrip(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	trip := driverTrip(t, helper, 1, false)
	_, _, otherID := todaysTrip(t, helper)
	driver := SetupDriverTest(t)
	defer driver.Close()

	w := driver.DoRequest("POST", "/mobile/driver/trips/"+trip.ID.String()+"/start", nil, map[string]string{})
	if assert.Equal(t, http.StatusOK, w.Code, w.Body.String()) {
		var detail models.TripDetail
		decodeBody(t, w, &detail)
		assert.Equal(t, "in_progress", detail.Status)
		assert.Len(t, detail.Stops, 2)
	}

	w = driver.DoRequest("POST", "/mobile/driver/trips/"+otherID+"/start", nil, map[string]string{})
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "tracking.trip.not-found")
	_, status := tripOn(t, helper, tripRouteOf(t, helper, otherID), dayOffset(t, "UTC", 0), "07:00")
	assert.Equal(t, "planned", status)
}

// tripRouteOf answers a trip's route.
func tripRouteOf(t *testing.T, helper *testhelpers.ApiTestHelper, tripID string) string {
	t.Helper()
	return tripRequest(t, helper, "GET", "/tracking/trips/"+tripID, nil, http.StatusOK).RouteID.String()
}

// ---- sync ----

// TestDriverSync_EveryOp - arrive, done, no_show, skip, complete, posted out of order: applied in the
// order done, each stamped with its client_recorded_at (D2); the skipped stop's dropoffs cancelled.
func TestDriverSync_EveryOp(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	trip := driverTrip(t, helper, 2, true)
	driver := SetupDriverTest(t)
	defer driver.Close()

	first, second := trip.Stops[0], trip.Stops[1]
	at := time.Now().UTC().Truncate(time.Second).Add(-50 * time.Minute)
	ops := []models.DriverSyncOp{
		op(models.DriverOpTripComplete, models.DriverSyncArgs{TripID: &trip.ID}, at.Add(15*time.Minute)),
		op(models.DriverOpStopArrive, models.DriverSyncArgs{TripStopID: &first.ID}, at),
		taskOp(models.DriverOpTaskDone, first.Tasks[0].ID, at.Add(5*time.Minute)),
		taskOp(models.DriverOpTaskNoShow, first.Tasks[1].ID, at.Add(6*time.Minute)),
		op(models.DriverOpStopSkip, models.DriverSyncArgs{TripStopID: &second.ID}, at.Add(10*time.Minute)),
	}

	results := sync(t, driver, ops...)

	if !assert.Len(t, results, 5) {
		return
	}
	for i, want := range []int{1, 2, 3, 4, 0} {
		assert.Equal(t, *ops[want].ClientOpID, results[i].ClientOpID)
		assert.Equal(t, models.DriverOpApplied, results[i].Status, "op %s: %v", ops[want].Op, results[i].Code)
		assert.Equal(t, trip.ID, *results[i].TripID)
	}

	var arrivedAt, doneAt, noShowAt, endedAt time.Time
	scanAt := func(query string, into *time.Time, args ...interface{}) {
		if err := helper.DB().QueryRow(t.Context(), query, args...).Scan(into); err != nil {
			t.Fatalf("read %q: %v", query, err)
		}
	}
	scanAt(`SELECT arrived_at FROM tracking.trip_stops WHERE id = $1`, &arrivedAt, first.ID)
	scanAt(`SELECT done_at FROM tracking.trip_stop_tasks WHERE id = $1`, &doneAt, first.Tasks[0].ID)
	scanAt(`SELECT done_at FROM tracking.trip_stop_tasks WHERE id = $1`, &noShowAt, first.Tasks[1].ID)
	scanAt(`SELECT ended_at FROM tracking.trips WHERE id = $1`, &endedAt, trip.ID)
	assert.True(t, arrivedAt.Equal(at), "arrived_at %s", arrivedAt)
	assert.True(t, doneAt.Equal(at.Add(5*time.Minute)), "done_at %s", doneAt)
	assert.True(t, noShowAt.Equal(at.Add(6*time.Minute)), "no_show done_at %s", noShowAt)
	assert.True(t, endedAt.Equal(at.Add(15*time.Minute)), "ended_at %s", endedAt)
	assert.ElementsMatch(t, []string{"dropoff:cancelled", "dropoff:cancelled", "pickup:done", "pickup:no_show"}, taskStatuses(t, helper, trip.ID.String()))
	assert.Equal(t, 5, countRows(t, helper, `SELECT count(*) FROM tracking.driver_ops WHERE trip_id = $1 AND status = 'applied'`, trip.ID))
}

// TestDriverSync_ReplayIsANoOp - 30 task ops twice: applied then replayed, 30 driver_ops rows, each
// task moved once — one trip_events row per task.
func TestDriverSync_ReplayIsANoOp(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	trip := driverTrip(t, helper, 15, true)
	driver := SetupDriverTest(t)
	defer driver.Close()

	at := time.Now().UTC().Add(-30 * time.Minute)
	ops := []models.DriverSyncOp{}
	for _, stop := range trip.Stops {
		for _, task := range stop.Tasks {
			ops = append(ops, taskOp(models.DriverOpTaskDone, task.ID, at.Add(time.Duration(len(ops))*time.Second)))
		}
	}
	if !assert.Len(t, ops, 30) {
		return
	}

	first := sync(t, driver, ops...)
	second := sync(t, driver, ops...)

	assert.Equal(t, 30, countOf(statusesOf(first), models.DriverOpApplied))
	assert.Equal(t, 30, countOf(statusesOf(second), models.DriverOpReplayed))
	assert.Equal(t, 30, countRows(t, helper, `SELECT count(*) FROM tracking.driver_ops WHERE trip_id = $1`, trip.ID))
	assert.Equal(t, 30, countRows(t, helper, `SELECT count(*) FROM tracking.trip_events WHERE trip_id = $1 AND type = 'done'`, trip.ID))
}

func countOf(values []string, want string) int {
	n := 0
	for _, v := range values {
		if v == want {
			n++
		}
	}
	return n
}

// TestDriverSync_Conflict - the web cancels the trip while the phone is offline: the queued task.done
// is rejected trip.task.trip-not-active, and sent again it is rejected again with the same code,
// nothing written in between.
func TestDriverSync_Conflict(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	trip := driverTrip(t, helper, 1, true)
	driver := SetupDriverTest(t)
	defer driver.Close()
	tripRequest(t, helper, "POST", "/tracking/trips/"+trip.ID.String()+"/cancel", nil, http.StatusOK)

	queued := taskOp(models.DriverOpTaskDone, trip.Stops[0].Tasks[0].ID, time.Now().UTC().Add(-time.Minute))
	first := sync(t, driver, queued)
	again := sync(t, driver, queued)

	for _, results := range [][]models.DriverOpResult{first, again} {
		if assert.Len(t, results, 1) {
			assert.Equal(t, models.DriverOpRejected, results[0].Status)
			if assert.NotNil(t, results[0].Code) {
				assert.Equal(t, "tracking.trip.task.trip-not-active", *results[0].Code)
			}
		}
	}
	assert.Equal(t, 1, countRows(t, helper, `SELECT count(*) FROM tracking.driver_ops WHERE client_op_id = $1 AND status = 'rejected'`, *queued.ClientOpID))
	assert.Equal(t, 0, countRows(t, helper, `SELECT count(*) FROM tracking.trip_events WHERE trip_id = $1 AND type = 'done'`, trip.ID))
}

// TestDriverSync_NotMyTrip - an op on another driver's task is rejected trip.not-found and moves
// nothing; a body without its target id is a 400.
func TestDriverSync_NotMyTrip(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	driverTrip(t, helper, 1, true)
	_, _, otherID := todaysTrip(t, helper)
	other := tripRequest(t, helper, "POST", "/tracking/trips/"+otherID+"/start", nil, http.StatusOK)
	driver := SetupDriverTest(t)
	defer driver.Close()

	results := sync(t, driver, taskOp(models.DriverOpTaskDone, other.Stops[0].Tasks[0].ID, time.Now().UTC()))

	if assert.Len(t, results, 1) {
		assert.Equal(t, models.DriverOpRejected, results[0].Status)
		assert.Equal(t, "tracking.trip.not-found", *results[0].Code)
		assert.Nil(t, results[0].TripID)
	}
	assert.Equal(t, []string{"dropoff:pending", "pickup:pending"}, taskStatuses(t, helper, otherID))

	w := driver.DoRequest("POST", "/mobile/driver/sync", map[string]interface{}{
		"ops": []map[string]interface{}{{"client_op_id": uuid.NewString(), "op": "task.done", "args": map[string]interface{}{}, "client_recorded_at": time.Now().UTC()}},
	}, map[string]string{})
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
}

// ---- access ----

// TestDriverAPI_OnlyDrivers - a member (and an admin) of the tenant → 403 on every driver endpoint.
func TestDriverAPI_OnlyDrivers(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	for _, path := range []string{"/mobile/driver/today"} {
		w := helper.DoRequest("GET", path, nil, map[string]string{})
		assert.Equal(t, http.StatusForbidden, w.Code, "admin %s: %s", path, w.Body.String())
	}

	setAdminRole := func(role string) {
		execSQL(t, helper, `UPDATE tenancy.tenant_users SET role = $1
			WHERE user_id = (SELECT id FROM auth.users WHERE email = 'admin@test.local')`, role)
	}
	setAdminRole("member")
	t.Cleanup(func() { setAdminRole("admin") })
	member := SetupTrackingTest(t)
	defer member.Close()

	for _, req := range []struct{ method, path string }{
		{"GET", "/mobile/driver/today"},
		{"POST", "/mobile/driver/trips/" + uuid.NewString() + "/start"},
		{"POST", "/mobile/driver/sync"},
	} {
		w := member.DoRequest(req.method, req.path, nil, map[string]string{})
		assert.Equal(t, http.StatusForbidden, w.Code, "member %s %s: %s", req.method, req.path, w.Body.String())
	}
}
