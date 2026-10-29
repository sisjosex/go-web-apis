//go:build integration
// +build integration

package tracking_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"josex/web/config"
	coreJobs "josex/web/modules/core/jobs"
	"josex/web/modules/core/testhelpers"
	geoRouting "josex/web/modules/geo/services/routing"
	trackingJobs "josex/web/modules/tracking/jobs"
	"josex/web/modules/tracking/models"
)

// ============================================================================
// THE TWO DETECTORS: DEVICE SILENT, OFF ROUTE (TRACK-071 D1, D2)
// ============================================================================

func silentPass(t *testing.T, helper *testhelpers.ApiTestHelper) {
	t.Helper()
	for _, r := range trackingJobs.RunDeviceSilentPass(context.Background(), helper.DB(), []coreJobs.Tenant{testTenant()}) {
		if r.Err != nil {
			t.Fatalf("device silent pass: %v", r.Err)
		}
	}
}

// TestDeviceSilent_SweepAndResolve - a trip running 6 min whose last fix is 5½ min old gets one
// device_silent alert; a second pass adds none; one point resolves it; a trip running 2 min gets none.
func TestDeviceSilent_SweepAndResolve(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	ResetAlertSettings(t, helper)
	tripID, vehicleID := liveTrip(t, helper)
	BackdateTripStart(t, helper, tripID, 6*time.Minute)
	ingestPriority(t, helper, sourcePoint(t, vehicleID, models.PointSourcePhone, time.Now().UTC().Add(-330*time.Second), -17.39, -66.15))

	silentPass(t, helper)
	statuses, _, _ := TripAlerts(t, helper, tripID, "device_silent")
	assert.Equal(t, []string{"active"}, statuses)
	silentPass(t, helper)
	statuses, _, _ = TripAlerts(t, helper, tripID, "device_silent")
	assert.Equal(t, []string{"active"}, statuses, "a second pass adds none")

	ingestPriority(t, helper, sourcePoint(t, vehicleID, models.PointSourcePhone, time.Now().UTC(), -17.391, -66.151))
	statuses, _, _ = TripAlerts(t, helper, tripID, "device_silent")
	assert.Equal(t, []string{"resolved"}, statuses, "the next position resolves it")

	fresh, _ := liveTrip(t, helper)
	BackdateTripStart(t, helper, fresh, 2*time.Minute)
	silentPass(t, helper)
	statuses, _, _ = TripAlerts(t, helper, fresh, "device_silent")
	assert.Empty(t, statuses, "running for less than the threshold")
}

// TestOffRoute_Hysteresis - a version planned by the router stores planned_line; 2 fixes far off it
// raise nothing, the 3rd raises one off_route alert at that point, 3 back on resolve it; a trip whose
// version has no street line never raises.
func TestOffRoute_Hysteresis(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	ResetAlertSettings(t, helper)
	routeID, _ := cochabambaRoute(t, helper)
	planner := trackingJobs.PathPlanner{Router: geoRouting.NewRouter(config.ModularAppConfig.Geo), FallbackKmh: 25}
	if err := planner.ApplyRoutePaths(context.Background(), helper.DB(), testTenant(), uuid.MustParse(routeID), nil); err != nil {
		t.Fatalf("plan the route (is Valhalla up?): %v", err)
	}
	assert.True(t, RouteVersionHasLine(t, helper, routeID))
	tripID, vehicleID := startCochabambaTrip(t, helper, routeID)

	at := time.Now().UTC()
	fix := func(lat, lng float64) {
		at = at.Add(5 * time.Second)
		ingestPriority(t, helper, sourcePoint(t, vehicleID, models.PointSourcePhone, at, lat, lng))
	}
	const offLat, offLng = -17.4080, -66.1515 // about 1.5 km south of the line
	fix(offLat, offLng)
	fix(offLat, offLng+0.0005)
	statuses, _, _ := TripAlerts(t, helper, tripID, "off_route")
	assert.Empty(t, statuses, "two fixes off are not yet a detour")
	fix(offLat, offLng+0.001)
	statuses, lat, lng := TripAlerts(t, helper, tripID, "off_route")
	if assert.Equal(t, []string{"active"}, statuses) && assert.NotNil(t, lat) {
		assert.InDelta(t, offLat, *lat, 1e-6)
		assert.InDelta(t, offLng+0.001, *lng, 1e-6)
	}
	for i := 0; i < 3; i++ {
		fix(-17.3950, -66.1550)
	}
	statuses, _, _ = TripAlerts(t, helper, tripID, "off_route")
	assert.Equal(t, []string{"resolved"}, statuses, "three fixes back on the line resolve it")

	plain, plainVehicle := liveTrip(t, helper)
	for i := 0; i < 4; i++ {
		at = at.Add(5 * time.Second)
		ingestPriority(t, helper, sourcePoint(t, plainVehicle, models.PointSourcePhone, at, offLat, offLng))
	}
	statuses, _, _ = TripAlerts(t, helper, plain, "off_route")
	assert.Empty(t, statuses, "no street line: never off route")
}
