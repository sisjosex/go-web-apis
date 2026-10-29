//go:build integration
// +build integration

package tracking_test

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"josex/web/modules/core/testhelpers"
	tenancyRepos "josex/web/modules/tenancy/repositories"
	trackingJobs "josex/web/modules/tracking/jobs"
	"josex/web/modules/tracking/models"
)

// ============================================================================
// OSMAND PROTOCOL (TRACK-068 D1)
// ============================================================================

// issueDeviceToken links a fresh device to vehicleID the way the CLI does and answers its token.
func issueDeviceToken(t *testing.T, helper *testhelpers.ApiTestHelper, vehicleID string) string {
	t.Helper()
	token := tenancyRepos.GPSDeviceToken(uuid.MustParse(TestTenantID), []byte(uuid.NewString()))
	_, hash, _ := tenancyRepos.ParseGPSDeviceToken(token)
	if err := tenancyRepos.NewGPSDeviceRepository(helper.DB()).Issue(context.Background(), uuid.MustParse(TestTenantID), uuid.MustParse(vehicleID), hash); err != nil {
		t.Fatalf("issue device: %v", err)
	}
	return token
}

// osmandURL is one OsmAnd point of the device with that token.
func osmandURL(token string, lat float64, at time.Time) string {
	q := url.Values{
		"id": {token}, "lat": {fmt.Sprint(lat)}, "lon": {"-66.15"},
		"timestamp": {fmt.Sprint(at.Unix())}, "speed": {"10"}, "bearing": {"90"}, "accuracy": {"8"},
		"altitude": {"2550"}, "batt": {"87"},
	}
	return "/gps/osmand?" + q.Encode()
}

// TestIngestOsmand_DevicePointDuringTrip - a tracker's GET during a trip with a silent phone → 200
// with an empty body and one row source=device, speed in m/s, the bearing as heading; the same
// timestamp again (as POST) → still one row; a forged id → 401; lat=91 → 400.
func TestIngestOsmand_DevicePointDuringTrip(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	if err := helper.Valkey().Client().FlushDB(context.Background()).Err(); err != nil {
		t.Fatalf("Valkey unreachable: %v", err)
	}
	tripID, vehicleID := liveTrip(t, helper)
	token := issueDeviceToken(t, helper, vehicleID)
	// The device chain queues the seeded tenant's points on the shared stream (no database URL).
	consumer := trackingJobs.NewPositionsConsumer(helper.DB(), helper.Valkey(), sharedTenants(TestTenantID), models.IngestRadii{ArrivalM: 50, ApproachM: 800})
	consumer.Start(context.Background())
	defer consumer.Shutdown()
	device := testhelpers.SetupApiTest(t)
	defer device.Close()
	at := time.Now().Add(-10 * time.Second)

	w := device.DoRequest("GET", osmandURL(token, -17.39, at), nil, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Empty(t, w.Body.String())
	waitFor(t, 3*time.Second, "the device point stored", func() bool {
		return VehiclePositionCount(t, helper, vehicleID) == 1
	})
	stored := VehiclePositions(t, helper, vehicleID)[0]
	assert.Equal(t, models.PointSourceDevice, stored.Source)
	assert.Equal(t, &tripID, stored.TripID)
	if assert.NotNil(t, stored.Speed) && assert.NotNil(t, stored.Heading) {
		assert.InDelta(t, 5.14444, *stored.Speed, 0.001)
		assert.InDelta(t, 90, *stored.Heading, 1e-6)
	}

	w = device.DoRequest("POST", osmandURL(token, -17.39, at), nil, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	time.Sleep(time.Second)
	assert.Equal(t, 1, VehiclePositionCount(t, helper, vehicleID))

	w = device.DoRequest("GET", osmandURL(TestTenantID+".nope", -17.39, at), nil, map[string]string{})
	assert.Equal(t, http.StatusUnauthorized, w.Code, w.Body.String())
	w = device.DoRequest("GET", osmandURL(token, 91, at), nil, map[string]string{})
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
}
