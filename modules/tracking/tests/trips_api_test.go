//go:build integration
// +build integration

package tracking_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"josex/web/config"
	coreJobs "josex/web/modules/core/jobs"
	"josex/web/modules/core/testhelpers"
	trackingJobs "josex/web/modules/tracking/jobs"
)

// ============================================================================
// TRIPS (TRACK-008)
// ============================================================================

// EveryDayMask is Monday to Sunday: a route that runs every day makes a test independent of the
// weekday it runs on.
const EveryDayMask = 127

// testTenant is the seeded tenant as the job walk sees it.
func testTenant() coreJobs.Tenant {
	return coreJobs.Tenant{ID: TestTenantID, DatabaseURL: config.ModularAppConfig.Core.DatabaseURL}
}

// dayOffset is the route-local date n days from today, for a route in zone.
func dayOffset(t *testing.T, zone string, n int) string {
	t.Helper()
	location, err := time.LoadLocation(zone)
	if err != nil {
		t.Fatalf("load zone %s: %v", zone, err)
	}
	return time.Now().In(location).AddDate(0, 0, n).Format(time.DateOnly)
}

// createTripRoute builds a route that runs every day at each of startTimes, calling at Central
// Station then School A, with one rider assigned from the first stop to the second. It answers the
// route and the rider.
func createTripRoute(t *testing.T, helper *testhelpers.ApiTestHelper, startTimes ...string) (string, string) {
	t.Helper()
	routeID := CreateTestRoute(t, helper)
	PublishRouteVersion(t, helper, routeID, "2026-01-01", []map[string]interface{}{
		{"stop_place_id": CentralStationID, "sequence": 1, "planned_offset_min": 0},
		{"stop_place_id": SchoolAStopID, "sequence": 2, "planned_offset_min": 30},
	})
	for _, startTime := range startTimes {
		CreateRouteSchedule(t, helper, routeID, map[string]interface{}{
			"days_of_week": EveryDayMask, "start_time": startTime, "valid_from": "2026-01-01",
		})
	}
	riderID := CreateTestRider(t, helper)
	assignRider(t, helper, riderID, routeID)
	return routeID, riderID
}

func assignRider(t *testing.T, helper *testhelpers.ApiTestHelper, riderID, routeID string) {
	t.Helper()
	w := helper.DoRequest("POST", "/tracking/assignments", map[string]interface{}{
		"rider_id": riderID, "route_id": routeID,
		"pickup_stop_id": CentralStationID, "dropoff_stop_id": SchoolAStopID,
	}, map[string]string{})
	if w.Code != http.StatusCreated {
		t.Fatalf("assign rider: expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

// materialiseRoute is what the route.changed handler does for one route over [from, to].
func materialiseRoute(t *testing.T, helper *testhelpers.ApiTestHelper, routeID, from, to string) {
	t.Helper()
	_, err := trackingJobs.ApplyRouteChanged(context.Background(), helper.DB(), testTenant(),
		trackingJobs.RouteChanged{RouteID: routeID, DateFrom: from, DateTo: to})
	if err != nil {
		t.Fatalf("materialise route %s: %v", routeID, err)
	}
}

// tripShape is a route's trips and tasks as a comparable fingerprint: counts, and every trip and
// task id with its status.
type tripShape struct {
	Trips, Tasks int
	Fingerprint  string
}

func routeTripShape(t *testing.T, helper *testhelpers.ApiTestHelper, routeID string) tripShape {
	t.Helper()
	var shape tripShape
	err := helper.DB().QueryRow(context.Background(), `
		SELECT
			(SELECT count(*) FROM tracking.trips WHERE route_id = $1),
			(SELECT count(*) FROM tracking.trip_stop_tasks k
			   JOIN tracking.trip_stops s ON s.id = k.trip_stop_id
			   JOIN tracking.trips tr ON tr.id = s.trip_id WHERE tr.route_id = $1),
			COALESCE((SELECT string_agg(tr.id || tr.status || k.id || k.status, ',' ORDER BY tr.id, k.id)
			   FROM tracking.trips tr
			   JOIN tracking.trip_stops s ON s.trip_id = tr.id
			   JOIN tracking.trip_stop_tasks k ON k.trip_stop_id = s.id
			  WHERE tr.route_id = $1), '')`, routeID).Scan(&shape.Trips, &shape.Tasks, &shape.Fingerprint)
	if err != nil {
		t.Fatalf("read trip shape: %v", err)
	}
	return shape
}

// tripOn answers the id and status of the route's trip at startTime (HH:MM) on date.
func tripOn(t *testing.T, helper *testhelpers.ApiTestHelper, routeID, date, startTime string) (string, string) {
	t.Helper()
	var id, status string
	err := helper.DB().QueryRow(context.Background(), `
		SELECT tr.id, tr.status FROM tracking.trips tr
		JOIN tracking.route_schedules s ON s.id = tr.route_schedule_id
		WHERE tr.route_id = $1 AND tr.service_date = $2::DATE AND TO_CHAR(s.start_time, 'HH24:MI') = $3`,
		routeID, date, startTime).Scan(&id, &status)
	if err != nil {
		t.Fatalf("find the %s trip on %s: %v", startTime, date, err)
	}
	return id, status
}

// taskStatuses answers the statuses of a trip's tasks, kind:status per task.
func taskStatuses(t *testing.T, helper *testhelpers.ApiTestHelper, tripID string) []string {
	t.Helper()
	rows, err := helper.DB().Query(context.Background(), `
		SELECT k.kind || ':' || k.status FROM tracking.trip_stop_tasks k
		JOIN tracking.trip_stops s ON s.id = k.trip_stop_id
		WHERE s.trip_id = $1 ORDER BY 1`, tripID)
	if err != nil {
		t.Fatalf("read tasks: %v", err)
	}
	defer rows.Close()
	statuses := []string{}
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatalf("scan task: %v", err)
		}
		statuses = append(statuses, s)
	}
	return statuses
}

func execSQL(t *testing.T, helper *testhelpers.ApiTestHelper, query string, args ...interface{}) {
	t.Helper()
	if _, err := helper.DB().Execute(context.Background(), query, args...); err != nil {
		t.Fatalf("exec %q: %v", query, err)
	}
}

// ---- step 1: the model replaces ride_events ----

// TestRideEventsEndpointIsGone - D3: boarding is a trip task transition now → 404
func TestRideEventsEndpointIsGone(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	w := helper.DoRequest("POST", "/tracking/events", map[string]interface{}{
		"rider_id": TestRiderJohnID, "route_id": MorningRouteID, "vehicle_id": TestBusID, "event_type": "check_in",
	}, map[string]string{})

	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
}

// TestUpdateRouteTimezone - D2: PATCH carries an IANA zone, the read shows it; an unknown one → 400
func TestUpdateRouteTimezone(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	routeID := CreateTestRoute(t, helper)

	w := helper.DoRequest("PATCH", "/tracking/routes/"+routeID, map[string]interface{}{"timezone": "America/Lima"}, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, "America/Lima", ParseResponse(t, w.Body.Bytes())["timezone"])

	w = helper.DoRequest("GET", "/tracking/routes/"+routeID, nil, map[string]string{})
	assert.Equal(t, "America/Lima", ParseResponse(t, w.Body.Bytes())["timezone"])

	w = helper.DoRequest("PATCH", "/tracking/routes/"+routeID, map[string]interface{}{"timezone": "Mars/Olympus_Mons"}, map[string]string{})
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "tracking.route.timezone")
}

// ---- step 2: the materialiser ----

// TestMaterialiseTwiceSameTrips - the daily pass run twice builds the same trips and tasks: 15 days
// of one departure, a pickup and a dropoff each, and nothing new the second time.
func TestMaterialiseTwiceSameTrips(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	routeID, _ := createTripRoute(t, helper, "07:00")

	first := trackingJobs.RunTripsMaterialisePass(context.Background(), helper.DB(), []coreJobs.Tenant{testTenant()})
	afterFirst := routeTripShape(t, helper, routeID)
	second := trackingJobs.RunTripsMaterialisePass(context.Background(), helper.DB(), []coreJobs.Tenant{testTenant()})
	afterSecond := routeTripShape(t, helper, routeID)

	if first[0].Err != nil || second[0].Err != nil {
		t.Fatalf("pass failed: %v / %v", first[0].Err, second[0].Err)
	}
	assert.Equal(t, 15, afterFirst.Trips)
	assert.Equal(t, 30, afterFirst.Tasks)
	assert.Equal(t, afterFirst, afterSecond, "a second run changes nothing")
}

// TestMaterialiseCancelException - a cancel exception cancels that day's trip and its tasks and
// leaves every other day alone; taking it back plans the day again.
func TestMaterialiseCancelException(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	routeID, _ := createTripRoute(t, helper, "07:00")
	today, day := dayOffset(t, "UTC", 0), dayOffset(t, "UTC", 2)
	materialiseRoute(t, helper, routeID, today, "")

	exceptionID := CreateRouteException(t, helper, routeID, CancelDay(day, "Strike"))
	materialiseRoute(t, helper, routeID, day, day)

	cancelledID, status := tripOn(t, helper, routeID, day, "07:00")
	assert.Equal(t, "cancelled", status)
	assert.Equal(t, []string{"dropoff:cancelled", "pickup:cancelled"}, taskStatuses(t, helper, cancelledID))
	otherID, otherStatus := tripOn(t, helper, routeID, dayOffset(t, "UTC", 1), "07:00")
	assert.Equal(t, "planned", otherStatus)
	assert.Equal(t, []string{"dropoff:pending", "pickup:pending"}, taskStatuses(t, helper, otherID))

	w := helper.DoRequest("DELETE", "/tracking/routes/"+routeID+"/exceptions/"+exceptionID, nil, map[string]string{})
	assert.Equal(t, http.StatusNoContent, w.Code, w.Body.String())
	materialiseRoute(t, helper, routeID, day, day)
	_, status = tripOn(t, helper, routeID, day, "07:00")
	assert.Equal(t, "planned", status)
}

// TestMaterialiseOverrideSurvives - an overridden trip keeps its header through a run, and still
// gains the tasks of a rider assigned afterwards; the assignment writes exactly one route.changed
// row, and that row is what the run consumes.
func TestMaterialiseOverrideSurvives(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	routeID, _ := createTripRoute(t, helper, "07:00")
	today, day := dayOffset(t, "UTC", 0), dayOffset(t, "UTC", 1)
	materialiseRoute(t, helper, routeID, today, "")
	tripID, _ := tripOn(t, helper, routeID, day, "07:00")
	execSQL(t, helper, `UPDATE tracking.trips SET is_overridden = TRUE, planned_start = planned_start + INTERVAL '1 hour' WHERE id = $1`, tripID)
	var overridden time.Time
	if err := helper.DB().QueryRow(context.Background(), `SELECT planned_start FROM tracking.trips WHERE id = $1`, tripID).Scan(&overridden); err != nil {
		t.Fatalf("read the override: %v", err)
	}

	before := routeChangedRows(t, helper)
	newRider := CreateTestRider(t, helper)
	assignRider(t, helper, newRider, routeID)
	assert.Equal(t, before+1, routeChangedRows(t, helper), "one assignment, one route.changed row")
	var payload trackingJobs.RouteChanged
	if err := helper.DB().QueryRow(context.Background(), `
		SELECT payload->>'route_id', payload->>'date_from', COALESCE(payload->>'date_to', '')
		FROM tracking.outbox WHERE topic = 'route.changed' ORDER BY id DESC LIMIT 1`).
		Scan(&payload.RouteID, &payload.DateFrom, &payload.DateTo); err != nil {
		t.Fatalf("read the route.changed row: %v", err)
	}
	assert.Equal(t, routeID, payload.RouteID)
	assert.Equal(t, today, payload.DateFrom)
	assert.Equal(t, "", payload.DateTo)
	if _, err := trackingJobs.ApplyRouteChanged(context.Background(), helper.DB(), testTenant(), payload); err != nil {
		t.Fatalf("apply route.changed: %v", err)
	}

	var kept time.Time
	var newRiderTasks int
	if err := helper.DB().QueryRow(context.Background(), `
		SELECT tr.planned_start,
		       (SELECT count(*) FROM tracking.trip_stop_tasks k JOIN tracking.trip_stops s ON s.id = k.trip_stop_id
		         WHERE s.trip_id = tr.id AND k.subject_id = $2)
		FROM tracking.trips tr WHERE tr.id = $1`, tripID, newRider).Scan(&kept, &newRiderTasks); err != nil {
		t.Fatalf("read the trip back: %v", err)
	}
	assert.True(t, overridden.Equal(kept), "the override's start survives: %v vs %v", overridden, kept)
	assert.Equal(t, 2, newRiderTasks, "the new rider's pickup and dropoff are on the overridden trip")
}

// TestMaterialiseLimaServiceDate - D2: a Lima route leaving at 23:30 local runs at 04:30 UTC the next
// day, and its trip still belongs to the Lima date it was planned for: one trip per service date.
func TestMaterialiseLimaServiceDate(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	routeID := CreateTestRoute(t, helper)
	w := helper.DoRequest("PATCH", "/tracking/routes/"+routeID, map[string]interface{}{"timezone": "America/Lima"}, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	CreateRouteSchedule(t, helper, routeID, map[string]interface{}{
		"days_of_week": EveryDayMask, "start_time": "23:30", "valid_from": "2026-01-01",
	})
	from, to := dayOffset(t, "America/Lima", 0), dayOffset(t, "America/Lima", 2)
	materialiseRoute(t, helper, routeID, from, to)

	var trips, dates, mismatched int
	if err := helper.DB().QueryRow(context.Background(), `
		SELECT count(*), count(DISTINCT service_date),
		       count(*) FILTER (WHERE CAST(planned_start AT TIME ZONE 'America/Lima' AS DATE) <> service_date
		                           OR CAST(planned_start AT TIME ZONE 'UTC' AS TIME) <> TIME '04:30'
		                           OR CAST(planned_start AT TIME ZONE 'UTC' AS DATE) <> service_date + 1
		                           OR timezone <> 'America/Lima')
		FROM tracking.trips WHERE route_id = $1`, routeID).Scan(&trips, &dates, &mismatched); err != nil {
		t.Fatalf("read the Lima trips: %v", err)
	}
	assert.Equal(t, 3, trips)
	assert.Equal(t, 3, dates, "one service_date per trip")
	assert.Equal(t, 0, mismatched, "service_date is the Lima day, planned_start 04:30 UTC the next")
}

// ---- step 3: the status wrappers ----

// TestRouteStatusCountsTheCurrentTripOnly - an outbound and an inbound run of one route on one day
// are two trips: a pickup on the inbound one does not move the outbound trip's boarded_count.
func TestRouteStatusCountsTheCurrentTripOnly(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	routeID, riderID := createTripRoute(t, helper, "07:00", "16:00")
	today := dayOffset(t, "UTC", 0)
	materialiseRoute(t, helper, routeID, today, today)
	outboundID, _ := tripOn(t, helper, routeID, today, "07:00")
	inboundID, _ := tripOn(t, helper, routeID, today, "16:00")
	execSQL(t, helper, `UPDATE tracking.trips SET status = 'in_progress', started_at = now() WHERE id = $1`, outboundID)
	markPickup := `UPDATE tracking.trip_stop_tasks k SET status = 'done', done_at = now()
		FROM tracking.trip_stops s WHERE s.id = k.trip_stop_id AND s.trip_id = $1 AND k.kind = 'pickup' AND k.subject_id = $2`

	execSQL(t, helper, markPickup, inboundID, riderID)
	status := routeStatus(t, helper, routeID)
	assert.Equal(t, float64(1), status["total_riders"])
	assert.Equal(t, float64(0), status["boarded_count"], "the inbound trip's pickup is not the current trip's")
	assert.Equal(t, float64(1), status["pending_count"])

	execSQL(t, helper, markPickup, outboundID, riderID)
	status = routeStatus(t, helper, routeID)
	assert.Equal(t, float64(1), status["boarded_count"])
	assert.Equal(t, float64(0), status["pending_count"])
}

// TestRiderStatusLastEventFromTask - the guardian view's last event is the rider's latest task
// transition, named in ride_events' words, at the trip stop where it happened.
func TestRiderStatusLastEventFromTask(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	routeID, riderID := createTripRoute(t, helper, "07:00")
	today := dayOffset(t, "UTC", 0)
	materialiseRoute(t, helper, routeID, today, today)
	tripID, _ := tripOn(t, helper, routeID, today, "07:00")
	execSQL(t, helper, `UPDATE tracking.trip_stop_tasks k SET status = 'done', done_at = now(), proof = '{"notes": "on time"}'
		FROM tracking.trip_stops s WHERE s.id = k.trip_stop_id AND s.trip_id = $1 AND k.kind = 'pickup'`, tripID)

	w := helper.DoRequest("GET", "/tracking/riders/"+riderID+"/status", nil, map[string]string{})

	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	status := ParseResponse(t, w.Body.Bytes())
	assert.Equal(t, "check_in", status["last_event_type"])
	assert.Equal(t, "Central Station", status["last_event_stop"])
	assert.Equal(t, "on time", status["last_event_notes"])
}

func routeStatus(t *testing.T, helper *testhelpers.ApiTestHelper, routeID string) map[string]interface{} {
	t.Helper()
	w := helper.DoRequest("GET", "/tracking/routes/"+routeID+"/status", nil, map[string]string{})
	if w.Code != http.StatusOK {
		t.Fatalf("route status: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	return ParseResponse(t, w.Body.Bytes())
}
