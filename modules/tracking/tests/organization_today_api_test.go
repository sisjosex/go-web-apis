//go:build integration
// +build integration

package tracking_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"josex/web/modules/core/testhelpers"
	"josex/web/modules/tracking/models"
)

// ---- MOBILE-039: a school's or company's day, and the passenger's pass ----

// newOrganization creates a client of kind with no rider yet: its day counts only what a test seats.
func newOrganization(t *testing.T, helper *testhelpers.ApiTestHelper, kind string) string {
	t.Helper()
	dto := ValidOrganizationDto()
	dto.Kind = kind
	w := helper.DoRequest("POST", "/tracking/organizations", OrganizationBody(dto), map[string]string{})
	if w.Code != http.StatusCreated {
		t.Fatalf("create organization: %d %s", w.Code, w.Body.String())
	}
	return ExtractID(t, ParseResponse(t, w.Body.Bytes()))
}

// orgRider seats a new rider of organizationID on routeID.
func orgRider(t *testing.T, helper *testhelpers.ApiTestHelper, organizationID, routeID string) string {
	t.Helper()
	riderID := createRider(t, helper, map[string]interface{}{"organization_id": organizationID})
	assignRider(t, helper, riderID, routeID)
	return riderID
}

func organizationToday(t *testing.T, helper *testhelpers.ApiTestHelper, query string) models.OrganizationToday {
	t.Helper()
	w := helper.DoRequest("GET", "/tracking/organization/today?"+query, nil, map[string]string{})
	if w.Code != http.StatusOK {
		t.Fatalf("organization today: %d %s", w.Code, w.Body.String())
	}
	var day models.OrganizationToday
	decodeBody(t, w, &day)
	return day
}

// TestOrganizationAbsent_ListsWhoTheDayCounts - only the absent rider, by name; staff must name the organization.
func TestOrganizationAbsent_ListsWhoTheDayCounts(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	org := newOrganization(t, helper, "school")
	routeID, _ := createTripRoute(t, helper, "07:00")
	present, absent := orgRider(t, helper, org, routeID), orgRider(t, helper, org, routeID)
	today := dayOffset(t, "UTC", 0)
	materialiseRoute(t, helper, routeID, today, today)
	postAbsence(t, helper, absent, map[string]interface{}{"date_from": today, "date_to": today}, http.StatusCreated)

	w := helper.DoRequest("GET", "/tracking/organization/absent?organization_id="+org+"&date="+today, nil, map[string]string{})
	if w.Code != http.StatusOK {
		t.Fatalf("organization absent: %d %s", w.Code, w.Body.String())
	}
	var list models.OrganizationAbsent
	decodeBody(t, w, &list)
	if assert.Len(t, list.Riders, 1) {
		assert.Equal(t, absent, list.Riders[0].RiderID.String())
		assert.NotEqual(t, present, list.Riders[0].RiderID.String())
		assert.NotEmpty(t, list.Riders[0].Name)
	}

	w = helper.DoRequest("GET", "/tracking/organization/absent", nil, map[string]string{})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestOrganizationToday_SchoolTotals - three of a school's riders on a running trip: one arrived, one on
// the way, one absent; staff must name the organization; a school user reads its own.
func TestOrganizationToday_SchoolTotals(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	org := newOrganization(t, helper, "school")
	routeID, _ := createTripRoute(t, helper, "07:00")
	arrived, onTheWay, absent := orgRider(t, helper, org, routeID), orgRider(t, helper, org, routeID), orgRider(t, helper, org, routeID)
	today := dayOffset(t, "UTC", 0)
	materialiseRoute(t, helper, routeID, today, today)
	tripID, _ := tripOn(t, helper, routeID, today, "07:00")
	tripRequest(t, helper, "PATCH", "/tracking/trips/"+tripID, map[string]interface{}{"vehicle_id": CreateTestVehicleOf(t, helper, MainCompanyID)}, http.StatusOK)
	tripRequest(t, helper, "POST", "/tracking/trips/"+tripID+"/start", nil, http.StatusOK)
	postAbsence(t, helper, absent, map[string]interface{}{"date_from": today, "date_to": today}, http.StatusCreated)
	pickup, dropoff := todayTasks(t, helper, tripID, arrived)
	doneTask(t, helper, pickup)
	doneTask(t, helper, dropoff)
	pickup, _ = todayTasks(t, helper, tripID, onTheWay)
	doneTask(t, helper, pickup)

	day := organizationToday(t, helper, "organization_id="+org+"&date="+today)
	assert.Equal(t, org, day.Organization.ID.String())
	assert.Equal(t, "school", day.Organization.Kind)
	// A 07:00 trip still running hours later is late to its shift: both riders who ride count as late.
	assert.Equal(t, models.OrganizationTodayTotals{Expected: 3, Arrived: 1, OnTheWay: 1, Absent: 1, Late: 2, OnTime: 0}, day.Totals)
	if assert.Len(t, day.Routes, 1) {
		assert.Equal(t, tripID, day.Routes[0].TripID.String())
		assert.Equal(t, 2, day.Routes[0].Riders, "the absent rider does not ride")
		assert.Equal(t, "in_progress", day.Routes[0].Status)
		assert.NotNil(t, day.Routes[0].ExpectedArrival)
	}

	w := helper.DoRequest("GET", "/tracking/organization/today", nil, map[string]string{})
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "tracking.organization.required")

	school := SetupOrganizationTest(t)
	defer school.Close()
	own := organizationToday(t, school, "")
	assert.Equal(t, MainSchoolID, own.Organization.ID.String(), "a school user reads its own organization")
	w = school.DoRequest("GET", "/tracking/organization/today?organization_id="+org, nil, map[string]string{})
	assert.Equal(t, http.StatusNotFound, w.Code, "another client's day: %s", w.Body.String())
}

// TestOrganizationToday_CompanyShifts - a company's trips in arriving at 08:00 and 14:00 answer both
// shifts, and shift=14:00 narrows the routes and totals to that one (D2).
func TestOrganizationToday_CompanyShifts(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	org := newOrganization(t, helper, "company")
	morning, _ := createTripRoute(t, helper, "07:30")
	afternoon, _ := createTripRoute(t, helper, "13:30")
	orgRider(t, helper, org, morning)
	orgRider(t, helper, org, afternoon)
	orgRider(t, helper, org, afternoon)
	today := dayOffset(t, "UTC", 0)
	materialiseRoute(t, helper, morning, today, today)
	materialiseRoute(t, helper, afternoon, today, today)

	day := organizationToday(t, helper, "organization_id="+org+"&date="+today)
	assert.Equal(t, []string{"08:00", "14:00"}, day.Shifts)
	assert.Len(t, day.Routes, 2)
	assert.Equal(t, 3, day.Totals.Expected)

	shift := organizationToday(t, helper, "organization_id="+org+"&date="+today+"&shift=14:00")
	if assert.NotNil(t, shift.Shift) {
		assert.Equal(t, "14:00", *shift.Shift)
	}
	if assert.Len(t, shift.Routes, 1) {
		assert.Equal(t, afternoon, shift.Routes[0].RouteID.String())
	}
	assert.Equal(t, 2, shift.Totals.Expected)
	assert.Equal(t, []string{"08:00", "14:00"}, shift.Shifts, "the selector keeps every shift")
}

// TestPortalMe_SelfPass - the passenger's own account reads its card code on /me; a guardian never does.
func TestPortalMe_SelfPass(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	passenger := CreateTestRider(t, helper)
	LinkPortalSelf(t, helper, passenger)

	if self := meRider(t, passenger); assert.NotNil(t, self, "a self contact is in scope") && assert.NotNil(t, self.QRToken) {
		assert.Len(t, *self.QRToken, 32)
	}
	if guarded := meRider(t, TestRiderJohnID); assert.NotNil(t, guarded) {
		assert.Nil(t, guarded.QRToken, "a guardian never gets the pass")
	}
}
