//go:build integration
// +build integration

package tracking_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	coreJobs "josex/web/modules/core/jobs"
	"josex/web/modules/core/testhelpers"
	trackingJobs "josex/web/modules/tracking/jobs"
)

// ============================================================================
// RIDERLESS TRIPS CANCEL THEMSELVES (TRACK-069 D1, D2)
// ============================================================================

// createLeadOrganization creates a client whose riderless trips cancel lead minutes before departure.
func createLeadOrganization(t *testing.T, helper *testhelpers.ApiTestHelper, lead int) string {
	t.Helper()
	body := OrganizationBody(ValidOrganizationDto())
	body["empty_trip_cancel_min"] = lead
	w := helper.DoRequest("POST", "/tracking/organizations", body, map[string]string{})
	if w.Code != http.StatusCreated {
		t.Fatalf("create organization: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	data := ParseResponse(t, w.Body.Bytes())
	assert.Equal(t, float64(lead), data["empty_trip_cancel_min"])
	return ExtractID(t, data)
}

// clientTrip is today's 07:00 trip of a new route of organizationID, moved to start in minutes from
// now; with a rider when riders, without one otherwise.
func clientTrip(t *testing.T, helper *testhelpers.ApiTestHelper, organizationID string, minutes int, riders bool) string {
	t.Helper()
	var routeID string
	if riders {
		routeID, _ = createTripRoute(t, helper, "07:00")
	} else {
		routeID = CreateTestRoute(t, helper)
		PublishRouteVersion(t, helper, routeID, "2026-01-01", []map[string]interface{}{
			{"stop_place_id": CentralStationID, "sequence": 1, "planned_offset_min": 0},
			{"stop_place_id": SchoolAStopID, "sequence": 2, "planned_offset_min": 30},
		})
		CreateRouteSchedule(t, helper, routeID, map[string]interface{}{
			"days_of_week": EveryDayMask, "start_time": "07:00", "valid_from": "2026-01-01",
		})
	}
	today := dayOffset(t, "UTC", 0)
	materialiseRoute(t, helper, routeID, today, today)
	tripID, _ := tripOn(t, helper, routeID, today, "07:00")
	SetTripRouteClient(t, helper, tripID, organizationID)
	MoveTripStart(t, helper, tripID, minutes)
	return tripID
}

// runCancelEmpty is one pass of the job over the test tenant, without a pusher.
func runCancelEmpty(t *testing.T, helper *testhelpers.ApiTestHelper) {
	t.Helper()
	for _, r := range trackingJobs.RunTripsCancelEmptyPass(context.Background(), helper.DB(), nil, []coreJobs.Tenant{testTenant()}) {
		if r.Err != nil {
			t.Fatalf("cancel empty trips: %v", r.Err)
		}
	}
}

// TestCancelEmptyTrips_InsideTheLead - D1: a riderless trip 20 min out on a client with a 30 min lead
// cancels, overridden, with one cancel event saying why; a second pass writes no second event.
func TestCancelEmptyTrips_InsideTheLead(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	tripID := clientTrip(t, helper, createLeadOrganization(t, helper, 30), 20, false)

	runCancelEmpty(t, helper)
	runCancelEmpty(t, helper)

	status, overridden, reasons := TripCancelState(t, helper, tripID)
	assert.Equal(t, "cancelled", status)
	assert.True(t, overridden, "the materialiser must never revive it")
	assert.Equal(t, []string{"no_riders"}, reasons)
}

// TestCancelEmptyTrips_LeavesTheRest - D1: a trip with a rider, one still outside the lead and one
// on a client that never auto-cancels all stay planned.
func TestCancelEmptyTrips_LeavesTheRest(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	lead := createLeadOrganization(t, helper, 30)
	ridden := clientTrip(t, helper, lead, 20, true)
	early := clientTrip(t, helper, lead, 40, false)
	never := clientTrip(t, helper, createLeadOrganization(t, helper, 0), 20, false)

	runCancelEmpty(t, helper)

	for name, tripID := range map[string]string{"with a rider": ridden, "outside the lead": early, "lead 0": never} {
		status, _, reasons := TripCancelState(t, helper, tripID)
		assert.Equal(t, "planned", status, name)
		assert.Empty(t, reasons, name)
	}
}

// TestOrganization_UpdatesEmptyTripLead - the client form saves its lead and reads it back.
func TestOrganization_UpdatesEmptyTripLead(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	organizationID := createLeadOrganization(t, helper, 30)

	w := helper.DoRequest("PATCH", "/tracking/organizations/"+organizationID, map[string]interface{}{"empty_trip_cancel_min": 15}, map[string]string{})
	if !assert.Equal(t, http.StatusOK, w.Code, w.Body.String()) {
		return
	}
	w = helper.DoRequest("GET", "/tracking/organizations/"+organizationID, nil, map[string]string{})
	assert.Equal(t, float64(15), ParseResponse(t, w.Body.Bytes())["empty_trip_cancel_min"])

	w = helper.DoRequest("PATCH", "/tracking/organizations/"+organizationID, map[string]interface{}{"empty_trip_cancel_min": 2000}, map[string]string{})
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
}
