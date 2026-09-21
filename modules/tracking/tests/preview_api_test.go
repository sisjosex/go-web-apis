//go:build integration
// +build integration

package tracking_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"josex/web/modules/core/testhelpers"
)

// ============================================================================
// ROUTE EXCEPTIONS (TRACK-018 step 3)
// ============================================================================

func TestCreateRouteExceptionSuccess(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	routeID := CreateTestRoute(t, helper)

	w := helper.DoRequest("POST", "/tracking/routes/"+routeID+"/exceptions",
		CancelDay("2026-10-03", "Public holiday"), map[string]string{})

	assert.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	created := ParseResponse(t, w.Body.Bytes())
	assert.Equal(t, "cancel", created["kind"])
	assert.Equal(t, "2026-10-03", created["date_from"])
	assert.Equal(t, "2026-10-03", created["date_to"])
	assert.Equal(t, "Public holiday", created["reason"])
}

// TestCreateRouteExceptionEveryKind - each kind's payload is its contract, and the SP is where that
// contract lives; this is the list of shapes it accepts.
func TestCreateRouteExceptionEveryKind(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	routeID := CreateTestRoute(t, helper)
	driverID := CreateTestDriver(t, helper)
	kinds := []struct {
		kind    string
		payload map[string]interface{}
	}{
		{"cancel", map[string]interface{}{}},
		{"change_time", map[string]interface{}{"start_time": "08:15"}},
		{"change_vehicle", map[string]interface{}{"vehicle_id": TestBusID}},
		{"change_driver", map[string]interface{}{"driver_id": driverID}},
		{"skip_stop", map[string]interface{}{"stop_place_id": SchoolAStopID}},
		{"add_stop", map[string]interface{}{"stop_place_id": SchoolBStopID, "after_sequence": 2}},
		{"detour", map[string]interface{}{"stops": []map[string]interface{}{{"stop_place_id": CentralStationID}}}},
	}

	for i, attempt := range kinds {
		date := fmt.Sprintf("2026-11-%02d", i+1)
		w := helper.DoRequest("POST", "/tracking/routes/"+routeID+"/exceptions", map[string]interface{}{
			"date_from": date, "date_to": date, "kind": attempt.kind, "payload": attempt.payload,
		}, map[string]string{})
		assert.Equal(t, http.StatusCreated, w.Code, "kind %s: %s", attempt.kind, w.Body.String())
	}
	assert.Len(t, ListRouteExceptions(t, helper, routeID, ""), len(kinds))
}

// TestCreateRouteExceptionWrongPayloadKeys - an extra or missing key is a caller saying something
// the SP will not read → 400
func TestCreateRouteExceptionWrongPayloadKeys(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	routeID := CreateTestRoute(t, helper)
	bad := []map[string]interface{}{
		{"kind": "change_time", "payload": map[string]interface{}{"time": "08:00"}},
		{"kind": "change_time", "payload": map[string]interface{}{"start_time": "08:00", "reason": "x"}},
		{"kind": "cancel", "payload": map[string]interface{}{"start_time": "08:00"}},
		{"kind": "add_stop", "payload": map[string]interface{}{"stop_place_id": SchoolAStopID}},
		{"kind": "change_time", "payload": map[string]interface{}{"start_time": "25:00"}},
		{"kind": "detour", "payload": map[string]interface{}{"stops": "Central Station"}},
	}

	for _, attempt := range bad {
		body := map[string]interface{}{
			"date_from": "2026-11-02", "date_to": "2026-11-02",
			"kind": attempt["kind"], "payload": attempt["payload"],
		}
		w := helper.DoRequest("POST", "/tracking/routes/"+routeID+"/exceptions", body, map[string]string{})
		assert.Equal(t, http.StatusBadRequest, w.Code, "%v: %s", attempt, w.Body.String())
		assert.Contains(t, w.Body.String(), "tracking.exception.payload", "%v", attempt)
	}
	assert.Empty(t, ListRouteExceptions(t, helper, routeID, ""))
}

func TestCreateRouteExceptionRejectsUnknownKind(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	routeID := CreateTestRoute(t, helper)

	w := helper.DoRequest("POST", "/tracking/routes/"+routeID+"/exceptions", map[string]interface{}{
		"date_from": "2026-11-02", "date_to": "2026-11-02",
		"kind": "reroute", "payload": map[string]interface{}{},
	}, map[string]string{})

	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
}

// TestCreateRouteExceptionForeignVehicle - an id the payload names has to be this tenant's, or the
// preview would hand a reader another tenant's bus → 404
func TestCreateRouteExceptionForeignVehicle(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	routeID := CreateTestRoute(t, helper)

	w := helper.DoRequest("POST", "/tracking/routes/"+routeID+"/exceptions", map[string]interface{}{
		"date_from": "2026-11-02", "date_to": "2026-11-02",
		"kind": "change_vehicle", "payload": map[string]interface{}{"vehicle_id": uuid.New().String()},
	}, map[string]string{})

	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "tracking.vehicle.not-found")
}

// TestListRouteExceptionsOverlappingTheWindow - an exception that starts before the window and runs
// into it is part of what the window looks like.
func TestListRouteExceptionsOverlappingTheWindow(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	routeID := CreateTestRoute(t, helper)
	CreateRouteException(t, helper, routeID, map[string]interface{}{
		"date_from": "2026-12-20", "date_to": "2027-01-06",
		"kind": "cancel", "payload": map[string]interface{}{}, "reason": "Winter shutdown",
	})
	CreateRouteException(t, helper, routeID, CancelDay("2027-05-01", "Labour day"))

	inWindow := ListRouteExceptions(t, helper, routeID, "?date_from=2027-01-01&date_to=2027-01-31")

	assert.Len(t, inWindow, 1)
	assert.Equal(t, "Winter shutdown", inWindow[0]["reason"])
}

func TestDeleteRouteException(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	routeID := CreateTestRoute(t, helper)
	exceptionID := CreateRouteException(t, helper, routeID, CancelDay("2026-10-03", "Public holiday"))

	w := helper.DoRequest("DELETE", "/tracking/routes/"+routeID+"/exceptions/"+exceptionID, nil, map[string]string{})

	assert.Equal(t, http.StatusNoContent, w.Code, w.Body.String())
	assert.Empty(t, ListRouteExceptions(t, helper, routeID, ""))
}

func TestDeleteRouteExceptionUnknown(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	w := helper.DoRequest("DELETE", "/tracking/routes/"+MorningRouteID+"/exceptions/"+uuid.New().String(),
		nil, map[string]string{})

	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "tracking.exception.not-found")
}

func TestListRouteExceptionsUnknownRoute(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	w := helper.DoRequest("GET", "/tracking/routes/"+uuid.New().String()+"/exceptions", nil, map[string]string{})

	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "tracking.route.not-found")
}

// ============================================================================
// PREVIEW (TRACK-018 step 3)
// ============================================================================

// TestPreviewExcludesTheCalendarsHoliday - the acceptance criterion of the source requirement: a
// Mon/Wed/Fri schedule reading a calendar produces no run on a no_service date.
func TestPreviewExcludesTheCalendarsHoliday(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	routeID := CreateTestRoute(t, helper)
	calendarID := CreateTestCalendar(t, helper)
	ReplaceCalendarDates(t, helper, calendarID, []map[string]interface{}{
		{"date": "2026-10-07", "kind": "no_service", "label": "Founders' day"},
	})
	body := WeekdayScheduleBody("07:00", "2026-01-01")
	body["days_of_week"] = 21 // Monday, Wednesday, Friday
	body["calendar_id"] = calendarID
	CreateRouteSchedule(t, helper, routeID, body)

	plan := PreviewRoute(t, helper, routeID, "2026-10-01", "2026-10-09")

	// Thursday the 1st and the weekend are not Mon/Wed/Fri; Wednesday the 7th is the holiday.
	assert.Equal(t, []string{"2026-10-02", "2026-10-05", "2026-10-09"}, ServiceDates(plan))
	for _, day := range plan {
		assert.Equal(t, "planned", day["status"])
		assert.Equal(t, "07:00", day["start_time"])
	}
}

// TestPreviewSpecialServiceAddsADay - the other direction: a date the weekday mask excludes on
// which the route runs anyway.
func TestPreviewSpecialServiceAddsADay(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	routeID := CreateTestRoute(t, helper)
	calendarID := CreateTestCalendar(t, helper)
	ReplaceCalendarDates(t, helper, calendarID, []map[string]interface{}{
		{"date": "2026-10-03", "kind": "special_service", "label": "Make-up Saturday"},
	})
	body := WeekdayScheduleBody("07:00", "2026-01-01")
	body["calendar_id"] = calendarID
	CreateRouteSchedule(t, helper, routeID, body)

	plan := PreviewRoute(t, helper, routeID, "2026-10-03", "2026-10-04")

	assert.Equal(t, []string{"2026-10-03"}, ServiceDates(plan), "the Saturday runs, the Sunday does not")
}

// TestPreviewCancelAffectsThatDateOnly - "only 2026-10-03: cancel" marks that day and leaves every
// other day of the window planned.
func TestPreviewCancelAffectsThatDateOnly(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	routeID := CreateTestRoute(t, helper)
	body := WeekdayScheduleBody("07:00", "2026-01-01")
	body["days_of_week"] = 127 // every day
	CreateRouteSchedule(t, helper, routeID, body)
	CreateRouteException(t, helper, routeID, CancelDay("2026-10-03", "Strike"))

	plan := PreviewRoute(t, helper, routeID, "2026-10-01", "2026-10-05")

	if len(plan) != 5 {
		t.Fatalf("expected five days in the plan, got %d", len(plan))
	}
	for _, day := range plan {
		if day["service_date"] == "2026-10-03" {
			assert.Equal(t, "cancelled", day["status"])
			assert.Len(t, day["exceptions"], 1)
			continue
		}
		assert.Equal(t, "planned", day["status"], day["service_date"])
		assert.Empty(t, day["exceptions"], day["service_date"])
	}
}

// TestPreviewChangeTimeOverridesTheDeparture - an exception that moves one day's departure shows the
// moved time, and the days around it keep the schedule's.
func TestPreviewChangeTimeOverridesTheDeparture(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	routeID := CreateTestRoute(t, helper)
	body := WeekdayScheduleBody("07:00", "2026-01-01")
	body["days_of_week"] = 127
	CreateRouteSchedule(t, helper, routeID, body)
	CreateRouteException(t, helper, routeID, map[string]interface{}{
		"date_from": "2026-10-02", "date_to": "2026-10-02",
		"kind": "change_time", "payload": map[string]interface{}{"start_time": "09:30"},
	})

	plan := PreviewRoute(t, helper, routeID, "2026-10-01", "2026-10-03")

	assert.Equal(t, "07:00", plan[0]["start_time"])
	assert.Equal(t, "09:30", plan[1]["start_time"])
	assert.Equal(t, "planned", plan[1]["status"], "a moved departure still runs")
	assert.Equal(t, "07:00", plan[2]["start_time"])
}

// TestPreviewNamesTheVersionInForce - every day of the plan names the stop list a trip would run
// that day (D3), so publishing a new version moves the boundary and nothing else.
func TestPreviewNamesTheVersionInForce(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	routeID := CreateTestRoute(t, helper)
	first := PublishRouteVersion(t, helper, routeID, "2026-10-01", []map[string]interface{}{
		{"stop_place_id": CentralStationID, "sequence": 1},
	})
	second := PublishRouteVersion(t, helper, routeID, "2026-10-03", []map[string]interface{}{
		{"stop_place_id": SchoolAStopID, "sequence": 1},
	})
	body := WeekdayScheduleBody("07:00", "2026-01-01")
	body["days_of_week"] = 127
	CreateRouteSchedule(t, helper, routeID, body)

	plan := PreviewRoute(t, helper, routeID, "2026-10-01", "2026-10-04")

	assert.Equal(t, first, plan[0]["version_id"], "the 1st runs the list published for the 1st")
	assert.Equal(t, first, plan[1]["version_id"])
	assert.Equal(t, second, plan[2]["version_id"], "the 3rd runs the one published for the 3rd")
	assert.Equal(t, second, plan[3]["version_id"])
}

// TestPreviewTwoSchedulesOneDay - a route that leaves twice a day is two rows for that date, in
// departure order.
func TestPreviewTwoSchedulesOneDay(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	routeID := CreateTestRoute(t, helper)
	body := WeekdayScheduleBody("07:00", "2026-01-01")
	body["days_of_week"] = 127
	CreateRouteSchedule(t, helper, routeID, body)
	body = WeekdayScheduleBody("16:30", "2026-01-01")
	body["days_of_week"] = 127
	CreateRouteSchedule(t, helper, routeID, body)

	plan := PreviewRoute(t, helper, routeID, "2026-10-01", "2026-10-01")

	if len(plan) != 2 {
		t.Fatalf("expected two departures that day, got %d", len(plan))
	}
	assert.Equal(t, "07:00", plan[0]["start_time"])
	assert.Equal(t, "16:30", plan[1]["start_time"])
	assert.NotEqual(t, plan[0]["schedule_id"], plan[1]["schedule_id"])
}

// TestPreviewEmptyBeforeTheScheduleStarts - a route with no recurrence in the window plans nothing,
// which is an empty list and not a refusal.
func TestPreviewEmptyBeforeTheScheduleStarts(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	routeID := CreateTestRoute(t, helper)
	CreateRouteSchedule(t, helper, routeID, WeekdayScheduleBody("07:00", "2027-01-01"))

	assert.Empty(t, PreviewRoute(t, helper, routeID, "2026-10-01", "2026-10-31"))
}

// TestPreviewRangeTooWide - a quarter is what the screen shows; wider is one generate_series away
// from a request that scans years → 400
func TestPreviewRangeTooWide(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	w := helper.DoRequest("GET", "/tracking/routes/"+MorningRouteID+"/preview?from=2026-01-01&to=2026-12-31",
		nil, map[string]string{})

	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "tracking.route.preview-range")
}

func TestPreviewRequiresBothBounds(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	w := helper.DoRequest("GET", "/tracking/routes/"+MorningRouteID+"/preview?from=2026-10-01", nil, map[string]string{})

	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
}

func TestPreviewUnknownRoute(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	w := helper.DoRequest("GET", "/tracking/routes/"+uuid.New().String()+"/preview?from=2026-10-01&to=2026-10-31",
		nil, map[string]string{})

	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "tracking.route.not-found")
}

// TestPreviewWritesNothing - the promise the whole design rests on: TRACK-008 materialises trips
// from this query, so reading the plan must leave no row behind, in the tables or in the outbox.
func TestPreviewWritesNothing(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	routeID := CreateTestRoute(t, helper)
	body := WeekdayScheduleBody("07:00", "2026-01-01")
	body["days_of_week"] = 127
	CreateRouteSchedule(t, helper, routeID, body)
	CreateRouteException(t, helper, routeID, CancelDay("2026-10-03", "Strike"))
	before := planningRowCounts(t, helper)

	plan := PreviewRoute(t, helper, routeID, "2026-10-01", "2026-10-31")

	assert.NotEmpty(t, plan)
	assert.Equal(t, before, planningRowCounts(t, helper), "a preview writes nothing")
}

// planningRowCounts is every table TRACK-018 writes, plus the outbox: what "the preview writes
// nothing" is measured against.
func planningRowCounts(t *testing.T, helper *testhelpers.ApiTestHelper) map[string]int {
	t.Helper()
	counts := map[string]int{}
	for _, table := range []string{"route_schedules", "calendars", "calendar_dates", "route_exceptions", "outbox"} {
		var n int
		if err := helper.DB().QueryRow(context.Background(),
			`SELECT count(*) FROM tracking.`+table).Scan(&n); err != nil {
			t.Fatalf("count tracking.%s: %v", table, err)
		}
		counts[table] = n
	}
	return counts
}
