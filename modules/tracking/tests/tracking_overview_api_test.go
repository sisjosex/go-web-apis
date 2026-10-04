//go:build integration
// +build integration

package tracking_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TRACK-041 D2: the tracking home in one read.

// TestTrackingOverview_CountsAndDelayed - a trip started today whose stops were due long ago (later than
// any other fixture's, so it is in the top ten) is counted in progress and named with its minutes.
func TestTrackingOverview_CountsAndDelayed(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	_, _, tripID := todaysTrip(t, helper)
	tripRequest(t, helper, "POST", "/tracking/trips/"+tripID+"/start", nil, http.StatusOK)
	SetTripStopsDueAgo(t, helper, tripID, 20000)

	w := helper.DoRequest("GET", "/tracking/overview?date="+dayOffset(t, "UTC", 0), nil, map[string]string{})
	expectStatus(t, w, http.StatusOK)
	body := ParseResponse(t, w.Body.Bytes())
	assert.GreaterOrEqual(t, body["trips_in_progress"], float64(1))
	assert.Equal(t, true, body["has_routes"])
	var found map[string]interface{}
	for _, row := range body["trips_delayed"].([]interface{}) {
		if row.(map[string]interface{})["trip_id"] == tripID {
			found = row.(map[string]interface{})
		}
	}
	if assert.NotNil(t, found, "the late trip is named") {
		assert.GreaterOrEqual(t, found["delay_minutes"], float64(19999))
	}
}
