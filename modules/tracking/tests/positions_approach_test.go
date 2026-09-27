//go:build integration
// +build integration

package tracking_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestIngestPositions_ApproachOnce - TRACK-012 step 5: two positions inside the approach radius of the
// next stop, in two batches, stamp it once and write one trip.changed `approach`, which becomes one
// `approaching` row for the rider's guardian; the stop is not arrived.
func TestIngestPositions_ApproachOnce(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	tripID, vehicleID := liveTrip(t, helper)
	now := time.Now().UTC()

	// About 500 m, then 400 m north of Central Station: inside 800 m, outside the 50 m arrival.
	first := ingest(t, helper, pointsJSON(t, vehicleID, now, [2]float64{centralLat + 0.0045, centralLng}))
	ingest(t, helper, pointsJSON(t, vehicleID, now.Add(5*time.Second), [2]float64{centralLat + 0.0036, centralLng}))

	assert.Equal(t, 1, tripChangedRows(t, helper, tripID, "approach"))
	assert.Equal(t, 0, tripChangedRows(t, helper, tripID, "arrive"))
	assert.Equal(t, 1, countRows(t, helper, `SELECT count(*) FROM tracking.trip_stops
		WHERE trip_id = $1 AND sequence = 1 AND approach_notified_at IS NOT NULL AND status = 'pending'`, tripID))

	if !assert.Len(t, first, 1) || !assert.Len(t, first[0].RiderIDs, 1) {
		return
	}
	riderID := first[0].RiderIDs[0].String()
	execSQL(t, helper, `INSERT INTO tracking.rider_contacts (rider_id, relation, name, user_id)
		SELECT $1, 'guardian', 'Portal Guardian', u.id FROM auth.users u WHERE u.email = 'portal@test.local'`, riderID)
	t.Cleanup(func() {
		execSQL(t, helper, `DELETE FROM tracking.rider_contacts WHERE rider_id = $1`, riderID)
		execSQL(t, helper, `DELETE FROM tracking.notifications WHERE rider_id = $1`, riderID)
	})
	var stopName string
	if err := helper.DB().QueryRow(t.Context(), `
		SELECT payload->>'stop_name' FROM tracking.sp_notify_event($1, 'trip.changed', (
			SELECT o.payload FROM tracking.outbox o
			WHERE o.topic = 'trip.changed' AND o.payload->>'trip_id' = $2 AND o.payload->>'type' = 'approach'))`,
		TestTenantID, tripID).Scan(&stopName); err != nil {
		t.Fatalf("notify the approach: %v", err)
	}
	assert.NotEmpty(t, stopName, "the notice names the stop")
	assert.Len(t, noticeRows(t, helper, riderID, "approaching"), 1)
}
