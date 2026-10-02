//go:build integration
// +build integration

package tracking_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"josex/web/config"
	authServices "josex/web/modules/auth/services"
	coreJobs "josex/web/modules/core/jobs"
	trackingJobs "josex/web/modules/tracking/jobs"
	"josex/web/modules/tracking/models"
	"josex/web/modules/tracking/realtime"
	trackingRepos "josex/web/modules/tracking/repositories"
)

// ============================================================================
// REALTIME VISIBILITY AND THE DRIVER CHANNEL (TRACK-030)
// ============================================================================

// TestRealtimeVisibility_RiderOffAfterDropoff - a position lists the rider while it rides; once its
// dropoff is done, a position no longer does, and the trip's audience keeps it only for the dropoff's
// own transition.
func TestRealtimeVisibility_RiderOffAfterDropoff(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	riderID, tripID, vehicleID := guardianRiding(t, helper)
	rider := uuid.MustParse(riderID)
	point := func() []uuid.UUID {
		rows := ingest(t, helper, pointsJSON(t, vehicleID, time.Now().UTC(), [2]float64{-17.3935, -66.1570}))
		if len(rows) != 1 {
			t.Fatalf("want one vehicle, got %d", len(rows))
		}
		return rows[0].RiderIDs
	}
	assert.Contains(t, point(), rider, "riding: the position lists the rider")

	var detail models.TripDetail
	decodeBody(t, helper.DoRequest("GET", "/tracking/trips/"+tripID, nil, map[string]string{}), &detail)
	tripRequest(t, helper, "POST", "/tracking/trip-stop-tasks/"+taskOf(t, detail, riderID, "pickup")+"/done",
		map[string]interface{}{"client_op_id": uuid.NewString()}, http.StatusOK)
	assert.Contains(t, point(), rider, "aboard: still listed")

	dropoff := taskOf(t, detail, riderID, "dropoff")
	tripRequest(t, helper, "POST", "/tracking/trip-stop-tasks/"+dropoff+"/done",
		map[string]interface{}{"client_op_id": uuid.NewString()}, http.StatusOK)
	assert.NotContains(t, point(), rider, "off: no longer listed")

	repo := trackingRepos.NewTrackingRepository(helper.DB())
	tenant, trip := uuid.MustParse(TestTenantID), uuid.MustParse(tripID)
	riders, organizations, err := repo.TripAudience(context.Background(), tenant, trip, uuid.Nil)
	if assert.NoError(t, err) {
		assert.NotContains(t, riders, rider, "a later transition does not reach the rider")
		assert.NotEmpty(t, organizations, "the organization's fleet still watches the trip")
	}
	riders, _, err = repo.TripAudience(context.Background(), tenant, trip, uuid.MustParse(dropoff))
	if assert.NoError(t, err) {
		assert.Contains(t, riders, rider, "the dropoff itself reaches the rider")
	}
}

// shortLivedToken is the helper user's access token re-signed to expire in ttl.
func shortLivedToken(t *testing.T, helper interface{ Token() string }, ttl time.Duration) string {
	t.Helper()
	claims := jwt.MapClaims{}
	if _, _, err := jwt.NewParser().ParseUnverified(helper.Token(), claims); err != nil {
		t.Fatalf("parse token: %v", err)
	}
	auth := config.ModularAppConfig.Auth
	userID, _ := uuid.Parse(claims["user_id"].(string))
	sessionID, _ := uuid.Parse(claims["session_id"].(string))
	role, _ := claims["system_role"].(string)
	token, err := authServices.NewJWTService(auth.JWTSecretKey, auth.JWTRefreshKey, ttl, auth.JWTRefreshExpiration).
		GenerateAccessToken(userID, sessionID, role)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return *token
}

// TestRealtimeSocket_ClosedAtTokenExpiry - a socket opened with a token expiring in 2 s is closed
// 4401 then.
func TestRealtimeSocket_ClosedAtTokenExpiry(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	server := httptest.NewServer(helper.Engine())
	defer server.Close()

	url := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/v1/tracking/ws?tenant_slug=test-company"
	conn, _, err := websocket.Dial(context.Background(), url, &websocket.DialOptions{
		Subprotocols: []string{"bearer", shortLivedToken(t, helper, 2*time.Second)},
	})
	if err != nil {
		t.Fatalf("dial ws: %v", err)
	}
	defer conn.CloseNow()
	opened := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _, err = conn.Read(ctx)
	assert.Equal(t, realtime.StatusTokenExpired, websocket.CloseStatus(err), "closed 4401: %v", err)
	assert.Less(t, time.Since(opened), 4*time.Second)
}

// TestRealtimeSocket_DriverDayChanged - a schedule edit on the route the driver drives today reaches
// its driver:{id} channel as one day_changed within 3 s, through the outbox, the route.changed
// handler and the grouped signal; another driver's channel, and the fleet, are refused.
func TestRealtimeSocket_DriverDayChanged(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	routeID := CreateTestRoute(t, helper)
	PublishRouteVersion(t, helper, routeID, "2026-01-01", []map[string]interface{}{
		{"stop_place_id": CentralStationID, "sequence": 1, "planned_offset_min": 0},
		{"stop_place_id": SchoolAStopID, "sequence": 2, "planned_offset_min": 30},
	})
	scheduleID := CreateRouteSchedule(t, helper, routeID, map[string]interface{}{
		"days_of_week": EveryDayMask, "start_time": "07:00", "valid_from": "2026-01-01",
	})
	driverID := CreateTestDriver(t, helper)
	otherDriverID := CreateTestDriver(t, helper)
	LinkDriverAccount(t, helper, driverID)
	SetRouteDefaultDriver(t, helper, routeID, driverID)
	today := dayOffset(t, "UTC", 0)
	materialiseRoute(t, helper, routeID, today, today)

	relay, inspector := startTestRelay(t, helper, TestTenantID)
	defer relay.Shutdown()
	defer inspector.Close()
	lister := coreJobs.NewTenantDirectory(func(context.Context) ([]coreJobs.Tenant, error) { return []coreJobs.Tenant{testTenant()}, nil })
	registry := coreJobs.NewRegistry()
	signal := trackingJobs.NewDriverSignal(helper.DB(), coreJobs.NewClient(helper.Valkey()))
	registry.Handle(trackingJobs.TaskRouteChanged, trackingJobs.RouteChangedHandler(helper.DB(), lister,
		trackingJobs.PathPlanner{Router: &stubRouter{}, FallbackKmh: 25}, signal))
	registry.Handle(trackingJobs.TaskDayChanged, trackingJobs.DayChangedHandler(helper.DB(), helper.Valkey(), nil))
	worker, err := coreJobs.StartWorker(helper.Valkey(), registry, 2)
	if err != nil {
		t.Fatalf("worker: %v", err)
	}
	defer worker.Shutdown()

	server := httptest.NewServer(helper.Engine())
	defer server.Close()
	driver := SetupDriverTest(t)
	defer driver.Close()
	socket := dialWS(t, server, driver)
	defer socket.conn.CloseNow()
	own := trackingJobs.DriverChannel(driverID)
	socket.send(t, "subscribe", own)
	for _, channel := range []string{trackingJobs.DriverChannel(otherDriverID), trackingJobs.FleetChannel(TestTenantID)} {
		socket.send(t, "subscribe", channel)
		refusal := socket.waitFrame(channel, "error", 2*time.Second)
		if assert.NotNil(t, refusal, channel) {
			assert.Equal(t, "forbidden", refusal["code"], channel)
		}
	}

	edited := time.Now()
	w := helper.DoRequest("PATCH", "/tracking/routes/"+routeID+"/schedules/"+scheduleID,
		map[string]interface{}{"start_time": "07:30"}, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	if !assert.NotNil(t, socket.waitFrame(own, trackingJobs.FrameDayChanged, 3*time.Second), "day_changed within 3 s") {
		return
	}
	assert.Less(t, time.Since(edited), 3*time.Second)
	for _, f := range socket.collect(3 * time.Second) {
		assert.NotEqual(t, trackingJobs.FrameDayChanged, f["type"], "one signal per burst")
	}
}
