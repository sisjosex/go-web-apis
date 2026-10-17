//go:build integration
// +build integration

package tracking_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"josex/web/modules/core/testhelpers"
)

// ============================================================================
// A DAY'S STOP EXCEPTIONS (TRACK-065)
// ============================================================================

// stopExceptionRoute builds a route that runs every day at 07:00 over Central Station (0 min),
// School A (10), School B (20) and a fourth place (30), with one rider boarding at School A and
// getting off at the fourth. It answers the route, the fourth place and the rider.
func stopExceptionRoute(t *testing.T, helper *testhelpers.ApiTestHelper) (string, string, string) {
	t.Helper()
	routeID := CreateTestRoute(t, helper)
	fourth := CreateTestStopPlace(t, helper)
	PublishRouteVersion(t, helper, routeID, "2026-01-01", []map[string]interface{}{
		{"stop_place_id": CentralStationID, "sequence": 1, "planned_offset_min": 0},
		{"stop_place_id": SchoolAStopID, "sequence": 2, "planned_offset_min": 10},
		{"stop_place_id": SchoolBStopID, "sequence": 3, "planned_offset_min": 20},
		{"stop_place_id": fourth, "sequence": 4, "planned_offset_min": 30},
	})
	CreateRouteSchedule(t, helper, routeID, map[string]interface{}{
		"days_of_week": EveryDayMask, "start_time": "07:00", "valid_from": "2026-01-01",
	})
	riderID := CreateTestRider(t, helper)
	body := AssignmentBody(riderID, routeID, EveryDayMask)
	body["pickup_stop_place_id"] = SchoolAStopID
	body["dropoff_stop_place_id"] = fourth
	CreateAssignment(t, helper, body)
	t.Cleanup(func() { DeleteTestRiders(t, helper, riderID) })
	return routeID, fourth, riderID
}

func stopException(date, kind string, payload map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{"date_from": date, "date_to": date, "kind": kind, "payload": payload}
}

// TestStopExceptions_SkipStopMovesThePickup - D3: a skipped stop leaves that day's trip one stop short,
// its rider boards at the next stop, the day after keeps every stop, and taking the exception back
// restores the stop on the next run.
func TestStopExceptions_SkipStopMovesThePickup(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	routeID, fourth, riderID := stopExceptionRoute(t, helper)
	today, tomorrow, after := dayOffset(t, "UTC", 0), dayOffset(t, "UTC", 1), dayOffset(t, "UTC", 2)

	exceptionID := CreateRouteException(t, helper, routeID,
		stopException(tomorrow, "skip_stop", map[string]interface{}{"stop_place_id": SchoolAStopID}))
	materialiseRoute(t, helper, routeID, today, after)

	tripID, _ := tripOn(t, helper, routeID, tomorrow, "07:00")
	places, _ := TripStopPlaces(t, helper, tripID)
	assert.Equal(t, []string{CentralStationID, SchoolBStopID, fourth}, places)
	assert.Equal(t, SchoolBStopID, RiderTaskPlace(t, helper, tripID, riderID, "pickup"), "boards at the next stop")
	assert.Equal(t, fourth, RiderTaskPlace(t, helper, tripID, riderID, "dropoff"))

	nextID, _ := tripOn(t, helper, routeID, after, "07:00")
	nextPlaces, _ := TripStopPlaces(t, helper, nextID)
	assert.Len(t, nextPlaces, 4)

	w := helper.DoRequest("DELETE", "/tracking/routes/"+routeID+"/exceptions/"+exceptionID, nil, map[string]string{})
	assert.Equal(t, http.StatusNoContent, w.Code, w.Body.String())
	materialiseRoute(t, helper, routeID, today, after)
	places, _ = TripStopPlaces(t, helper, tripID)
	assert.Len(t, places, 4)
	assert.Equal(t, SchoolAStopID, RiderTaskPlace(t, helper, tripID, riderID, "pickup"))
}

// TestStopExceptions_AddStopTimedBetweenNeighbours - D3: an extra stop lands after the given one, due
// between its neighbours, and the later stops move back.
func TestStopExceptions_AddStopTimedBetweenNeighbours(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	routeID, fourth, _ := stopExceptionRoute(t, helper)
	extra := CreateTestStopPlace(t, helper)
	tomorrow := dayOffset(t, "UTC", 1)

	CreateRouteException(t, helper, routeID, stopException(tomorrow, "add_stop",
		map[string]interface{}{"stop_place_id": extra, "after_sequence": 2}))
	materialiseRoute(t, helper, routeID, tomorrow, tomorrow)

	tripID, _ := tripOn(t, helper, routeID, tomorrow, "07:00")
	places, due := TripStopPlaces(t, helper, tripID)
	assert.Equal(t, []string{CentralStationID, SchoolAStopID, extra, SchoolBStopID, fourth}, places)
	assert.True(t, due[2].After(due[1]) && due[3].After(due[2]), "due between its neighbours: %v", due)

	preview := PreviewRoute(t, helper, routeID, tomorrow, tomorrow)
	assert.Equal(t, float64(5), preview[0]["stops_count"])
}

// TestStopExceptions_DetourIsTheDaysList - D3: a detour is the day's whole list in the given order;
// the preview counts it and leaves the other days at the version's count.
func TestStopExceptions_DetourIsTheDaysList(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	routeID, fourth, _ := stopExceptionRoute(t, helper)
	tomorrow, after := dayOffset(t, "UTC", 1), dayOffset(t, "UTC", 2)

	CreateRouteException(t, helper, routeID, stopException(tomorrow, "detour",
		map[string]interface{}{"stops": []string{SchoolBStopID, CentralStationID, fourth}}))
	materialiseRoute(t, helper, routeID, tomorrow, tomorrow)

	tripID, _ := tripOn(t, helper, routeID, tomorrow, "07:00")
	places, _ := TripStopPlaces(t, helper, tripID)
	assert.Equal(t, []string{SchoolBStopID, CentralStationID, fourth}, places)

	preview := PreviewRoute(t, helper, routeID, tomorrow, after)
	assert.Equal(t, float64(3), preview[0]["stops_count"])
	assert.Equal(t, float64(4), preview[1]["stops_count"])
}

// TestStopExceptions_RunningTripKeepsItsStops - D1: a trip already on the road keeps the stops it set
// out with; the exception is for the trips that have not left.
func TestStopExceptions_RunningTripKeepsItsStops(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	trip := driverTrip(t, helper, 1, true)
	today := dayOffset(t, "UTC", 0)

	CreateRouteException(t, helper, trip.RouteID.String(),
		stopException(today, "skip_stop", map[string]interface{}{"stop_place_id": SchoolAStopID}))
	materialiseRoute(t, helper, trip.RouteID.String(), today, today)

	places, _ := TripStopPlaces(t, helper, trip.ID.String())
	assert.Equal(t, []string{CentralStationID, SchoolAStopID}, places)
}

// TestStopExceptions_DetourNeedsTwoOfTheTenantsPlaces - D3: a detour of one stop, of an unknown place
// or of something that is not an id is refused as a bad payload.
func TestStopExceptions_DetourNeedsTwoOfTheTenantsPlaces(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	routeID := CreateTestRoute(t, helper)

	for _, stops := range []interface{}{
		[]string{CentralStationID},
		[]string{CentralStationID, "99999999-9999-9999-9999-999999999999"},
		[]interface{}{CentralStationID, 7},
	} {
		w := helper.DoRequest("POST", "/tracking/routes/"+routeID+"/exceptions",
			stopException("2026-11-20", "detour", map[string]interface{}{"stops": stops}), map[string]string{})
		assert.Equal(t, http.StatusBadRequest, w.Code, "%v: %s", stops, w.Body.String())
		assert.Contains(t, w.Body.String(), "tracking.exception.payload")
	}
}
