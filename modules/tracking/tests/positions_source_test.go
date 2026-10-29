//go:build integration
// +build integration

package tracking_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"josex/web/modules/core/testhelpers"
	"josex/web/modules/tracking/models"
	trackingRepos "josex/web/modules/tracking/repositories"
)

// ============================================================================
// PHONE AND DEVICE (TRACK-068 D2, D3)
// ============================================================================

// sourcePoint is one point of vehicleID at at, sent by source, as sp_ingest_positions takes it.
func sourcePoint(t *testing.T, vehicleID, source string, at time.Time, lat, lng float64) []byte {
	t.Helper()
	raw, err := json.Marshal([]models.StoredPoint{{VehicleID: uuid.MustParse(vehicleID), RecordedAt: at, Lat: lat, Lng: lng, Source: source}})
	if err != nil {
		t.Fatalf("marshal point: %v", err)
	}
	return raw
}

// ingestPriority is ingest with the 90 s phone window the server runs with.
func ingestPriority(t *testing.T, helper *testhelpers.ApiTestHelper, points []byte) {
	t.Helper()
	radii := models.IngestRadii{ArrivalM: 50, ApproachM: 800, PhonePriorityS: 90}
	if _, err := trackingRepos.NewTrackingRepository(helper.DB()).IngestPositions(context.Background(), uuid.MustParse(TestTenantID), points, radii); err != nil {
		t.Fatalf("sp_ingest_positions: %v", err)
	}
}

func lastSource(t *testing.T, helper *testhelpers.ApiTestHelper, vehicleID string) string {
	t.Helper()
	source, _ := LastPosition(t, helper, vehicleID)
	return source
}

// TestIngestPositions_PhoneWinsDuringTrip - D2: a device point 5 s after a live phone fix adds no row
// and leaves the phone last; once the phone fix is 120 s old the device point is stored with the trip
// and becomes the last position.
func TestIngestPositions_PhoneWinsDuringTrip(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	tripID, vehicleID := liveTrip(t, helper)
	now := time.Now().UTC().Truncate(time.Second)

	ingestPriority(t, helper, sourcePoint(t, vehicleID, models.PointSourcePhone, now.Add(-5*time.Second), -17.39, -66.15))
	ingestPriority(t, helper, sourcePoint(t, vehicleID, models.PointSourceDevice, now, -17.391, -66.151))
	assert.Equal(t, 1, VehiclePositionCount(t, helper, vehicleID))
	assert.Equal(t, models.PointSourcePhone, lastSource(t, helper, vehicleID))

	AgeLastPosition(t, helper, vehicleID, 120*time.Second)
	ingestPriority(t, helper, sourcePoint(t, vehicleID, models.PointSourceDevice, now.Add(time.Second), -17.392, -66.152))
	positions := VehiclePositions(t, helper, vehicleID)
	if assert.Len(t, positions, 2) {
		assert.Equal(t, models.PointSourceDevice, positions[1].Source)
		assert.Equal(t, &tripID, positions[1].TripID)
	}
	assert.Equal(t, models.PointSourceDevice, lastSource(t, helper, vehicleID))
}

// TestIngestPositions_DeviceOffTripMovesLastOnly - D3: with no trip a device point moves the last
// position and stores no trace row.
func TestIngestPositions_DeviceOffTripMovesLastOnly(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	vehicleID := CreateTestVehicleOf(t, helper, MainCompanyID)
	now := time.Now().UTC().Truncate(time.Second)

	ingestPriority(t, helper, sourcePoint(t, vehicleID, models.PointSourceDevice, now, -17.39, -66.15))
	ingestPriority(t, helper, sourcePoint(t, vehicleID, models.PointSourceDevice, now.Add(time.Second), -17.395, -66.155))
	assert.Equal(t, 0, VehiclePositionCount(t, helper, vehicleID))
	source, at := LastPosition(t, helper, vehicleID)
	assert.Equal(t, models.PointSourceDevice, source)
	assert.True(t, at.Equal(now.Add(time.Second)), at)
}
