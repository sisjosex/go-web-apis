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
	"github.com/stretchr/testify/assert"

	coreJobs "josex/web/modules/core/jobs"
	"josex/web/modules/core/testhelpers"
	trackingJobs "josex/web/modules/tracking/jobs"
	"josex/web/modules/tracking/models"
)

// ============================================================================
// ALERTS LIST AND RESOLVE, TRIP EVENT LOG (TRACK-004)
// ============================================================================

// raiseAlert posts an office alert on routeID and answers its id.
func raiseAlert(t *testing.T, helper *testhelpers.ApiTestHelper, routeID, alertType, severity, title string) string {
	t.Helper()
	w := helper.DoRequest("POST", "/tracking/alerts", map[string]interface{}{
		"route_id": routeID, "alert_type": alertType, "severity": severity, "title": title, "message": "Raised by a test",
	}, map[string]string{})
	if w.Code != http.StatusCreated {
		t.Fatalf("raise alert: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var body models.AlertCreatedResponse
	decodeBody(t, w, &body)
	return body.AlertID.String()
}

func listAlerts(t *testing.T, helper *testhelpers.ApiTestHelper, query string) models.ListAlertsResponse {
	t.Helper()
	w := helper.DoRequest("GET", "/tracking/alerts?"+query, nil, map[string]string{})
	if w.Code != http.StatusOK {
		t.Fatalf("list alerts: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var body models.ListAlertsResponse
	decodeBody(t, w, &body)
	return body
}

func alertChangedRows(t *testing.T, helper *testhelpers.ApiTestHelper, alertID, changeType string) int {
	t.Helper()
	return countRows(t, helper, `SELECT count(*) FROM tracking.outbox
		WHERE topic = 'alert.changed' AND payload->>'alert_id' = $1 AND payload->>'type' = $2`, alertID, changeType)
}

func routeActiveAlerts(t *testing.T, helper *testhelpers.ApiTestHelper, routeID string) int32 {
	t.Helper()
	w := helper.DoRequest("GET", "/tracking/routes/"+routeID+"/status", nil, map[string]string{})
	if w.Code != http.StatusOK {
		t.Fatalf("route status: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var status models.RouteRealtimeStatusResponse
	decodeBody(t, w, &status)
	return status.ActiveAlerts
}

// TestRouteAlerts_ListFilters - one route's alerts newest first with the route and who raised them;
// severity and status narrow the page, page_size pages it, and malformed filters are a 400.
func TestRouteAlerts_ListFilters(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	routeID := CreateTestRoute(t, helper)
	first := raiseAlert(t, helper, routeID, "delay", "high", "Traffic at the bridge")
	second := raiseAlert(t, helper, routeID, "other", "low", "Road works")

	page := listAlerts(t, helper, "route_id="+routeID)
	if assert.Len(t, page.Alerts, 2) {
		assert.Equal(t, second, page.Alerts[0].ID.String(), "newest first")
		assert.Equal(t, first, page.Alerts[1].ID.String())
		assert.NotEmpty(t, page.Alerts[0].RouteName)
		assert.Equal(t, "active", page.Alerts[0].Status)
		if assert.NotNil(t, page.Alerts[0].CreatedByName, "the raiser is recorded") {
			assert.NotEmpty(t, *page.Alerts[0].CreatedByName)
		}
	}
	assert.Equal(t, int64(2), page.TotalCount)
	assert.Equal(t, 1, page.Page)
	assert.Equal(t, 20, page.PageSize)

	high := listAlerts(t, helper, "route_id="+routeID+"&severity=high")
	if assert.Len(t, high.Alerts, 1) {
		assert.Equal(t, first, high.Alerts[0].ID.String())
	}
	assert.Empty(t, listAlerts(t, helper, "route_id="+routeID+"&status=resolved").Alerts)
	paged := listAlerts(t, helper, "route_id="+routeID+"&page=2&page_size=1")
	if assert.Len(t, paged.Alerts, 1) {
		assert.Equal(t, first, paged.Alerts[0].ID.String())
	}
	assert.Equal(t, int64(2), paged.TotalCount)

	for _, bad := range []string{"status=open", "severity=urgent", "route_id=nope", "page_size=101"} {
		w := helper.DoRequest("GET", "/tracking/alerts?"+bad, nil, map[string]string{})
		assert.Equal(t, http.StatusBadRequest, w.Code, bad)
	}
}

// TestRouteAlerts_Resolve - a resolve answers the row resolved with who did it, drops the route's
// active count and leaves the Active list; a second resolve is 409, an unknown id 404; raising and
// resolving each leave one alert.changed row.
func TestRouteAlerts_Resolve(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	routeID := CreateTestRoute(t, helper)
	alertID := raiseAlert(t, helper, routeID, "breakdown", "high", "Flat tyre")
	assert.Equal(t, 1, alertChangedRows(t, helper, alertID, "raised"))
	before := routeActiveAlerts(t, helper, routeID)

	w := helper.DoRequest("PATCH", "/tracking/alerts/"+alertID+"/resolve", nil, map[string]string{})
	if !assert.Equal(t, http.StatusOK, w.Code, w.Body.String()) {
		return
	}
	var resolved models.RouteAlert
	decodeBody(t, w, &resolved)
	assert.Equal(t, "resolved", resolved.Status)
	assert.NotNil(t, resolved.ResolvedAt)
	assert.NotNil(t, resolved.ResolvedBy)
	assert.NotNil(t, resolved.ResolvedByName)
	assert.Equal(t, before-1, routeActiveAlerts(t, helper, routeID))
	assert.Empty(t, listAlerts(t, helper, "route_id="+routeID).Alerts, "no longer in the Active list")
	assert.Len(t, listAlerts(t, helper, "route_id="+routeID+"&status=all").Alerts, 1)
	assert.Equal(t, 1, alertChangedRows(t, helper, alertID, "resolved"))

	again := helper.DoRequest("PATCH", "/tracking/alerts/"+alertID+"/resolve", nil, map[string]string{})
	assert.Equal(t, http.StatusConflict, again.Code, again.Body.String())
	assert.Contains(t, again.Body.String(), "tracking.alert.already-resolved")
	assert.Equal(t, 1, alertChangedRows(t, helper, alertID, "resolved"), "a refused resolve writes nothing")

	unknown := helper.DoRequest("PATCH", "/tracking/alerts/"+uuid.NewString()+"/resolve", nil, map[string]string{})
	assert.Equal(t, http.StatusNotFound, unknown.Code, unknown.Body.String())
	assert.Contains(t, unknown.Body.String(), "tracking.alert.not-found")
	bad := helper.DoRequest("PATCH", "/tracking/alerts/nope/resolve", nil, map[string]string{})
	assert.Equal(t, http.StatusBadRequest, bad.Code)
}

// TestRouteAlerts_Isolation - another tenant reaches neither an alert nor a trip's log, and the
// organization portal reaches neither endpoint.
func TestRouteAlerts_Isolation(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	ctx := context.Background()
	routeID := CreateTestRoute(t, helper)
	alertID := raiseAlert(t, helper, routeID, "delay", "medium", "Late start")
	_, _, tripID := todaysTrip(t, helper)
	otherTenant := uuid.NewString()

	_, err := helper.DB().Execute(ctx, `SELECT * FROM tracking.sp_resolve_route_alert($1, $2, NULL)`, otherTenant, alertID)
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "alert.not-found")
	}
	assert.Equal(t, 0, countRows(t, helper, `SELECT count(*) FROM tracking.sp_list_route_alerts($1, NULL, NULL, NULL, 1, 100)`, otherTenant))
	_, err = helper.DB().Execute(ctx, `SELECT * FROM tracking.sp_list_trip_events($1, $2)`, otherTenant, tripID)
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "trip.not-found")
	}
	assert.Equal(t, "active", listAlerts(t, helper, "route_id="+routeID).Alerts[0].Status, "still active")

	organization := SetupOrganizationTest(t)
	defer organization.Close()
	for _, req := range [][2]string{
		{"GET", "/tracking/alerts"},
		{"PATCH", "/tracking/alerts/" + alertID + "/resolve"},
		{"GET", "/tracking/trips/" + tripID + "/events"},
	} {
		w := organization.DoRequest(req[0], req[1], nil, map[string]string{})
		assert.Equal(t, http.StatusForbidden, w.Code, req[1])
	}
}

// TestTripEvents_Order - a start and a task done are two events oldest first, each with who made it;
// the driver's incident adds a third and an alert.changed row; an unknown trip is 404.
func TestTripEvents_Order(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	trip := driverTrip(t, helper, 1, false)
	tripID := trip.ID.String()

	events := func() []models.TripEvent {
		t.Helper()
		w := helper.DoRequest("GET", "/tracking/trips/"+tripID+"/events", nil, map[string]string{})
		if w.Code != http.StatusOK {
			t.Fatalf("trip events: expected 200, got %d: %s", w.Code, w.Body.String())
		}
		var body []models.TripEvent
		decodeBody(t, w, &body)
		return body
	}
	assert.Empty(t, events(), "nothing has happened yet")

	detail := tripRequest(t, helper, "POST", "/tracking/trips/"+tripID+"/start", nil, http.StatusOK)
	tripRequest(t, helper, "POST", "/tracking/trip-stop-tasks/"+pickupOf(t, detail).ID.String()+"/done",
		map[string]interface{}{"client_op_id": uuid.NewString()}, http.StatusOK)
	got := events()
	if assert.Len(t, got, 2) {
		assert.Equal(t, "start", got[0].Type)
		assert.Equal(t, "done", got[1].Type)
		assert.False(t, got[1].CreatedAt.Before(got[0].CreatedAt))
		if assert.NotNil(t, got[0].CreatedByName) {
			assert.NotEmpty(t, *got[0].CreatedByName)
		}
	}

	driver := SetupDriverTest(t)
	defer driver.Close()
	w := driver.DoRequest("POST", "/mobile/driver/incidents", map[string]interface{}{
		"trip_id": trip.ID, "alert_type": "breakdown", "message": "Engine light", "lat": -17.3895, "lng": -66.1568,
	}, map[string]string{})
	if !assert.Equal(t, http.StatusCreated, w.Code, w.Body.String()) {
		return
	}
	var incident models.DriverIncidentResponse
	decodeBody(t, w, &incident)
	got = events()
	if assert.Len(t, got, 3) {
		assert.Equal(t, "incident", got[2].Type)
		var payload map[string]interface{}
		assert.NoError(t, json.Unmarshal(got[2].Payload, &payload))
		assert.Equal(t, incident.Alert.ID.String(), payload["alert_id"])
	}
	assert.Equal(t, 1, alertChangedRows(t, helper, incident.Alert.ID.String(), "raised"))
	row := listAlerts(t, helper, "route_id="+trip.RouteID.String()).Alerts
	if assert.Len(t, row, 1) {
		assert.Equal(t, trip.ID, *row[0].TripID)
		assert.InDelta(t, -17.3895, *row[0].Lat, 1e-9)
	}

	unknown := helper.DoRequest("GET", "/tracking/trips/"+uuid.NewString()+"/events", nil, map[string]string{})
	assert.Equal(t, http.StatusNotFound, unknown.Code, unknown.Body.String())
}

// TestAlertChanged_FrameOnFleet - a raise and a resolve each reach the tenant's fleet channel as an
// `alert` frame within a second, through the outbox, the relay and the worker (D1).
func TestAlertChanged_FrameOnFleet(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	ctx := context.Background()
	routeID := CreateTestRoute(t, helper)

	relay, inspector := startTestRelay(t, helper, TestTenantID)
	defer relay.Shutdown()
	defer inspector.Close()
	registry := coreJobs.NewRegistry()
	lister := func(context.Context) ([]coreJobs.Tenant, error) { return []coreJobs.Tenant{testTenant()}, nil }
	registry.Handle(trackingJobs.TaskAlertChanged, trackingJobs.AlertChangedHandler(lister, helper.Valkey()))
	worker, err := coreJobs.StartWorker(helper.Valkey(), registry, 2)
	if err != nil {
		t.Fatalf("worker: %v", err)
	}
	defer worker.Shutdown()

	sub := helper.Valkey().Client().Subscribe(ctx, trackingJobs.FleetChannel(TestTenantID))
	defer sub.Close()
	if _, err := sub.Receive(ctx); err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	waitAlertFrame := func(alertID, changeType string) time.Duration {
		t.Helper()
		started := time.Now()
		deadline := time.After(3 * time.Second)
		for {
			select {
			case msg := <-sub.Channel():
				var frame trackingJobs.Frame
				if json.Unmarshal([]byte(msg.Payload), &frame) == nil && frame.Type == trackingJobs.FrameAlert &&
					strings.Contains(string(frame.Data), alertID) && strings.Contains(string(frame.Data), changeType) {
					return time.Since(started)
				}
			case <-deadline:
				t.Fatalf("no %s alert frame for %s", changeType, alertID)
				return 0
			}
		}
	}

	started := time.Now()
	alertID := raiseAlert(t, helper, routeID, "delay", "medium", "Heavy rain")
	waitAlertFrame(alertID, "raised")
	assert.Less(t, time.Since(started), time.Second)

	started = time.Now()
	w := helper.DoRequest("PATCH", "/tracking/alerts/"+alertID+"/resolve", nil, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	waitAlertFrame(alertID, "resolved")
	assert.Less(t, time.Since(started), time.Second)
}
