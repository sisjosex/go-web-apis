//go:build integration
// +build integration

package tracking_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"josex/web/config"
	coreJobs "josex/web/modules/core/jobs"
	"josex/web/modules/core/testhelpers"
	geoInterfaces "josex/web/modules/geo/interfaces"
	geoModels "josex/web/modules/geo/models"
	geoServices "josex/web/modules/geo/services"
	geoRouting "josex/web/modules/geo/services/routing"
	trackingJobs "josex/web/modules/tracking/jobs"
	"josex/web/modules/tracking/models"
	trackingRepos "josex/web/modules/tracking/repositories"
	trackingServices "josex/web/modules/tracking/services"
)

// ============================================================================
// REALTIME GATEWAY, SNAPSHOTS AND ETA (TRACK-025)
// ============================================================================

// cochabambaTrip is today's trip of a fresh route through two new stops in central Cochabamba — where
// the local Valhalla has roads — on a fresh vehicle, started, with a point ingested before the first
// stop. It answers the trip and the vehicle.
func cochabambaTrip(t *testing.T, helper *testhelpers.ApiTestHelper) (string, string) {
	t.Helper()
	routeID, _ := cochabambaRoute(t, helper)
	return startCochabambaTrip(t, helper, routeID)
}

// cochabambaRoute is a fresh route through two new stops in central Cochabamba, running every day
// at 07:00. It answers the route and its stops.
func cochabambaRoute(t *testing.T, helper *testhelpers.ApiTestHelper) (string, []string) {
	t.Helper()
	stops := make([]string, 2)
	for i, c := range [][2]float64{{-17.3950, -66.1550}, {-17.3890, -66.1480}} {
		w := helper.DoRequest("POST", "/tracking/stop-places", map[string]interface{}{
			"name": "Cbba Stop " + uuid.NewString()[:8], "address": "Cochabamba", "latitude": c[0], "longitude": c[1],
		}, map[string]string{})
		if w.Code != http.StatusCreated {
			t.Fatalf("create stop place: %d %s", w.Code, w.Body.String())
		}
		stops[i] = ExtractID(t, ParseResponse(t, w.Body.Bytes()))
	}
	routeID := CreateTestRoute(t, helper)
	PublishRouteVersion(t, helper, routeID, "2026-01-01", []map[string]interface{}{
		{"stop_place_id": stops[0], "sequence": 1, "planned_offset_min": 0},
		{"stop_place_id": stops[1], "sequence": 2, "planned_offset_min": 20},
	})
	CreateRouteSchedule(t, helper, routeID, map[string]interface{}{"days_of_week": EveryDayMask, "start_time": "07:00", "valid_from": "2026-01-01"})
	return routeID, stops
}

// startCochabambaTrip starts today's trip of routeID on a fresh vehicle and ingests a point before
// its first stop. It answers the trip and the vehicle.
func startCochabambaTrip(t *testing.T, helper *testhelpers.ApiTestHelper, routeID string) (string, string) {
	t.Helper()
	today := dayOffset(t, "UTC", 0)
	materialiseRoute(t, helper, routeID, today, today)
	tripID, _ := tripOn(t, helper, routeID, today, "07:00")
	vehicleID := CreateTestVehicleOf(t, helper, MainCompanyID)
	tripRequest(t, helper, "PATCH", "/tracking/trips/"+tripID, map[string]interface{}{"vehicle_id": vehicleID}, http.StatusOK)
	tripRequest(t, helper, "POST", "/tracking/trips/"+tripID+"/start", nil, http.StatusOK)
	ingest(t, helper, pointsJSON(t, vehicleID, time.Now().UTC(), [2]float64{-17.3935, -66.1570}))
	return tripID, vehicleID
}

// ---- step 1: snapshots ----

// TestLiveFleet_Snapshot - the fleet snapshot lists the ingesting vehicle with its trip and route; the
// organization filter keeps it for its rider's school and drops it for another; a guardian → 403.
func TestLiveFleet_Snapshot(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	tripID, vehicleID := liveTrip(t, helper)
	ingest(t, helper, pointsJSON(t, vehicleID, time.Now().UTC(), [2]float64{-17.39, -66.15}))

	find := func(query string) *models.LiveVehicle {
		w := helper.DoRequest("GET", "/tracking/live/fleet"+query, nil, map[string]string{})
		if w.Code != http.StatusOK {
			t.Fatalf("live fleet %s: %d %s", query, w.Code, w.Body.String())
		}
		var res models.LiveFleetResponse
		decodeBody(t, w, &res)
		for i := range res.Vehicles {
			if res.Vehicles[i].VehicleID.String() == vehicleID {
				return &res.Vehicles[i]
			}
		}
		return nil
	}
	v := find("")
	if assert.NotNil(t, v) {
		assert.Equal(t, tripID, v.TripID.String())
		assert.NotNil(t, v.RouteName)
		assert.InDelta(t, -17.39, v.Lat, 1e-9)
	}
	assert.NotNil(t, find("?organization_id="+MainSchoolID))
	assert.Nil(t, find("?organization_id="+EmptyEmployerID))

	portal := SetupPortalTest(t)
	defer portal.Close()
	w := portal.DoRequest("GET", "/tracking/live/fleet", nil, map[string]string{})
	assert.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
}

// countingRouter counts the route calls that reach the real router.
type countingRouter struct {
	geoInterfaces.Router
	routes atomic.Int32
}

func (r *countingRouter) Route(ctx context.Context, locations []geoModels.LatLng, costing string) (*geoModels.Route, error) {
	r.routes.Add(1)
	return r.Router.Route(ctx, locations, costing)
}

// TestTripLive_EtaPerPendingStop - the trip snapshot answers the position and one ETA per pending stop,
// later stops later; twice within 30 s is one routing call (D1).
func TestTripLive_EtaPerPendingStop(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	tripID, _ := cochabambaTrip(t, helper)

	w := helper.DoRequest("GET", "/tracking/trips/"+tripID+"/live", nil, map[string]string{})
	if w.Code != http.StatusOK {
		t.Fatalf("trip live: %d %s", w.Code, w.Body.String())
	}
	var live models.TripLive
	decodeBody(t, w, &live)
	assert.NotEmpty(t, string(live.Position))
	if assert.Len(t, live.Eta, 2) {
		assert.True(t, live.Eta[1].EtaAt.After(live.Eta[0].EtaAt))
		assert.NotEmpty(t, live.Eta[0].EstimateSource)
		assert.Equal(t, live.Stops[0].ID, live.Eta[0].TripStopID)
	}

	ctx := context.Background()
	_ = helper.Valkey().Client().Del(ctx, "eta:"+tripID).Err()
	geoConf := config.ModularAppConfig.Geo
	router := &countingRouter{Router: geoRouting.NewRouter(geoConf)}
	eta := trackingServices.NewEtaService(router, geoServices.NewGeoService(nil, router, nil, geoConf), helper.Valkey())
	live2 := trackingServices.NewLiveService(trackingRepos.NewTrackingRepository(helper.DB()), eta)
	for i := 0; i < 2; i++ {
		if _, _, err := live2.Trip(ctx, uuid.MustParse(TestTenantID), uuid.MustParse(tripID)); err != nil {
			t.Fatalf("trip live %d: %v", i, err)
		}
	}
	assert.Equal(t, int32(1), router.routes.Load(), "one routing call per trip per 30 s")
}

// ---- step 3: the endpoint ----

// wsClient is one signed-in socket and the frames it has received.
type wsClient struct {
	conn   *websocket.Conn
	frames chan map[string]interface{}
	closed chan websocket.StatusCode
}

// dialWS opens GET /tracking/ws as a browser does: the token as the bearer subprotocol, the tenant
// as ?tenant_slug=.
func dialWS(t *testing.T, server *httptest.Server, helper *testhelpers.ApiTestHelper) *wsClient {
	t.Helper()
	url := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/v1/tracking/ws?tenant_slug=test-company"
	conn, res, err := websocket.Dial(context.Background(), url, &websocket.DialOptions{Subprotocols: []string{"bearer", helper.Token()}})
	if err != nil {
		status := 0
		if res != nil {
			status = res.StatusCode
		}
		t.Fatalf("dial ws: %v (%d)", err, status)
	}
	c := &wsClient{conn: conn, frames: make(chan map[string]interface{}, 256), closed: make(chan websocket.StatusCode, 1)}
	go func() {
		for {
			_, raw, err := conn.Read(context.Background())
			if err != nil {
				c.closed <- websocket.CloseStatus(err)
				return
			}
			var f map[string]interface{}
			if json.Unmarshal(raw, &f) == nil {
				c.frames <- f
			}
		}
	}()
	return c
}

func (c *wsClient) send(t *testing.T, op, channel string) {
	t.Helper()
	raw, _ := json.Marshal(map[string]string{"op": op, "channel": channel})
	if err := c.conn.Write(context.Background(), websocket.MessageText, raw); err != nil {
		t.Fatalf("ws write: %v", err)
	}
}

// collect answers the frames received within d.
func (c *wsClient) collect(d time.Duration) []map[string]interface{} {
	var out []map[string]interface{}
	deadline := time.After(d)
	for {
		select {
		case f := <-c.frames:
			out = append(out, f)
		case <-deadline:
			return out
		}
	}
}

// waitFrame answers the first frame of type on channel within d, or nil.
func (c *wsClient) waitFrame(channel, frameType string, d time.Duration) map[string]interface{} {
	deadline := time.After(d)
	for {
		select {
		case f := <-c.frames:
			if f["channel"] == channel && f["type"] == frameType {
				return f
			}
		case <-deadline:
			return nil
		}
	}
}

// TestRealtimeSocket_FleetAndRefusal - an operator subscribed to its fleet sees both ingesting
// vehicles, at most one position per 3 s each; a guardian's fleet subscribe answers a forbidden frame
// and keeps the socket.
func TestRealtimeSocket_FleetAndRefusal(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	ctx := context.Background()
	if err := helper.Valkey().Client().FlushDB(ctx).Err(); err != nil {
		t.Fatalf("Valkey unreachable: %v", err)
	}
	server := httptest.NewServer(helper.Engine())
	defer server.Close()
	vehicles := []string{CreateTestVehicleOf(t, helper, MainCompanyID), CreateTestVehicleOf(t, helper, MainCompanyID)}

	operator := dialWS(t, server, helper)
	defer operator.conn.CloseNow()
	fleet := trackingJobs.FleetChannel(TestTenantID)
	operator.send(t, "subscribe", fleet)
	time.Sleep(200 * time.Millisecond)

	consumer := startTestConsumer(t, helper, time.Minute)
	defer consumer.Shutdown()
	at := time.Now().UTC().Add(-time.Minute)
	for i := 0; i < 8; i++ {
		for _, v := range vehicles {
			xadd(t, helper, models.StoredPoint{VehicleID: uuid.MustParse(v), RecordedAt: at.Add(time.Duration(i) * time.Second), Lat: -17.39, Lng: -66.15})
		}
		time.Sleep(500 * time.Millisecond)
	}
	perVehicle := map[string]int{}
	for _, f := range operator.collect(time.Second) {
		if f["channel"] != fleet || f["type"] != "position" {
			continue
		}
		data, _ := json.Marshal(f["data"])
		for _, v := range vehicles {
			if strings.Contains(string(data), v) {
				perVehicle[v]++
			}
		}
	}
	for _, v := range vehicles {
		// Five seconds of points: the throttle lets through one, then one more after 3 s.
		assert.GreaterOrEqual(t, perVehicle[v], 1, v)
		assert.LessOrEqual(t, perVehicle[v], 2, v)
	}

	portal := SetupPortalTest(t)
	defer portal.Close()
	guardian := dialWS(t, server, portal)
	defer guardian.conn.CloseNow()
	guardian.send(t, "subscribe", fleet)
	refusal := guardian.waitFrame(fleet, "error", 2*time.Second)
	if assert.NotNil(t, refusal) {
		assert.Equal(t, "forbidden", refusal["code"])
	}
	guardian.send(t, "subscribe", trackingJobs.RiderChannel(TestRiderJaneID))
	assert.NotNil(t, guardian.waitFrame(trackingJobs.RiderChannel(TestRiderJaneID), "error", 2*time.Second), "the socket stays open")
}

// ---- step 4: fan-out ----

// TestRealtimeSocket_TripChangedFanOut - a start reaches the trip channel within a second, through
// the outbox, the worker and pub/sub; the rider's guardian subscribes while the trip runs and gets
// the next stop; a guardian of someone else is refused.
func TestRealtimeSocket_TripChangedFanOut(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	routeID, riderID := createTripRoute(t, helper, "07:00")
	// The seeded guardian becomes this rider's guardian too.
	execSQL(t, helper, `INSERT INTO tracking.rider_contacts (rider_id, relation, name, phone, email, user_id, is_primary)
		SELECT $1, 'guardian', 'Portal Guardian', '+3333333333', u.email, u.id, FALSE FROM auth.users u WHERE u.email = 'portal@test.local'`, riderID)
	// The seeded guardian has exactly one rider everywhere else.
	t.Cleanup(func() { execSQL(t, helper, `DELETE FROM tracking.rider_contacts WHERE rider_id = $1`, riderID) })
	today := dayOffset(t, "UTC", 0)
	materialiseRoute(t, helper, routeID, today, today)
	tripID, _ := tripOn(t, helper, routeID, today, "07:00")

	relay, inspector := startTestRelay(t, helper, TestTenantID)
	defer relay.Shutdown()
	defer inspector.Close()
	registry := coreJobs.NewRegistry()
	lister := func(context.Context) ([]coreJobs.Tenant, error) { return []coreJobs.Tenant{testTenant()}, nil }
	registry.Handle(trackingJobs.TaskTripChanged, trackingJobs.TripChangedHandler(helper.DB(), lister, nil, helper.Valkey(), nil))
	worker, err := coreJobs.StartWorker(helper.Valkey(), registry, 2)
	if err != nil {
		t.Fatalf("worker: %v", err)
	}
	defer worker.Shutdown()

	server := httptest.NewServer(helper.Engine())
	defer server.Close()
	operator := dialWS(t, server, helper)
	defer operator.conn.CloseNow()
	trip := trackingJobs.TripChannel(tripID)
	operator.send(t, "subscribe", trip)
	time.Sleep(200 * time.Millisecond)

	started := time.Now()
	detail := tripRequest(t, helper, "POST", "/tracking/trips/"+tripID+"/start", nil, http.StatusOK)
	frame := operator.waitFrame(trip, "trip_status", 3*time.Second)
	if assert.NotNil(t, frame, "trip_status on the trip channel") {
		assert.Less(t, time.Since(started), time.Second)
	}

	portal := SetupPortalTest(t)
	defer portal.Close()
	guardian := dialWS(t, server, portal)
	defer guardian.conn.CloseNow()
	rider := trackingJobs.RiderChannel(riderID)
	guardian.send(t, "subscribe", rider)
	time.Sleep(200 * time.Millisecond)
	tripRequest(t, helper, "POST", "/tracking/trip-stops/"+detail.Stops[0].ID.String()+"/arrive", nil, http.StatusOK)
	assert.NotNil(t, guardian.waitFrame(rider, "stop", 3*time.Second), "the guardian's rider channel carries the stop")

	stranger := SetupUnlinkedPortalTest(t)
	defer stranger.Close()
	other := dialWS(t, server, stranger)
	defer other.conn.CloseNow()
	other.send(t, "subscribe", rider)
	refusal := other.waitFrame(rider, "error", 2*time.Second)
	if assert.NotNil(t, refusal) {
		assert.Equal(t, "forbidden", refusal["code"])
	}
}

// ---- MOBILE-010: the guardian's map ----

// guardianRiding is a fresh rider on today's started Cochabamba trip, linked to the seeded guardian,
// on a fresh vehicle that has just reported. It answers the rider, the trip and the vehicle.
func guardianRiding(t *testing.T, helper *testhelpers.ApiTestHelper) (string, string, string) {
	t.Helper()
	routeID, stops := cochabambaRoute(t, helper)
	riderID := CreateTestRider(t, helper)
	body := AssignmentBody(riderID, routeID, EveryDayMask)
	body["pickup_stop_place_id"] = stops[0]
	body["dropoff_stop_place_id"] = stops[1]
	CreateAssignment(t, helper, body)
	LinkPortalGuardian(t, helper, riderID)
	tripID, vehicleID := startCochabambaTrip(t, helper, routeID)
	return riderID, tripID, vehicleID
}

// TestRiderLive_GuardianSnapshot - while the rider rides, the guardian's snapshot answers the trip,
// the rider's two stops with coordinates and the vehicle's position; someone else's guardian → 404;
// once the trip is completed, no trip and no position.
func TestRiderLive_GuardianSnapshot(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	riderID, tripID, vehicleID := guardianRiding(t, helper)

	portal := SetupPortalTest(t)
	defer portal.Close()
	w := portal.DoRequest("GET", "/tracking/riders/"+riderID+"/live", nil, map[string]string{})
	if w.Code != http.StatusOK {
		t.Fatalf("rider live: %d %s", w.Code, w.Body.String())
	}
	var live models.RiderLive
	decodeBody(t, w, &live)
	if assert.NotNil(t, live.TripID) {
		assert.Equal(t, tripID, live.TripID.String())
	}
	if assert.Len(t, live.Stops, 2) {
		assert.Equal(t, "pickup", live.Stops[0].Kind)
		assert.NotZero(t, live.Stops[0].Lat)
		assert.NotZero(t, live.Stops[0].Lng)
	}
	var position struct {
		VehicleID string  `json:"vehicle_id"`
		Lat       float64 `json:"lat"`
	}
	if assert.NoError(t, json.Unmarshal(live.Position, &position)) {
		assert.Equal(t, vehicleID, position.VehicleID)
		assert.InDelta(t, -17.3935, position.Lat, 1e-6)
	}
	assert.NotEmpty(t, live.Eta, "the ETA to the rider's pending stops")
	for _, e := range live.Eta {
		assert.Contains(t, []uuid.UUID{live.Stops[0].TripStopID, live.Stops[1].TripStopID}, e.TripStopID)
	}

	stranger := SetupUnlinkedPortalTest(t)
	defer stranger.Close()
	w = stranger.DoRequest("GET", "/tracking/riders/"+riderID+"/live", nil, map[string]string{})
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())

	tripRequest(t, helper, "POST", "/tracking/trips/"+tripID+"/complete", nil, http.StatusOK)
	w = portal.DoRequest("GET", "/tracking/riders/"+riderID+"/live", nil, map[string]string{})
	if w.Code != http.StatusOK {
		t.Fatalf("rider live after the trip: %d %s", w.Code, w.Body.String())
	}
	live = models.RiderLive{}
	decodeBody(t, w, &live)
	assert.Nil(t, live.TripID)
	assert.Empty(t, live.Stops)
	assert.Equal(t, "null", string(live.Position))
}

// TestTripLive_StopsCarryCoordinates - the staff snapshot's stops carry where they are.
func TestTripLive_StopsCarryCoordinates(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	tripID, _ := cochabambaTrip(t, helper)
	w := helper.DoRequest("GET", "/tracking/trips/"+tripID+"/live", nil, map[string]string{})
	if w.Code != http.StatusOK {
		t.Fatalf("trip live: %d %s", w.Code, w.Body.String())
	}
	var live models.TripLive
	decodeBody(t, w, &live)
	if assert.Len(t, live.Stops, 2) && assert.NotNil(t, live.Stops[0].Lat) {
		assert.InDelta(t, -17.3950, *live.Stops[0].Lat, 1e-6)
		assert.InDelta(t, -66.1550, *live.Stops[0].Lng, 1e-6)
	}
}

// TestRealtimeSocket_RiderEta - a position of the rider's trip reaches the guardian's rider channel
// with an eta frame for that trip, holding only the rider's stops.
func TestRealtimeSocket_RiderEta(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	ctx := context.Background()
	if err := helper.Valkey().Client().FlushDB(ctx).Err(); err != nil {
		t.Fatalf("Valkey unreachable: %v", err)
	}
	riderID, tripID, vehicleID := guardianRiding(t, helper)
	server := httptest.NewServer(helper.Engine())
	defer server.Close()

	portal := SetupPortalTest(t)
	defer portal.Close()
	guardian := dialWS(t, server, portal)
	defer guardian.conn.CloseNow()
	rider := trackingJobs.RiderChannel(riderID)
	guardian.send(t, "subscribe", rider)
	time.Sleep(200 * time.Millisecond)

	consumer := startTestConsumer(t, helper, time.Minute)
	defer consumer.Shutdown()
	xadd(t, helper, models.StoredPoint{VehicleID: uuid.MustParse(vehicleID), RecordedAt: time.Now().UTC(), Lat: -17.3930, Lng: -66.1560})

	assert.NotNil(t, guardian.waitFrame(rider, "position", 5*time.Second), "the position reaches the rider channel")
	frame := guardian.waitFrame(rider, "eta", 5*time.Second)
	if !assert.NotNil(t, frame, "an eta frame follows the position") {
		return
	}
	var data struct {
		TripID string           `json:"trip_id"`
		Eta    []models.StopEta `json:"eta"`
	}
	raw, _ := json.Marshal(frame["data"])
	if assert.NoError(t, json.Unmarshal(raw, &data)) {
		assert.Equal(t, tripID, data.TripID)
		assert.NotEmpty(t, data.Eta)
		assert.LessOrEqual(t, len(data.Eta), 2, "the rider's stops only")
	}
}
