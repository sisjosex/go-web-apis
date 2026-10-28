//go:build integration
// +build integration

package tracking_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"josex/web/modules/core/testhelpers"
	"josex/web/modules/tracking/models"
)

// ============================================================================
// TENANT AND CLIENT CALENDARS (TRACK-076 D1)
// ============================================================================

// addScopeDates marks from..to as no_service on the tenant calendar (organizationID "") or a client's,
// and gives every day back when the test ends: the tenant calendar reaches every route of the run.
func addScopeDates(t *testing.T, helper *testhelpers.ApiTestHelper, organizationID, from, to string) models.AddScopeCalendarDatesResponse {
	t.Helper()
	body := map[string]interface{}{"from": from, "to": to, "kind": "no_service", "label": "Día del Mar"}
	if organizationID != "" {
		body["organization_id"] = organizationID
	}
	w := helper.DoRequest("POST", "/tracking/calendar-dates", body, map[string]string{})
	expectStatus(t, w, http.StatusCreated)
	var added models.AddScopeCalendarDatesResponse
	decodeBody(t, w, &added)
	t.Cleanup(func() {
		start, _ := time.Parse(time.DateOnly, from)
		end, _ := time.Parse(time.DateOnly, to)
		for day := start; !day.After(end); day = day.AddDate(0, 0, 1) {
			// A test that removed its day already gets a 404 here, which is fine.
			helper.DoRequest("DELETE", "/tracking/calendar-dates/"+added.CalendarID.String()+"/"+day.Format(time.DateOnly), nil, map[string]string{})
		}
	})
	return added
}

// listScopeDates reads the month page's dates over a window.
func listScopeDates(t *testing.T, helper *testhelpers.ApiTestHelper, from, to string) []models.ScopeCalendarDate {
	t.Helper()
	w := helper.DoRequest("GET", "/tracking/calendar-dates?from="+from+"&to="+to, nil, map[string]string{})
	expectStatus(t, w, http.StatusOK)
	var page models.ListScopeCalendarDatesResponse
	decodeBody(t, w, &page)
	dates := make([]models.ScopeCalendarDate, 0, len(page.Dates))
	for _, d := range page.Dates {
		dates = append(dates, *d)
	}
	return dates
}

// everyDayRoute is a route that runs every day with no calendar on its schedule.
func everyDayRoute(t *testing.T, helper *testhelpers.ApiTestHelper) string {
	t.Helper()
	routeID := CreateTestRoute(t, helper)
	body := WeekdayScheduleBody("07:00", "2026-01-01")
	body["days_of_week"] = EveryDayMask
	CreateRouteSchedule(t, helper, routeID, body)
	return routeID
}

// TestScopeCalendar_TenantDateDropsEveryRoute - a holiday on the tenant calendar drops that day from
// a route whose schedule names no calendar, and removing it brings the day back.
func TestScopeCalendar_TenantDateDropsEveryRoute(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	routeID := everyDayRoute(t, helper)

	added := addScopeDates(t, helper, "", "2027-03-10", "2027-03-10")
	assert.Equal(t, int32(1), added.Added)

	plan := PreviewRoute(t, helper, routeID, "2027-03-09", "2027-03-11")
	assert.Equal(t, []string{"2027-03-09", "2027-03-11"}, ServiceDates(plan))

	listed := listScopeDates(t, helper, "2027-03-01", "2027-03-31")
	if assert.Len(t, listed, 1) {
		assert.Equal(t, "tenant", listed[0].Scope)
		assert.Equal(t, "no_service", listed[0].Kind)
		assert.Nil(t, listed[0].OrganizationID)
	}

	w := helper.DoRequest("DELETE", "/tracking/calendar-dates/"+added.CalendarID.String()+"/2027-03-10", nil, map[string]string{})
	expectStatus(t, w, http.StatusNoContent)
	plan = PreviewRoute(t, helper, routeID, "2027-03-09", "2027-03-11")
	assert.Equal(t, []string{"2027-03-09", "2027-03-10", "2027-03-11"}, ServiceDates(plan))
}

// TestScopeCalendar_ClientDateDropsOnlyItsRoutes - a client's vacation drops its routes' days and
// leaves another route running.
func TestScopeCalendar_ClientDateDropsOnlyItsRoutes(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	client := createPinnedClient(t, helper, nil)
	clientID := client["id"].(string)
	clientRoute := everyDayRoute(t, helper)
	SetRouteClient(t, helper, clientRoute, clientID)
	otherRoute := everyDayRoute(t, helper)

	addScopeDates(t, helper, clientID, "2027-03-15", "2027-03-16")

	assert.Equal(t, []string{"2027-03-14", "2027-03-17"},
		ServiceDates(PreviewRoute(t, helper, clientRoute, "2027-03-14", "2027-03-17")))
	assert.Len(t, PreviewRoute(t, helper, otherRoute, "2027-03-14", "2027-03-17"), 4)

	listed := listScopeDates(t, helper, "2027-03-15", "2027-03-16")
	if assert.Len(t, listed, 2) {
		assert.Equal(t, "organization", listed[0].Scope)
		assert.Equal(t, client["name"], *listed[0].OrganizationName)
	}
}

// TestScopeCalendar_RangeTooWide - more than 92 days at once is a school year, refused.
func TestScopeCalendar_RangeTooWide(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	w := helper.DoRequest("POST", "/tracking/calendar-dates", map[string]interface{}{
		"from": "2027-01-01", "to": "2027-04-30", "kind": "no_service",
	}, map[string]string{})
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "tracking.calendar.range-too-wide")
}

// TestScopeCalendar_DeleteScheduleCalendarDateRefused - the month page only removes the days it adds:
// a named schedule calendar's date is not found there.
func TestScopeCalendar_DeleteScheduleCalendarDateRefused(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	calendarID := CreateTestCalendar(t, helper)
	ReplaceCalendarDates(t, helper, calendarID, NoServiceDates("2027-03-20", 1))

	w := helper.DoRequest("DELETE", "/tracking/calendar-dates/"+calendarID+"/2027-03-20", nil, map[string]string{})
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
}

// TestScopeCalendar_TripsCancelledByRouteChanged - the trips of a newly marked day are cancelled by
// the route.changed pass the add wrote, and planned again once the day is removed.
func TestScopeCalendar_TripsCancelledByRouteChanged(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	routeID, _ := createTripRoute(t, helper, "07:00")
	day := dayOffset(t, "UTC", 3)
	materialiseRoute(t, helper, routeID, day, day)
	assert.Equal(t, "planned", TripStatusOn(t, helper, routeID, day))

	added := addScopeDates(t, helper, "", day, day)
	materialiseRoute(t, helper, routeID, day, day)
	assert.Equal(t, "cancelled", TripStatusOn(t, helper, routeID, day))

	w := helper.DoRequest("DELETE", "/tracking/calendar-dates/"+added.CalendarID.String()+"/"+day, nil, map[string]string{})
	expectStatus(t, w, http.StatusNoContent)
	materialiseRoute(t, helper, routeID, day, day)
	assert.Equal(t, "planned", TripStatusOn(t, helper, routeID, day))
}
