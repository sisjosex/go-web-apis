//go:build integration
// +build integration

package tracking_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"

	"josex/web/config"
	coreJobs "josex/web/modules/core/jobs"
	coreServices "josex/web/modules/core/services"
	"josex/web/modules/core/testhelpers"
	geoInterfaces "josex/web/modules/geo/interfaces"
	geoRouting "josex/web/modules/geo/services/routing"
	tenancyRepos "josex/web/modules/tenancy/repositories"
	trackingJobs "josex/web/modules/tracking/jobs"
	"josex/web/modules/tracking/models"
	trackingRepos "josex/web/modules/tracking/repositories"
	trackingServices "josex/web/modules/tracking/services"
)

// ============================================================================
// GPS INGEST AND POSITIONS (TRACK-010)
// ============================================================================

// Central Station, the first stop of every createTripRoute route, and a point about 20 m north of it.
const (
	centralLat, centralLng = 40.7128, -74.0060
	nearCentralLat         = 40.71298
)

// liveTrip is today's trip of a fresh route, on a fresh vehicle, driven by the seeded driver account,
// started. It answers the trip and the vehicle.
func liveTrip(t *testing.T, helper *testhelpers.ApiTestHelper) (string, string) {
	t.Helper()
	_, _, tripID := todaysTrip(t, helper)
	vehicleID := CreateTestVehicleOf(t, helper, MainCompanyID)
	driverID := CreateTestDriver(t, helper)
	unlinkDriverAccount(t, helper)
	t.Cleanup(func() { unlinkDriverAccount(t, helper) })
	execSQL(t, helper, `UPDATE tracking.drivers SET user_id = (SELECT id FROM auth.users WHERE email = 'driver@test.local') WHERE id = $1`, driverID)
	execSQL(t, helper, `UPDATE tracking.trips SET vehicle_id = $2, driver_id = $3 WHERE id = $1`, tripID, vehicleID, driverID)
	tripRequest(t, helper, "POST", "/tracking/trips/"+tripID+"/start", nil, http.StatusOK)
	return tripID, vehicleID
}

// unlinkDriverAccount frees the seeded driver account: one driver record per account (uk_driver_user),
// and the tests that link it leave it free again.
func unlinkDriverAccount(t *testing.T, helper *testhelpers.ApiTestHelper) {
	t.Helper()
	execSQL(t, helper, `UPDATE tracking.drivers SET user_id = NULL WHERE user_id = (SELECT id FROM auth.users WHERE email = 'driver@test.local')`)
}

// pointsJSON is a batch for vehicleID as sp_ingest_positions takes it: one point per [lat, lng], a
// second apart from at.
func pointsJSON(t *testing.T, vehicleID string, at time.Time, coords ...[2]float64) []byte {
	t.Helper()
	points := make([]models.StoredPoint, len(coords))
	for i, c := range coords {
		points[i] = models.StoredPoint{VehicleID: uuid.MustParse(vehicleID), RecordedAt: at.Add(time.Duration(i) * time.Second), Lat: c[0], Lng: c[1]}
	}
	raw, err := json.Marshal(points)
	if err != nil {
		t.Fatalf("marshal points: %v", err)
	}
	return raw
}

func ingest(t *testing.T, helper *testhelpers.ApiTestHelper, points []byte) []models.IngestedVehicle {
	t.Helper()
	rows, err := trackingRepos.NewTrackingRepository(helper.DB()).IngestPositions(context.Background(), uuid.MustParse(TestTenantID), points, models.IngestRadii{ArrivalM: 50, ApproachM: 800})
	if err != nil {
		t.Fatalf("sp_ingest_positions: %v", err)
	}
	return rows
}

// ---- step 1: storage ----

// TestIngestPositions_ReplayIsANoOp - the same 3-point batch twice stores 3 rows and one last
// position, the newest point; a late point after it does not move the last position back.
func TestIngestPositions_ReplayIsANoOp(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	vehicleID := CreateTestVehicleOf(t, helper, MainCompanyID)
	at := time.Now().UTC().Truncate(time.Second).Add(-time.Minute)
	batch := pointsJSON(t, vehicleID, at, [2]float64{-17.39, -66.15}, [2]float64{-17.391, -66.151}, [2]float64{-17.392, -66.152})

	ingest(t, helper, batch)
	rows := ingest(t, helper, batch)

	assert.Equal(t, 3, countRows(t, helper, `SELECT count(*) FROM tracking.vehicle_positions WHERE vehicle_id = $1`, vehicleID))
	assert.Equal(t, 1, countRows(t, helper, `SELECT count(*) FROM tracking.vehicle_last_position WHERE vehicle_id = $1`, vehicleID))
	if assert.Len(t, rows, 1) {
		var last map[string]interface{}
		assert.NoError(t, json.Unmarshal(rows[0].Last, &last))
		assert.InDelta(t, -17.392, last["lat"], 1e-9)
		assert.Nil(t, rows[0].TripID)
	}

	ingest(t, helper, pointsJSON(t, vehicleID, at.Add(-time.Hour), [2]float64{-17.5, -66.5}))
	assert.Equal(t, 4, countRows(t, helper, `SELECT count(*) FROM tracking.vehicle_positions WHERE vehicle_id = $1`, vehicleID))
	assert.Equal(t, 1, countRows(t, helper, `SELECT count(*) FROM tracking.vehicle_last_position
		WHERE vehicle_id = $1 AND recorded_at = $2`, vehicleID, at.Add(2*time.Second)))
}

// TestIngestPositions_ArrivalWithinRadius - a point 20 m from the trip's next stop marks it arrived,
// writes exactly one trip.changed row, tags the points with the trip and answers the stop and the
// trip's rider; a far point arrives nowhere.
func TestIngestPositions_ArrivalWithinRadius(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	tripID, vehicleID := liveTrip(t, helper)
	now := time.Now().UTC()

	far := ingest(t, helper, pointsJSON(t, vehicleID, now, [2]float64{centralLat + 0.01, centralLng}))
	if assert.Len(t, far, 1) {
		assert.Nil(t, far[0].ArrivedStopID)
		assert.Equal(t, tripID, far[0].TripID.String())
		assert.Len(t, far[0].RiderIDs, 1)
	}
	assert.Equal(t, 0, tripChangedRows(t, helper, tripID, "arrive"))

	near := ingest(t, helper, pointsJSON(t, vehicleID, now.Add(5*time.Second), [2]float64{nearCentralLat, centralLng}))
	if assert.Len(t, near, 1) && assert.NotNil(t, near[0].ArrivedStopID) {
		assert.Equal(t, "arrived", stopStatus(t, helper, near[0].ArrivedStopID.String()))
	}
	assert.Equal(t, 1, tripChangedRows(t, helper, tripID, "arrive"))
	assert.Equal(t, 2, countRows(t, helper, `SELECT count(*) FROM tracking.vehicle_positions WHERE trip_id = $1`, tripID))
}

func stopStatus(t *testing.T, helper *testhelpers.ApiTestHelper, stopID string) string {
	t.Helper()
	var status string
	if err := helper.DB().QueryRow(context.Background(), `SELECT status FROM tracking.trip_stops WHERE id = $1`, stopID).Scan(&status); err != nil {
		t.Fatalf("read stop %s: %v", stopID, err)
	}
	return status
}

// TestPositionsPartitions_DropsPastRetention - the daily entry drops a day partition dated
// retention + 1 days ago, and keeps a week of partitions ahead of today.
func TestPositionsPartitions_DropsPastRetention(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	old := time.Now().UTC().AddDate(0, 0, -31)
	name := "vehicle_positions_p" + old.Format("20060102")
	var ddl string
	if err := helper.DB().QueryRow(context.Background(), `SELECT format('CREATE TABLE IF NOT EXISTS tracking.%I PARTITION OF tracking.vehicle_positions FOR VALUES FROM (%L) TO (%L)',
		$1::text, $2::text || ' 00:00:00+00', $3::text || ' 00:00:00+00')`, name, old.Format(time.DateOnly), old.AddDate(0, 0, 1).Format(time.DateOnly)).Scan(&ddl); err != nil {
		t.Fatalf("build ddl: %v", err)
	}
	execSQL(t, helper, ddl)
	assert.Equal(t, 1, countRows(t, helper, `SELECT count(*) FROM pg_class WHERE relname = $1`, name))

	lister := func(context.Context) ([]coreJobs.Tenant, error) { return []coreJobs.Tenant{testTenant()}, nil }
	if err := trackingJobs.PositionsPartitionsHandler(helper.DB(), lister, 30)(context.Background(), asynq.NewTask(trackingJobs.TaskPositionsPartitions, nil)); err != nil {
		t.Fatalf("positions partitions: %v", err)
	}
	assert.Equal(t, 0, countRows(t, helper, `SELECT count(*) FROM pg_class WHERE relname = $1`, name))
	assert.Equal(t, 1, countRows(t, helper, `SELECT count(*) FROM pg_class WHERE relname = $1`, "vehicle_positions_p"+time.Now().UTC().AddDate(0, 0, 7).Format("20060102")))
}

// ---- step 4: the trace at trip close ----

// TestTripTrace_StoredAtComplete - a completed trip driven through Cochabamba answers a polyline and
// its km on GET /trips/:id: map-matched when Valhalla answers, the raw line flagged fallback when
// there is no router.
func TestTripTrace_StoredAtComplete(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	for _, withRouter := range []bool{true, false} {
		tripID, vehicleID := liveTrip(t, helper)
		// Started ten minutes ago, so the points below fall inside the trip whatever the skew between
		// this clock and the database's.
		execSQL(t, helper, `UPDATE tracking.trips SET started_at = now() - INTERVAL '10 minutes' WHERE id = $1`, tripID)
		ingest(t, helper, pointsJSON(t, vehicleID, time.Now().UTC().Add(-5*time.Minute),
			[2]float64{-17.3935, -66.1570}, [2]float64{-17.3940, -66.1560}, [2]float64{-17.3950, -66.1550}, [2]float64{-17.3962, -66.1541}))
		tripRequest(t, helper, "POST", "/tracking/trips/"+tripID+"/complete", nil, http.StatusOK)

		var router geoInterfaces.Router
		if withRouter {
			router = geoRouting.NewRouter(config.ModularAppConfig.Geo)
		}
		source, err := trackingJobs.ApplyTripTrace(context.Background(), helper.DB(), router, testTenant(), uuid.MustParse(tripID))
		if err != nil {
			t.Fatalf("trace: %v", err)
		}
		if !withRouter {
			assert.Equal(t, trackingJobs.TraceFallback, source)
		}

		detail := tripRequest(t, helper, "GET", "/tracking/trips/"+tripID, nil, http.StatusOK)
		if assert.NotNil(t, detail.Polyline, source) && assert.NotNil(t, detail.DistanceKm) {
			assert.NotEmpty(t, *detail.Polyline)
			assert.Greater(t, *detail.DistanceKm, 0.0)
			assert.Equal(t, source, *detail.TraceSource)
		}
	}
}

// ---- step 2: the endpoint ----

func ingestBody(lat, lng float64) []map[string]interface{} {
	return []map[string]interface{}{{"recorded_at": time.Now().UTC().Format(time.RFC3339Nano), "lat": lat, "lng": lng, "speed": 8.5, "heading": 90, "seq": 1}}
}

// TestIngestEndpoint_Driver - the driver on a trip in progress posts → 202 queued; the driver with no
// such trip → 409 no-active-trip; an operator account has no trip either → 409.
func TestIngestEndpoint_Driver(t *testing.T) {
	admin := SetupTrackingTest(t)
	defer admin.Close()
	driver := SetupDriverTest(t)
	defer driver.Close()

	w := driver.DoRequest("POST", "/tracking/ingest/positions", ingestBody(centralLat, centralLng), map[string]string{})
	assert.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "tracking.ingest.no-active-trip")

	_, vehicleID := liveTrip(t, admin)
	stream := trackingServices.GPSStream(TestTenantID)
	before := driver.Valkey().Client().XLen(context.Background(), stream).Val()
	w = driver.DoRequest("POST", "/tracking/ingest/positions", ingestBody(-17.39, -66.15), map[string]string{})
	assert.Equal(t, http.StatusAccepted, w.Code, w.Body.String())
	var res models.IngestResponse
	decodeBody(t, w, &res)
	assert.Equal(t, models.IngestResponse{Accepted: 1, Path: models.IngestPathQueued}, res)
	assert.Equal(t, before+1, driver.Valkey().Client().XLen(context.Background(), stream).Val())
	entries := driver.Valkey().Client().XRevRangeN(context.Background(), stream, "+", "-", 1).Val()
	if assert.Len(t, entries, 1) {
		assert.Contains(t, entries[0].Values[trackingServices.GPSStreamField], vehicleID)
	}
}

// TestIngestEndpoint_Validation - an empty batch, a point out of range and a point in the future → 400.
func TestIngestEndpoint_Validation(t *testing.T) {
	driver := SetupDriverTest(t)
	defer driver.Close()
	future := []map[string]interface{}{{"recorded_at": time.Now().Add(time.Hour).UTC().Format(time.RFC3339), "lat": 1, "lng": 1}}
	for name, body := range map[string]interface{}{
		"empty":        []map[string]interface{}{},
		"out of range": ingestBody(91, 0),
		"future":       future,
	} {
		w := driver.DoRequest("POST", "/tracking/ingest/positions", body, map[string]string{})
		assert.Equal(t, http.StatusBadRequest, w.Code, name+": "+w.Body.String())
	}
}

// TestIngestEndpoint_DeviceToken - a device posts with its token alone, no session and no slug → 202,
// last_seen_at stamped; a malformed token, a wrong secret and the right secret under another tenant
// → 401.
func TestIngestEndpoint_DeviceToken(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	vehicleID := CreateTestVehicleOf(t, helper, MainCompanyID)
	token := tenancyRepos.GPSDeviceToken(uuid.MustParse(TestTenantID), []byte(uuid.NewString()))
	_, hash, _ := tenancyRepos.ParseGPSDeviceToken(token)
	if err := tenancyRepos.NewGPSDeviceRepository(helper.DB()).Issue(context.Background(), uuid.MustParse(TestTenantID), uuid.MustParse(vehicleID), hash); err != nil {
		t.Fatalf("issue device: %v", err)
	}
	device := testhelpers.SetupApiTest(t)
	defer device.Close()

	w := device.DoRequest("POST", "/tracking/ingest/positions", ingestBody(-17.39, -66.15), map[string]string{"X-Device-Token": token})
	assert.Equal(t, http.StatusAccepted, w.Code, w.Body.String())
	assert.Equal(t, 1, countRows(t, helper, `SELECT count(*) FROM tenancy.gps_devices WHERE vehicle_id = $1 AND last_seen_at IS NOT NULL`, vehicleID))

	for _, forged := range []string{"nope", TestTenantID + ".nope", uuid.NewString() + token[36:]} {
		w = device.DoRequest("POST", "/tracking/ingest/positions", ingestBody(-17.39, -66.15), map[string]string{"X-Device-Token": forged})
		assert.Equal(t, http.StatusUnauthorized, w.Code, forged+": "+w.Body.String())
	}
}

// TestPositionIngest_ValkeyDownStoresInline - D2: no Valkey, or one that does not answer, and the
// batch is stored in the same request → stored, the row is there.
func TestPositionIngest_ValkeyDownStoresInline(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	vehicleID := CreateTestVehicleOf(t, helper, MainCompanyID)
	dead, err := coreServices.NewValkeyService("redis://127.0.0.1:1/0")
	if err != nil {
		t.Fatalf("valkey client: %v", err)
	}
	repo := trackingRepos.NewTrackingRepository(helper.DB())

	for i, valkey := range []coreServices.ValkeyService{nil, dead} {
		point := models.StoredPoint{VehicleID: uuid.MustParse(vehicleID), RecordedAt: time.Now().UTC().Add(time.Duration(i) * time.Second), Lat: -17.39, Lng: -66.15}
		path, err := trackingServices.NewPositionIngest(repo, valkey, 1000, models.IngestRadii{ArrivalM: 50, ApproachM: 800}).Ingest(context.Background(), uuid.MustParse(TestTenantID), []models.StoredPoint{point})
		assert.NoError(t, err)
		assert.Equal(t, models.IngestPathStored, path)
	}
	assert.Equal(t, 2, countRows(t, helper, `SELECT count(*) FROM tracking.vehicle_positions WHERE vehicle_id = $1`, vehicleID))
}

// ---- step 3: the worker ----

// startTestConsumer runs the GPS consumer over the seeded tenant, on a clean Valkey test database.
func startTestConsumer(t *testing.T, helper *testhelpers.ApiTestHelper, claimIdle time.Duration) *trackingJobs.PositionsConsumer {
	t.Helper()
	valkey := helper.Valkey()
	if valkey == nil {
		t.Fatal("REDIS_URL is not set in .env.test — the GPS consumer tests need Valkey (make docker-up)")
	}
	lister := func(context.Context) ([]coreJobs.Tenant, error) { return []coreJobs.Tenant{testTenant()}, nil }
	consumer := trackingJobs.NewPositionsConsumer(helper.DB(), valkey, lister, models.IngestRadii{ArrivalM: 50, ApproachM: 800})
	consumer.ClaimIdle = claimIdle
	consumer.ClaimEvery = claimIdle
	consumer.Start(context.Background())
	return consumer
}

// xadd queues one point on the seeded tenant's stream, as the API does.
func xadd(t *testing.T, helper *testhelpers.ApiTestHelper, point models.StoredPoint) {
	t.Helper()
	raw, _ := json.Marshal(point)
	if err := helper.Valkey().Client().XAdd(context.Background(), &redis.XAddArgs{
		Stream: trackingServices.GPSStream(TestTenantID), Values: []string{trackingServices.GPSStreamField, string(raw)},
	}).Err(); err != nil {
		t.Fatalf("xadd: %v", err)
	}
}

// TestPositionsConsumer_ThrottledPublish - three batches of one vehicle within two seconds are all
// stored, and the fleet channel carries one position frame for it.
func TestPositionsConsumer_ThrottledPublish(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	ctx := context.Background()
	if err := helper.Valkey().Client().FlushDB(ctx).Err(); err != nil {
		t.Fatalf("Valkey unreachable: %v", err)
	}
	vehicleID := CreateTestVehicleOf(t, helper, MainCompanyID)
	sub := helper.Valkey().Client().Subscribe(ctx, trackingJobs.FleetChannel(TestTenantID))
	defer sub.Close()
	if _, err := sub.Receive(ctx); err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	consumer := startTestConsumer(t, helper, time.Minute)
	defer consumer.Shutdown()

	at := time.Now().UTC().Add(-time.Minute)
	for i := 0; i < 3; i++ {
		xadd(t, helper, models.StoredPoint{VehicleID: uuid.MustParse(vehicleID), RecordedAt: at.Add(time.Duration(i) * time.Second), Lat: -17.39, Lng: -66.15})
		time.Sleep(600 * time.Millisecond)
	}
	waitFor(t, 3*time.Second, "the three points stored", func() bool {
		return countRows(t, helper, `SELECT count(*) FROM tracking.vehicle_positions WHERE vehicle_id = $1`, vehicleID) == 3
	})

	frames := 0
	deadline := time.After(time.Second)
	for done := false; !done; {
		select {
		case msg := <-sub.Channel():
			var frame trackingJobs.Frame
			if json.Unmarshal([]byte(msg.Payload), &frame) == nil && frame.Type == trackingJobs.FramePosition && strings.Contains(string(frame.Data), vehicleID) {
				frames++
			}
		case <-deadline:
			done = true
		}
	}
	assert.Equal(t, 1, frames, "one position frame per vehicle per 3 s")
}

// TestPositionsConsumer_CrashMidBatch - a consumer that read entries and died before acking leaves
// them pending; the next consumer claims them and stores them once, even when the dead one had
// already written them.
func TestPositionsConsumer_CrashMidBatch(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	ctx := context.Background()
	client := helper.Valkey().Client()
	if err := client.FlushDB(ctx).Err(); err != nil {
		t.Fatalf("Valkey unreachable: %v", err)
	}
	vehicleID := CreateTestVehicleOf(t, helper, MainCompanyID)
	stream := trackingServices.GPSStream(TestTenantID)
	at := time.Now().UTC().Add(-time.Minute)
	for i := 0; i < 5; i++ {
		xadd(t, helper, models.StoredPoint{VehicleID: uuid.MustParse(vehicleID), RecordedAt: at.Add(time.Duration(i) * time.Second), Lat: -17.39, Lng: -66.15})
	}

	// The dead consumer: reads everything, stores the first two points, never acks.
	if err := client.XGroupCreateMkStream(ctx, stream, "gps", "0").Err(); err != nil {
		t.Fatalf("create group: %v", err)
	}
	if err := client.XReadGroup(ctx, &redis.XReadGroupArgs{Group: "gps", Consumer: "dead", Streams: []string{stream, ">"}, Count: 10}).Err(); err != nil {
		t.Fatalf("dead read: %v", err)
	}
	ingest(t, helper, pointsJSON(t, vehicleID, at, [2]float64{-17.39, -66.15}, [2]float64{-17.39, -66.15}))

	consumer := startTestConsumer(t, helper, 10*time.Millisecond)
	defer consumer.Shutdown()
	waitFor(t, 3*time.Second, "the pending entries acked", func() bool {
		pending, err := client.XPending(ctx, stream, "gps").Result()
		return err == nil && pending.Count == 0
	})
	assert.Equal(t, 5, countRows(t, helper, `SELECT count(*) FROM tracking.vehicle_positions WHERE vehicle_id = $1`, vehicleID))
}
