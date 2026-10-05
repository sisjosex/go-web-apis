//go:build integration
// +build integration

package tracking_test

import (
	"context"
	"math/rand"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"josex/web/modules/core/testhelpers"
	"josex/web/modules/tracking/models"
)

// ============================================================================
// ABSENCES, HOME LOCATION, SUGGESTIONS (TRACK-022)
// ============================================================================

// insertAbsence writes an absence straight into the table, as the step 1 schema holds it.
func insertAbsence(t *testing.T, helper *testhelpers.ApiTestHelper, riderID, from, to string) {
	t.Helper()
	if _, err := helper.DB().Execute(context.Background(), `
		INSERT INTO tracking.rider_absences (tenant_id, rider_id, date_from, date_to, reported_via)
		VALUES ($1, $2, $3::DATE, $4::DATE, 'web')`, TestTenantID, riderID, from, to); err != nil {
		t.Fatalf("insert absence: %v", err)
	}
}

// ---- step 1: schema + materialiser ----

// TestMaterialise_SkipsAbsentRider - an absence tomorrow leaves the rider off tomorrow's trip and
// on the day after's.
func TestMaterialise_SkipsAbsentRider(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	routeID, riderID := createTripRoute(t, helper, "07:00")
	tomorrow, after := dayOffset(t, "UTC", 1), dayOffset(t, "UTC", 2)
	insertAbsence(t, helper, riderID, tomorrow, tomorrow)
	materialiseRoute(t, helper, routeID, tomorrow, after)

	tomorrowTrip, _ := tripOn(t, helper, routeID, tomorrow, "07:00")
	afterTrip, _ := tripOn(t, helper, routeID, after, "07:00")
	assert.Empty(t, taskStatuses(t, helper, tomorrowTrip), "absent tomorrow")
	assert.Equal(t, []string{"dropoff:pending", "pickup:pending"}, taskStatuses(t, helper, afterTrip))
}

// TestRider_HomeLocationAndNotes - PATCH sets the home point and notes, and GET echoes them.
func TestRider_HomeLocationAndNotes(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	riderID := CreateTestRider(t, helper)

	w := helper.DoRequest("PATCH", "/tracking/riders/"+riderID, map[string]interface{}{
		"home_latitude": -17.3935, "home_longitude": -66.157, "notes": "Gate on the left",
	}, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var rider models.Rider
	decodeBody(t, w, &rider)
	if assert.NotNil(t, rider.HomeLatitude) && assert.NotNil(t, rider.HomeLongitude) {
		assert.InDelta(t, -17.3935, *rider.HomeLatitude, 1e-6)
		assert.InDelta(t, -66.157, *rider.HomeLongitude, 1e-6)
	}
	if assert.NotNil(t, rider.Notes) {
		assert.Equal(t, "Gate on the left", *rider.Notes)
	}

	w = helper.DoRequest("GET", "/tracking/riders/"+riderID, nil, map[string]string{})
	decodeBody(t, w, &rider)
	assert.NotNil(t, rider.HomeLatitude)

	w = helper.DoRequest("PATCH", "/tracking/riders/"+riderID, map[string]interface{}{"home_latitude": 10}, map[string]string{})
	assert.Equal(t, http.StatusBadRequest, w.Code, "a latitude without a longitude")
}

// TestOrganization_AbsenceCutoff - 60 by default, editable by PATCH (D1).
func TestOrganization_AbsenceCutoff(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	w := helper.DoRequest("GET", "/tracking/organizations/"+MainSchoolID, nil, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var org models.Organization
	decodeBody(t, w, &org)
	assert.Equal(t, int32(60), org.AbsenceCutoffMin)

	w = helper.DoRequest("PATCH", "/tracking/organizations/"+EmptyEmployerID, map[string]interface{}{"absence_cutoff_min": 90}, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	decodeBody(t, w, &org)
	assert.Equal(t, int32(90), org.AbsenceCutoffMin)
	assert.True(t, strings.Contains(w.Body.String(), `"absence_cutoff_min":90`))
}

// ---- step 2: absences ----

func postAbsence(t *testing.T, helper *testhelpers.ApiTestHelper, riderID string, body map[string]interface{}, want int) models.CreateRiderAbsenceResponse {
	t.Helper()
	w := helper.DoRequest("POST", "/tracking/riders/"+riderID+"/absences", body, map[string]string{})
	if w.Code != want {
		t.Fatalf("report absence: expected %d, got %d: %s", want, w.Code, w.Body.String())
	}
	var created models.CreateRiderAbsenceResponse
	if want == http.StatusCreated {
		decodeBody(t, w, &created)
	}
	return created
}

// TestAbsence_CancelsInProgressTrip - D2: an operator's absence today cancels the rider's pending
// tasks on the started trip at once, with one trip.changed; the same days again are an overlap.
func TestAbsence_CancelsInProgressTrip(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	_, riderID, tripID := todaysTrip(t, helper)
	tripRequest(t, helper, "POST", "/tracking/trips/"+tripID+"/start", nil, http.StatusOK)
	today := dayOffset(t, "UTC", 0)

	created := postAbsence(t, helper, riderID, map[string]interface{}{
		"date_from": today, "date_to": today, "reason": "Sick",
	}, http.StatusCreated)
	assert.Equal(t, int32(2), created.CancelledTasks)
	if assert.NotNil(t, created.Absence) {
		assert.Equal(t, "web", created.Absence.ReportedVia)
		assert.Nil(t, created.Absence.Direction, "both directions")
		assert.NotNil(t, created.Absence.ReportedByName)
	}
	assert.Equal(t, []string{"dropoff:cancelled", "pickup:cancelled"}, taskStatuses(t, helper, tripID))
	assert.Equal(t, 1, tripChangedRows(t, helper, tripID, "absence"))

	w := helper.DoRequest("POST", "/tracking/riders/"+riderID+"/absences", map[string]interface{}{
		"date_from": today, "date_to": today, "direction": "outbound",
	}, map[string]string{})
	assert.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "tracking.absence.overlap")

	w = helper.DoRequest("GET", "/tracking/riders/"+riderID+"/absences?from="+today+"&to="+today, nil, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var list []models.RiderAbsence
	decodeBody(t, w, &list)
	assert.Len(t, list, 1)
}

// guardianRoute puts the portal guardian's rider (John) on an inbound route that runs every day at
// start, picked up at the first stop, and materialises today. Inbound, so it meets none of John's
// seeded outbound assignment (TRACK-009 D4).
func guardianRoute(t *testing.T, helper *testhelpers.ApiTestHelper, start string) string {
	t.Helper()
	routeID := CreateTestRoute(t, helper)
	SetRouteDirection(t, helper, routeID, "inbound")
	PublishRouteVersion(t, helper, routeID, "2026-01-01", []map[string]interface{}{
		{"stop_place_id": CentralStationID, "sequence": 1, "planned_offset_min": 0},
		{"stop_place_id": SchoolAStopID, "sequence": 2, "planned_offset_min": 30},
	})
	CreateRouteSchedule(t, helper, routeID, map[string]interface{}{
		"days_of_week": EveryDayMask, "start_time": start, "valid_from": "2026-01-01",
	})
	body := AssignmentBody(TestRiderJohnID, routeID, EveryDayMask)
	body["pickup_stop_place_id"] = CentralStationID
	body["dropoff_stop_place_id"] = SchoolAStopID
	assignment := CreateAssignment(t, helper, body)
	t.Cleanup(func() {
		helper.DoRequest("DELETE", "/tracking/assignments/"+assignment["id"].(string), nil, map[string]string{})
	})
	today := dayOffset(t, "UTC", 0)
	materialiseRoute(t, helper, routeID, today, today)
	return routeID
}

// TestAbsence_GuardianCutoff - D1/D3: the guardian reports from the portal for their own rider only;
// a pickup 10 minutes away is inside the 60-minute cut-off, a day with no trip is not.
func TestAbsence_GuardianCutoff(t *testing.T) {
	operator := SetupTrackingTest(t)
	defer operator.Close()
	guardianRoute(t, operator, time.Now().UTC().Add(10*time.Minute).Format("15:04"))

	portal := SetupPortalTest(t)
	defer portal.Close()
	today := dayOffset(t, "UTC", 0)

	w := portal.DoRequest("POST", "/tracking/riders/"+TestRiderJohnID+"/absences", map[string]interface{}{
		"date_from": today, "date_to": today, "direction": "inbound",
	}, map[string]string{})
	assert.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "tracking.absence.cutoff")

	w = portal.DoRequest("POST", "/tracking/riders/"+TestRiderJaneID+"/absences", map[string]interface{}{
		"date_from": today, "date_to": today,
	}, map[string]string{})
	assert.Equal(t, http.StatusNotFound, w.Code, "another guardian's rider: %s", w.Body.String())

	later := dayOffset(t, "UTC", 40)
	created := postAbsence(t, portal, TestRiderJohnID, map[string]interface{}{
		"date_from": later, "date_to": later, "reason": "Trip",
	}, http.StatusCreated)
	if assert.NotNil(t, created.Absence) {
		assert.Equal(t, "portal", created.Absence.ReportedVia)
		w = portal.DoRequest("DELETE", "/tracking/riders/"+TestRiderJohnID+"/absences/"+created.Absence.ID.String(), nil, map[string]string{})
		assert.Equal(t, http.StatusNoContent, w.Code, w.Body.String())
	}
}

// TestAbsence_DeleteReseats - deleting writes route.changed only: tomorrow's planned trip, emptied
// by the absence, seats the rider again on the next materialiser run.
func TestAbsence_DeleteReseats(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	routeID, riderID := createTripRoute(t, helper, "07:00")
	tomorrow := dayOffset(t, "UTC", 1)
	materialiseRoute(t, helper, routeID, tomorrow, tomorrow)
	tripID, _ := tripOn(t, helper, routeID, tomorrow, "07:00")

	created := postAbsence(t, helper, riderID, map[string]interface{}{"date_from": tomorrow, "date_to": tomorrow}, http.StatusCreated)
	assert.Equal(t, int32(2), created.CancelledTasks)
	materialiseRoute(t, helper, routeID, tomorrow, tomorrow)
	assert.Empty(t, taskStatuses(t, helper, tripID), "the materialiser drops the absent rider")

	w := helper.DoRequest("DELETE", "/tracking/riders/"+riderID+"/absences/"+created.Absence.ID.String(), nil, map[string]string{})
	assert.Equal(t, http.StatusNoContent, w.Code, w.Body.String())
	assert.Empty(t, taskStatuses(t, helper, tripID), "nothing moves before the run")
	materialiseRoute(t, helper, routeID, tomorrow, tomorrow)
	assert.Equal(t, []string{"dropoff:pending", "pickup:pending"}, taskStatuses(t, helper, tripID))

	w = helper.DoRequest("DELETE", "/tracking/riders/"+riderID+"/absences/"+created.Absence.ID.String(), nil, map[string]string{})
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
}

// ---- step 3: suggestions ----

// TestSuggestions_NearHome - a rider 300 m from a stop gets its route first, with free seats the
// vehicle's capacity less the riders assigned; 3 km away gets nothing; no home is a 409.
func TestSuggestions_NearHome(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	// A place of its own, so no stop another test left behind is nearer.
	lat, lng := -50-rand.Float64()*10, -60-rand.Float64()*10
	stop := ValidStopPlaceBody()
	stop["latitude"], stop["longitude"] = lat, lng
	w := helper.DoRequest("POST", "/tracking/stop-places", stop, map[string]string{})
	if w.Code != http.StatusCreated {
		t.Fatalf("create stop place: %d %s", w.Code, w.Body.String())
	}
	stopID := ExtractID(t, ParseResponse(t, w.Body.Bytes()))
	routeID := CreateTestRoute(t, helper)
	PublishRouteVersion(t, helper, routeID, "2026-01-01", []map[string]interface{}{
		{"stop_place_id": stopID, "sequence": 1, "planned_offset_min": 0},
		{"stop_place_id": SchoolAStopID, "sequence": 2, "planned_offset_min": 30},
	})
	if _, err := helper.DB().Execute(context.Background(),
		`UPDATE tracking.routes SET vehicle_id = $2 WHERE id = $1`, routeID, TestBusID); err != nil {
		t.Fatalf("set vehicle: %v", err)
	}
	CreateAssignment(t, helper, AssignmentBody(CreateHomedRider(t, helper), routeID, WeekdaysMask))

	riderID := CreateTestRider(t, helper)
	w = helper.DoRequest("GET", "/tracking/riders/"+riderID+"/suggestions?direction=outbound", nil, map[string]string{})
	assert.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "tracking.rider.no-home-location")

	suggest := func(homeLat float64) models.SuggestRiderStopsResponse {
		t.Helper()
		w := helper.DoRequest("PATCH", "/tracking/riders/"+riderID, map[string]interface{}{
			"home_latitude": homeLat, "home_longitude": lng,
		}, map[string]string{})
		assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
		w = helper.DoRequest("GET", "/tracking/riders/"+riderID+"/suggestions?direction=outbound&days=31", nil, map[string]string{})
		assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
		var res models.SuggestRiderStopsResponse
		decodeBody(t, w, &res)
		return res
	}

	near := suggest(lat + 0.0027) // ~300 m north
	if assert.NotEmpty(t, near.Suggestions) {
		first := near.Suggestions[0]
		assert.Equal(t, routeID, first.RouteID.String())
		assert.Equal(t, stopID, first.StopPlaceID.String())
		assert.InDelta(t, 300, first.WalkDistanceM, 15)
		assert.Equal(t, int32(1), first.Assigned)
		if assert.NotNil(t, first.Capacity) && assert.NotNil(t, first.FreeSeats) {
			assert.Equal(t, *first.Capacity-1, *first.FreeSeats)
		}
	}
	assert.Empty(t, suggest(lat+0.027).Suggestions, "~3 km is past the 800 m default")
}
