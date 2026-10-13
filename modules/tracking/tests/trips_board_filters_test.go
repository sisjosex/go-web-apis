//go:build integration
// +build integration

package tracking_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

// ============================================================================
// BOARD RANGE AND LATENESS (TRACK-069 D5)
// ============================================================================

// TestListTrips_LateOnly - late_min=5 lists only the trip whose next stop is 6 min overdue, not the
// one whose next stop is still ahead.
func TestListTrips_LateOnly(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	routeID, _ := createTripRoute(t, helper, "07:00", "08:00")
	today := dayOffset(t, "UTC", 0)
	materialiseRoute(t, helper, routeID, today, today)
	late, _ := tripOn(t, helper, routeID, today, "07:00")
	onTime, _ := tripOn(t, helper, routeID, today, "08:00")
	SetTripStopTimes(t, helper, late, -6)
	SetTripStopTimes(t, helper, onTime, 10)

	board := listTrips(t, helper, routeID, today, "&late_min=5")

	if assert.Equal(t, int64(1), board.TotalCount) && assert.Len(t, board.Trips, 1) {
		assert.Equal(t, late, board.Trips[0].ID.String())
		assert.GreaterOrEqual(t, *board.Trips[0].DelaySeconds, int32(5*60))
	}
	assert.Equal(t, int64(2), listTrips(t, helper, routeID, today, "").TotalCount, "without the filter both show")
}

// TestListTrips_DateRange - date..date_to lists both days' trips; a range over 31 days, ending before
// it starts, or without a start is a 400.
func TestListTrips_DateRange(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	routeID, _ := createTripRoute(t, helper, "07:00")
	today, tomorrow := dayOffset(t, "UTC", 0), dayOffset(t, "UTC", 1)
	materialiseRoute(t, helper, routeID, today, tomorrow)

	assert.Equal(t, int64(1), listTrips(t, helper, routeID, today, "").TotalCount)
	assert.Equal(t, int64(2), listTrips(t, helper, routeID, today, "&date_to="+tomorrow).TotalCount)

	for name, query := range map[string]string{
		"over 31 days":      "date=" + today + "&date_to=" + dayOffset(t, "UTC", 40),
		"ends before start": "date=" + tomorrow + "&date_to=" + today,
		"no start":          "date_to=" + tomorrow,
	} {
		w := helper.DoRequest("GET", "/tracking/trips?"+query+"&route_id="+routeID, nil, map[string]string{})
		assert.Equal(t, http.StatusBadRequest, w.Code, name+": "+w.Body.String())
	}
}
