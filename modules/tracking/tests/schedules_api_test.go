//go:build integration
// +build integration

package tracking_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// ============================================================================
// ROUTE SCHEDULES (TRACK-018 step 2)
// ============================================================================

func TestListRouteSchedulesShowsTheSeededOne(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	schedules := ListRouteSchedules(t, helper, MorningRouteID)

	if len(schedules) != 1 {
		t.Fatalf("expected one seeded schedule, got %d", len(schedules))
	}
	assert.Equal(t, MorningScheduleID, schedules[0]["id"])
	assert.Equal(t, float64(WeekdaysMask), schedules[0]["days_of_week"])
	assert.Equal(t, "07:00", schedules[0]["start_time"], "start_time crosses the wire as HH:MM")
	assert.Equal(t, "2026-01-01", schedules[0]["valid_from"])
	assert.Nil(t, schedules[0]["valid_until"])
	assert.Nil(t, schedules[0]["calendar_id"])
}

func TestCreateRouteScheduleSuccess(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	routeID := CreateTestRoute(t, helper)

	w := helper.DoRequest("POST", "/tracking/routes/"+routeID+"/schedules", map[string]interface{}{
		"days_of_week": 21, // Monday, Wednesday, Friday
		"start_time":   "07:00",
		"valid_from":   "2026-09-01",
		"valid_until":  "2027-06-30",
	}, map[string]string{})

	assert.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	created := ParseResponse(t, w.Body.Bytes())
	assert.Equal(t, float64(21), created["days_of_week"])
	assert.Equal(t, "07:00", created["start_time"])
	assert.Equal(t, "2027-06-30", created["valid_until"])
	assert.Equal(t, routeID, created["route_id"])
}

// TestCreateRouteScheduleOverlap - the same route may not leave twice at the same time of day on
// the same date; that is the exclusion constraint, reported as a conflict → 409
func TestCreateRouteScheduleOverlap(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	w := helper.DoRequest("POST", "/tracking/routes/"+MorningRouteID+"/schedules",
		WeekdayScheduleBody("07:00", "2026-06-01"), map[string]string{})

	assert.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "tracking.schedule.overlap")
}

// TestCreateRouteScheduleSameDayDifferentTime - a second departure on the same days is not an
// overlap: a route that runs morning and afternoon is two schedules.
func TestCreateRouteScheduleSameDayDifferentTime(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	routeID := CreateTestRoute(t, helper)
	CreateRouteSchedule(t, helper, routeID, WeekdayScheduleBody("07:00", "2026-06-01"))

	w := helper.DoRequest("POST", "/tracking/routes/"+routeID+"/schedules",
		WeekdayScheduleBody("18:00", "2026-06-01"), map[string]string{})

	assert.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	assert.Len(t, ListRouteSchedules(t, helper, routeID), 2)
}

// TestCreateRouteScheduleRejectsEmptyWeekdays - 0 is a schedule that never runs, which is a delete
// and not a schedule → 400
func TestCreateRouteScheduleRejectsEmptyWeekdays(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	routeID := CreateTestRoute(t, helper)

	w := helper.DoRequest("POST", "/tracking/routes/"+routeID+"/schedules", map[string]interface{}{
		"days_of_week": 0,
		"start_time":   "07:00",
		"valid_from":   "2026-09-01",
	}, map[string]string{})

	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
}

func TestCreateRouteScheduleRejectsMalformedTime(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	routeID := CreateTestRoute(t, helper)

	w := helper.DoRequest("POST", "/tracking/routes/"+routeID+"/schedules", map[string]interface{}{
		"days_of_week": WeekdaysMask,
		"start_time":   "7am",
		"valid_from":   "2026-09-01",
	}, map[string]string{})

	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
}

func TestCreateRouteScheduleUnknownCalendar(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	routeID := CreateTestRoute(t, helper)
	body := WeekdayScheduleBody("07:00", "2026-09-01")
	body["calendar_id"] = uuid.New().String()

	w := helper.DoRequest("POST", "/tracking/routes/"+routeID+"/schedules", body, map[string]string{})

	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "tracking.calendar.not-found")
}

func TestUpdateRouteScheduleKeepsWhatItIsNotSent(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	routeID := CreateTestRoute(t, helper)
	scheduleID := CreateRouteSchedule(t, helper, routeID, WeekdayScheduleBody("07:00", "2026-09-01"))
	calendarID := CreateTestCalendar(t, helper)

	w := helper.DoRequest("PATCH", "/tracking/routes/"+routeID+"/schedules/"+scheduleID,
		map[string]interface{}{"calendar_id": calendarID}, map[string]string{})

	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	updated := ParseResponse(t, w.Body.Bytes())
	assert.Equal(t, calendarID, updated["calendar_id"])
	assert.Equal(t, "07:00", updated["start_time"], "a field not sent keeps its value")
	assert.Equal(t, float64(WeekdaysMask), updated["days_of_week"])
	assert.NotNil(t, updated["calendar_name"])
}

func TestUpdateRouteScheduleUnknown(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	w := helper.DoRequest("PATCH", "/tracking/routes/"+MorningRouteID+"/schedules/"+uuid.New().String(),
		map[string]interface{}{"start_time": "08:00"}, map[string]string{})

	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "tracking.schedule.not-found")
}

func TestDeleteRouteSchedule(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	routeID := CreateTestRoute(t, helper)
	scheduleID := CreateRouteSchedule(t, helper, routeID, WeekdayScheduleBody("07:00", "2026-09-01"))

	w := helper.DoRequest("DELETE", "/tracking/routes/"+routeID+"/schedules/"+scheduleID, nil, map[string]string{})

	assert.Equal(t, http.StatusNoContent, w.Code, w.Body.String())
	assert.Empty(t, ListRouteSchedules(t, helper, routeID))
}

// TestSplitScheduleLeavesTwoHalvesWithNoGap - "from the 5th it leaves at 07:30": the old half closes
// on the 4th and the new one opens on the 5th, so the calendar is covered exactly once.
func TestSplitScheduleLeavesTwoHalvesWithNoGap(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	routeID := CreateTestRoute(t, helper)
	scheduleID := CreateRouteSchedule(t, helper, routeID, WeekdayScheduleBody("07:00", "2026-09-01"))

	w := helper.DoRequest("POST", "/tracking/routes/"+routeID+"/schedules/"+scheduleID+"/split",
		map[string]interface{}{
			"from_date": "2026-10-05",
			"changes":   map[string]interface{}{"start_time": "07:30"},
		}, map[string]string{})

	assert.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	result := ParseResponse(t, w.Body.Bytes())
	closed := result["closed"].(map[string]interface{})
	created := result["created"].(map[string]interface{})
	assert.Equal(t, scheduleID, closed["id"])
	assert.Equal(t, "2026-10-04", closed["valid_until"], "the old half closes the day before")
	assert.Equal(t, "07:00", closed["start_time"], "the days before keep the plan they had")
	assert.Equal(t, "2026-10-05", created["valid_from"])
	assert.Equal(t, "07:30", created["start_time"])
	assert.Nil(t, created["valid_until"], "the new half inherits the open end")

	schedules := ListRouteSchedules(t, helper, routeID)
	if len(schedules) != 2 {
		t.Fatalf("expected two halves after the split, got %d", len(schedules))
	}
	assert.Equal(t, "2026-10-05", schedules[0]["valid_from"], "newest validity first")
	assert.Equal(t, "2026-10-04", schedules[1]["valid_until"])
}

// TestSplitScheduleCarriesTheWeekdaysWhenNotChanged - a change that names only the time leaves the
// weekdays as they were.
func TestSplitScheduleCarriesTheWeekdaysWhenNotChanged(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	routeID := CreateTestRoute(t, helper)
	scheduleID := CreateRouteSchedule(t, helper, routeID, WeekdayScheduleBody("07:00", "2026-09-01"))

	w := helper.DoRequest("POST", "/tracking/routes/"+routeID+"/schedules/"+scheduleID+"/split",
		map[string]interface{}{
			"from_date": "2026-10-05",
			"changes":   map[string]interface{}{"days_of_week": 21},
		}, map[string]string{})

	assert.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	created := ParseResponse(t, w.Body.Bytes())["created"].(map[string]interface{})
	assert.Equal(t, float64(21), created["days_of_week"])
	assert.Equal(t, "07:00", created["start_time"], "what the change does not name is carried over")
}

// TestSplitScheduleBeforeItStarts - there is nothing to close before the schedule's first day → 400
func TestSplitScheduleBeforeItStarts(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	w := helper.DoRequest("POST", "/tracking/routes/"+MorningRouteID+"/schedules/"+MorningScheduleID+"/split",
		map[string]interface{}{"from_date": "2025-06-01", "changes": map[string]interface{}{}},
		map[string]string{})

	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "tracking.schedule.split-date")
}

// TestSplitScheduleAfterItEnds - there is nothing to carry past the schedule's last day → 400
func TestSplitScheduleAfterItEnds(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	routeID := CreateTestRoute(t, helper)
	body := WeekdayScheduleBody("07:00", "2026-09-01")
	body["valid_until"] = "2026-12-31"
	scheduleID := CreateRouteSchedule(t, helper, routeID, body)

	w := helper.DoRequest("POST", "/tracking/routes/"+routeID+"/schedules/"+scheduleID+"/split",
		map[string]interface{}{"from_date": "2027-03-01", "changes": map[string]interface{}{}},
		map[string]string{})

	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "tracking.schedule.split-date")
}

func TestListRouteSchedulesUnknownRoute(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	w := helper.DoRequest("GET", "/tracking/routes/"+uuid.New().String()+"/schedules", nil, map[string]string{})

	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "tracking.route.not-found")
}

// ============================================================================
// CALENDARS (TRACK-018 step 2)
// ============================================================================

// TestReplaceCalendarDatesSavesTheWholeList - a school year is saved once: twelve holidays go in
// one PUT and twelve come back.
func TestReplaceCalendarDatesSavesTheWholeList(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	calendarID := CreateTestCalendar(t, helper)

	stored := ReplaceCalendarDates(t, helper, calendarID, NoServiceDates("2026-12-21", 12))

	assert.Len(t, stored, 12)
	w := helper.DoRequest("GET", "/tracking/calendars/"+calendarID+"/dates", nil, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	read := ParseListResponse(t, w.Body.Bytes())
	assert.Len(t, read, 12)
	assert.Equal(t, "2026-12-21", read[0]["date"], "oldest first")
	assert.Equal(t, "no_service", read[0]["kind"])
	assert.Equal(t, "Holiday", read[0]["label"])
}

// TestReplaceCalendarDatesIsAReplace - the list that arrives is the calendar, so a date left out is
// gone rather than merged.
func TestReplaceCalendarDatesIsAReplace(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	calendarID := CreateTestCalendar(t, helper)
	ReplaceCalendarDates(t, helper, calendarID, NoServiceDates("2026-12-21", 12))

	stored := ReplaceCalendarDates(t, helper, calendarID, NoServiceDates("2026-12-25", 1))

	assert.Len(t, stored, 1)
	assert.Equal(t, "2026-12-25", stored[0]["date"])
}

func TestListCalendarDatesNarrowedToAWindow(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	calendarID := CreateTestCalendar(t, helper)
	ReplaceCalendarDates(t, helper, calendarID, NoServiceDates("2026-12-21", 12))

	w := helper.DoRequest("GET", "/tracking/calendars/"+calendarID+"/dates?from=2026-12-24&to=2026-12-26",
		nil, map[string]string{})

	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Len(t, ParseListResponse(t, w.Body.Bytes()), 3)
}

// TestReplaceCalendarDatesRejectsADateTwice - which of the two the calendar would keep is not the
// caller's to leave unsaid → 409
func TestReplaceCalendarDatesRejectsADateTwice(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	calendarID := CreateTestCalendar(t, helper)

	w := helper.DoRequest("PUT", "/tracking/calendars/"+calendarID+"/dates", []map[string]interface{}{
		{"date": "2026-12-25", "kind": "no_service", "label": "Christmas"},
		{"date": "2026-12-25", "kind": "special_service", "label": "Extra run"},
	}, map[string]string{})

	assert.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "tracking.calendar.duplicate-date")
}

func TestReplaceCalendarDatesRejectsAnUnknownKind(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	calendarID := CreateTestCalendar(t, helper)

	w := helper.DoRequest("PUT", "/tracking/calendars/"+calendarID+"/dates", []map[string]interface{}{
		{"date": "2026-12-25", "kind": "maybe", "label": "Christmas"},
	}, map[string]string{})

	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
}

func TestListCalendarsPaged(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	CreateTestCalendar(t, helper)
	CreateTestCalendar(t, helper)

	result := ListCalendars(t, helper, "?page=1&page_size=1")

	assert.Len(t, result.Calendars, 1)
	assert.GreaterOrEqual(t, result.TotalCount, int64(2))
	assert.Equal(t, 1, result.Page)
	assert.Equal(t, 1, result.PageSize)
}

func TestListCalendarsPageSizeAboveCapReturns400(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	w := helper.DoRequest("GET", "/tracking/calendars?page_size=500", nil, map[string]string{})

	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
}

func TestUpdateCalendarRenamesIt(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	calendarID := CreateTestCalendar(t, helper)

	w := helper.DoRequest("PATCH", "/tracking/calendars/"+calendarID,
		map[string]interface{}{"name": "School year 2027"}, map[string]string{})

	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, "School year 2027", ParseResponse(t, w.Body.Bytes())["name"])
}

// TestDeleteCalendarInUse - a calendar a schedule still reads is not deleted: the schedule would
// silently start running on the holidays → 409
func TestDeleteCalendarInUse(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	routeID := CreateTestRoute(t, helper)
	calendarID := CreateTestCalendar(t, helper)
	body := WeekdayScheduleBody("07:00", "2026-09-01")
	body["calendar_id"] = calendarID
	CreateRouteSchedule(t, helper, routeID, body)

	w := helper.DoRequest("DELETE", "/tracking/calendars/"+calendarID, nil, map[string]string{})

	assert.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "tracking.calendar.in-use")
}

func TestDeleteCalendarUnused(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	calendarID := CreateTestCalendar(t, helper)

	w := helper.DoRequest("DELETE", "/tracking/calendars/"+calendarID, nil, map[string]string{})

	assert.Equal(t, http.StatusNoContent, w.Code, w.Body.String())
	w = helper.DoRequest("GET", "/tracking/calendars/"+calendarID+"/dates", nil, map[string]string{})
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
}

func TestCalendarDatesUnknownCalendar(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	w := helper.DoRequest("GET", "/tracking/calendars/"+uuid.New().String()+"/dates", nil, map[string]string{})

	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "tracking.calendar.not-found")
}

func TestCreateCalendarUnknownOrganization(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	w := helper.DoRequest("POST", "/tracking/calendars", map[string]interface{}{
		"name":            "Foreign school year",
		"organization_id": uuid.New().String(),
	}, map[string]string{})

	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "tracking.organization.not-found")
}
