//go:build integration
// +build integration

package tracking_test

import (
	"net/http"
	"testing"
	"time"

	"josex/web/modules/tracking/models"

	"github.com/stretchr/testify/assert"
)

// ============================================================================
// OPERATION SCREENS' FIGURES (TRACK-073)
// ============================================================================

// TestListTrips_CountsAndSearch - the chips count every status whatever the status filter keeps, an
// empty page still answers them, the search finds a trip by its plate, and a day without trips counts 0.
func TestListTrips_CountsAndSearch(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	tripID, _ := liveTrip(t, helper)
	routeID := tripRequest(t, helper, "GET", "/tracking/trips/"+tripID, nil, http.StatusOK).RouteID.String()
	today := dayOffset(t, "UTC", 0)

	board := listTrips(t, helper, routeID, today, "")
	if assert.Len(t, board.Trips, 1) {
		assert.NotNil(t, board.Trips[0].PlannedEnd, "the last stop's planned time")
		assert.Equal(t, int32(1), board.Trips[0].RidersTotal, "a rider counts once, not per task")
		assert.Equal(t, int64(1), board.Counts.All)
		assert.Equal(t, int64(1), board.Counts.InProgress)
	}

	completed := listTrips(t, helper, routeID, today, "&status=completed")
	assert.Empty(t, completed.Trips)
	assert.Equal(t, int64(0), completed.TotalCount)
	assert.Equal(t, int64(1), completed.Counts.InProgress, "the status filter leaves the counts alone")

	plate := *board.Trips[0].LicensePlate
	found := listTrips(t, helper, routeID, today, "&search="+plate)
	if assert.Len(t, found.Trips, 1) {
		assert.Equal(t, tripID, found.Trips[0].ID.String())
	}
	assert.Empty(t, listTrips(t, helper, routeID, today, "&search=no-such-plate-xyz").Trips)

	empty := listTrips(t, helper, routeID, dayOffset(t, "UTC", 20), "")
	assert.Empty(t, empty.Trips)
	assert.Equal(t, int64(0), empty.Counts.All)
}

// TestLiveFleet_RowFigures - a vehicle on a trip in progress carries its driver, its client and the
// trip's riders expected and done; its delay is read from the next pending stop.
func TestLiveFleet_RowFigures(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	tripID, vehicleID := liveTrip(t, helper)
	SetTripRouteClient(t, helper, tripID, MainSchoolID)
	ingest(t, helper, pointsJSON(t, vehicleID, time.Now().UTC(), [2]float64{-17.39, -66.15}))

	w := helper.DoRequest("GET", "/tracking/live/fleet", nil, map[string]string{})
	expectStatus(t, w, http.StatusOK)
	var res models.LiveFleetResponse
	decodeBody(t, w, &res)
	var v *models.LiveVehicle
	for i := range res.Vehicles {
		if res.Vehicles[i].VehicleID.String() == vehicleID {
			v = &res.Vehicles[i]
		}
	}
	if assert.NotNil(t, v) {
		assert.NotNil(t, v.DriverName)
		assert.NotNil(t, v.OrganizationName)
		if assert.NotNil(t, v.RidersTotal) {
			assert.Equal(t, int32(1), *v.RidersTotal, "one rider assigned, one pickup")
		}
		assert.NotNil(t, v.DelaySeconds)
		assert.Nil(t, v.ArrivedAt, "only without a trip in progress")
	}
}
