//go:build integration
// +build integration

package tracking_test

import (
	"encoding/json"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"josex/web/modules/core/testhelpers"
	"josex/web/modules/tracking/models"
)

// ============================================================================
// THE TRAVELLED LINE IN THE LIVE READ (TRACK-052)
// ============================================================================

func tripLive(t *testing.T, helper *testhelpers.ApiTestHelper, tripID, query string) models.TripLive {
	t.Helper()
	w := helper.DoRequest("GET", "/tracking/trips/"+tripID+"/live"+query, nil, map[string]string{})
	if w.Code != http.StatusOK {
		t.Fatalf("trip live: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var live models.TripLive
	decodeBody(t, w, &live)
	return live
}

// TestTripLive_TrailSinceCursor - D1/D4: three fixes of a running trip come back as the trail; after the
// second one's time only the third; an imprecise fix (> 50 m) is never part of it.
func TestTripLive_TrailSinceCursor(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	trip := driverTrip(t, helper, 1, true)
	at := time.Now().UTC().Add(-10 * time.Minute).Truncate(time.Second)
	ingest(t, helper, pointsJSON(t, trip.VehicleID.String(), at,
		[2]float64{-17.3935, -66.1570}, [2]float64{-17.3940, -66.1560}, [2]float64{-17.3950, -66.1550}))
	vague := 80.0
	raw, err := json.Marshal([]models.StoredPoint{{
		VehicleID: *trip.VehicleID, RecordedAt: at.Add(5 * time.Second), Lat: -17.40, Lng: -66.10, Accuracy: &vague,
	}})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	ingest(t, helper, raw)

	live := tripLive(t, helper, trip.ID.String(), "")
	if assert.NotNil(t, live.Trail) {
		assert.Equal(t, 3, live.Trail.Points)
		assert.NotEmpty(t, live.Trail.Polyline6)
	}

	since := url.QueryEscape(at.Add(time.Second).Format(time.RFC3339Nano))
	live = tripLive(t, helper, trip.ID.String(), "?trail_since="+since)
	if assert.NotNil(t, live.Trail) {
		assert.Equal(t, 1, live.Trail.Points)
		if assert.NotNil(t, live.Trail.LastAt) {
			assert.True(t, live.Trail.LastAt.Equal(at.Add(2*time.Second)))
		}
	}
}

// TestTripLive_NoTrailBeforeStart - a planned trip answers no trail at all.
func TestTripLive_NoTrailBeforeStart(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	trip := driverTrip(t, helper, 1, false)

	assert.Nil(t, tripLive(t, helper, trip.ID.String(), "").Trail)
}
