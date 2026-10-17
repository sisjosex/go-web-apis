//go:build integration
// +build integration

package tracking_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	coreJobs "josex/web/modules/core/jobs"
	"josex/web/modules/core/testhelpers"
	trackingJobs "josex/web/modules/tracking/jobs"
	"josex/web/modules/tracking/models"
)

// TRACK-070 D3: a stop announced by its ETA, minutes before the bus is near.

// startEtaConsumer runs the GPS consumer with the ETA approach on, the ETA answered by eta.
func startEtaConsumer(t *testing.T, helper *testhelpers.ApiTestHelper, minutes int, eta trackingJobs.EtaFunc) *trackingJobs.PositionsConsumer {
	t.Helper()
	if err := helper.Valkey().Client().FlushDB(context.Background()).Err(); err != nil {
		t.Fatalf("Valkey unreachable: %v", err)
	}
	lister := coreJobs.NewTenantDirectory(func(context.Context) ([]coreJobs.Tenant, error) { return []coreJobs.Tenant{testTenant()}, nil })
	consumer := trackingJobs.NewPositionsConsumer(helper.DB(), helper.Valkey(), lister, models.IngestRadii{ArrivalM: 50, ApproachM: 800}).
		WithApproachEta(minutes, eta)
	consumer.Start(context.Background())
	return consumer
}

// farPoint is a fix about 5 km from Central Station: outside the 800 m radius, so only the ETA can
// announce a stop.
func farPoint(vehicleID string, at time.Time) models.StoredPoint {
	return models.StoredPoint{VehicleID: uuid.MustParse(vehicleID), RecordedAt: at, Lat: centralLat + 0.045, Lng: centralLng}
}

// TestPositionsConsumer_ApproachByEta - the stop 4 min away is announced, the one 9 min away is not,
// and another pass announces nothing more.
func TestPositionsConsumer_ApproachByEta(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	tripID, vehicleID := liveTrip(t, helper)
	stops := TripPendingStopIDs(t, helper, tripID)
	if !assert.GreaterOrEqual(t, len(stops), 2, "the seeded trip has two stops") {
		return
	}
	var reads atomic.Int32
	consumer := startEtaConsumer(t, helper, 5, func(context.Context, uuid.UUID, uuid.UUID) ([]models.StopEta, error) {
		reads.Add(1)
		now := time.Now()
		return []models.StopEta{
			{TripStopID: uuid.MustParse(stops[0]), EtaAt: now.Add(4 * time.Minute)},
			{TripStopID: uuid.MustParse(stops[1]), EtaAt: now.Add(9 * time.Minute)},
		}, nil
	})
	defer consumer.Shutdown()

	at := time.Now().UTC().Add(-time.Minute)
	xadd(t, helper, farPoint(vehicleID, at))
	waitFor(t, 5*time.Second, "the near stop announced", func() bool {
		return tripChangedRows(t, helper, tripID, "approach") == 1
	})
	assert.True(t, StopApproachNotified(t, helper, stops[0]), "4 min away: announced")
	assert.False(t, StopApproachNotified(t, helper, stops[1]), "9 min away: not yet")

	// The 30 s throttle let past, a second pass finds the near stop already announced.
	assert.NoError(t, helper.Valkey().Client().Del(context.Background(), "approach:"+tripID).Err())
	xadd(t, helper, farPoint(vehicleID, at.Add(time.Second)))
	waitFor(t, 5*time.Second, "the second ETA read", func() bool { return reads.Load() == 2 })
	time.Sleep(300 * time.Millisecond)
	assert.Equal(t, 1, tripChangedRows(t, helper, tripID, "approach"), "a stop is announced once")
}

// TestPositionsConsumer_ApproachEtaOff - without an ETA nothing is announced away from the stop, and
// minutes 0 never reads one.
func TestPositionsConsumer_ApproachEtaOff(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	tripID, vehicleID := liveTrip(t, helper)

	var reads atomic.Int32
	failing := startEtaConsumer(t, helper, 5, func(context.Context, uuid.UUID, uuid.UUID) ([]models.StopEta, error) {
		reads.Add(1)
		return nil, errors.New("router down")
	})
	at := time.Now().UTC().Add(-time.Minute)
	xadd(t, helper, farPoint(vehicleID, at))
	waitFor(t, 5*time.Second, "the ETA read", func() bool { return reads.Load() == 1 })
	failing.Shutdown()
	assert.Equal(t, 0, tripChangedRows(t, helper, tripID, "approach"))

	reads.Store(0)
	off := startEtaConsumer(t, helper, 0, func(context.Context, uuid.UUID, uuid.UUID) ([]models.StopEta, error) {
		reads.Add(1)
		return nil, nil
	})
	defer off.Shutdown()
	xadd(t, helper, farPoint(vehicleID, at.Add(time.Second)))
	waitFor(t, 5*time.Second, "the point stored", func() bool {
		return VehiclePositionCount(t, helper, vehicleID) == 2
	})
	assert.EqualValues(t, 0, reads.Load(), "minutes 0 reads no ETA")
}
