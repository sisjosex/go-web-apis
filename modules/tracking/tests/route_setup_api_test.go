//go:build integration
// +build integration

package tracking_test

import (
	"math/rand"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"josex/web/modules/core/testhelpers"
	"josex/web/modules/tracking/models"
)

// ============================================================================
// ROUTE SETUP FROM ONE SCREEN (TRACK-034)
// ============================================================================

// createSetupVehicle creates a bus of its own, so no seeded route shares its schedule.
func createSetupVehicle(t *testing.T, helper *testhelpers.ApiTestHelper) string {
	t.Helper()
	w := helper.DoRequest("POST", "/tracking/vehicles", ValidVehicleDto(), map[string]string{})
	if w.Code != http.StatusCreated {
		t.Fatalf("create vehicle: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	return ExtractID(t, ParseResponse(t, w.Body.Bytes()))
}

// TestRouteVersion_InlineStops - a stop added from the map reuses the tenant's place within 30 m and
// creates the one that is new, in the request that publishes the list (D3).
func TestRouteVersion_InlineStops(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	lat, lng := StopPlacePoint(t, helper, CentralStationID)
	before := CountStopPlaces(t, helper)

	routeID := CreateTestRoute(t, helper)
	PublishRouteVersion(t, helper, routeID, "2026-01-01", []map[string]interface{}{
		// ~10 m north of Central Station: the same stop under another click.
		{"name": "Clicked near Central", "latitude": lat + 0.00009, "longitude": lng, "sequence": 1},
		// 1-10 km away, somewhere no earlier run left a place: a new one.
		{"name": "Brand new corner", "address": "Av. Test 1", "latitude": lat + 0.009 + rand.Float64()*0.08, "longitude": lng, "sequence": 2},
	})

	stops := ListRouteStops(t, helper, routeID, "?date=2026-02-01")
	if assert.Len(t, stops, 2) {
		assert.Equal(t, CentralStationID, stops[0]["stop_place_id"], "reused within 30 m")
		assert.NotEqual(t, CentralStationID, stops[1]["stop_place_id"])
		assert.Equal(t, "Brand new corner", stops[1]["stop_name"])
	}
	assert.Equal(t, before+1, CountStopPlaces(t, helper), "only the far stop is a new place")

	w := helper.DoRequest("POST", "/tracking/routes/"+routeID+"/versions", map[string]interface{}{
		"effective_from": "2026-03-01",
		"stops":          []map[string]interface{}{{"latitude": lat, "longitude": lng, "sequence": 1}},
	}, map[string]string{})
	assert.Equal(t, http.StatusBadRequest, w.Code, "a new stop without its name: "+w.Body.String())
}

// TestVehicleConflicts - a route assigned to a vehicle that already runs another at the same hours
// answers that run; a route at another hour answers none (D2).
func TestVehicleConflicts(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	vehicleID := createSetupVehicle(t, helper)

	running, _ := createTripRoute(t, helper, "07:00")
	SetRouteVehicle(t, helper, running, vehicleID)
	clashing, _ := createTripRoute(t, helper, "07:30")
	later, _ := createTripRoute(t, helper, "15:00")

	conflicts := func(routeID string) []*models.VehicleScheduleConflict {
		t.Helper()
		w := helper.DoRequest("GET", "/tracking/vehicles/"+vehicleID+"/conflicts?route_id="+routeID, nil, map[string]string{})
		if w.Code != http.StatusOK {
			t.Fatalf("conflicts: expected 200, got %d: %s", w.Code, w.Body.String())
		}
		var body models.VehicleConflictsResponse
		decodeBody(t, w, &body)
		return body.Conflicts
	}

	got := conflicts(clashing)
	if assert.Len(t, got, 1) {
		assert.Equal(t, running, got[0].RouteID.String())
		assert.Equal(t, "vehicle", got[0].Reason)
		assert.Equal(t, "07:00", got[0].StartTime)
		assert.Equal(t, int16(EveryDayMask), got[0].Days)
	}
	assert.Empty(t, conflicts(later))
}

// TestVehicleDriver_TripTakesVehicleDriver - a route without its own driver runs with its vehicle's
// usual driver, and the vehicle reads it back (D1).
func TestVehicleDriver_TripTakesVehicleDriver(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	vehicleID := createSetupVehicle(t, helper)
	driverID := CreateTestDriver(t, helper)

	w := helper.DoRequest("PUT", "/tracking/vehicles/"+vehicleID+"/driver", map[string]interface{}{"driver_id": driverID}, map[string]string{})
	if !assert.Equal(t, http.StatusOK, w.Code, w.Body.String()) {
		return
	}
	var vehicle models.Vehicle
	decodeBody(t, w, &vehicle)
	if assert.NotNil(t, vehicle.DefaultDriverID) && assert.NotNil(t, vehicle.DriverName) {
		assert.Equal(t, driverID, vehicle.DefaultDriverID.String())
		assert.Equal(t, "Ana Quispe", *vehicle.DriverName)
	}

	routeID, _ := createTripRoute(t, helper, "07:00")
	SetRouteVehicle(t, helper, routeID, vehicleID)
	today := dayOffset(t, "UTC", 0)
	materialiseRoute(t, helper, routeID, today, today)

	assert.Equal(t, driverID, TripDriverOn(t, helper, routeID, today))

	routes := helper.DoRequest("GET", "/tracking/routes?vehicle_id="+vehicleID, nil, map[string]string{})
	var list models.ListRoutesResponse
	decodeBody(t, routes, &list)
	if assert.Len(t, list.Routes, 1) {
		assert.Equal(t, routeID, list.Routes[0].ID.String())
	}

	other := helper.DoRequest("PUT", "/tracking/vehicles/"+vehicleID+"/driver", map[string]interface{}{"driver_id": "00000000-0000-4000-8000-000000000000"}, map[string]string{})
	assert.Equal(t, http.StatusUnprocessableEntity, other.Code, other.Body.String())
}

// TestRoutePaths_DurationFollowsLine - storing the open-ended version's line writes the route's
// duration from its legs, so nobody types it (D4).
func TestRoutePaths_DurationFollowsLine(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	routeID := CreateTestRoute(t, helper)
	PublishRouteVersion(t, helper, routeID, dayOffset(t, "UTC", 0), cochabambaStops(t, helper))

	assert.NoError(t, applyPaths(t, helper, &stubRouter{}, routeID))
	_, _, path := getPath(helper, routeID, "", "")
	if !assert.NotNil(t, path.Path) {
		return
	}

	w := helper.DoRequest("GET", "/tracking/routes/"+routeID, nil, map[string]string{})
	var route models.Route
	decodeBody(t, w, &route)
	// The stub answers 90 s a leg; no stop dwells.
	want := int32((90*len(path.Path.Legs) + 59) / 60)
	if assert.NotNil(t, route.EstimatedDurationMinutes) {
		assert.Equal(t, want, *route.EstimatedDurationMinutes)
	}
}
