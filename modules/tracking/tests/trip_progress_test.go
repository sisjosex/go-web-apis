//go:build integration
// +build integration

package tracking_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"
	"golang.org/x/oauth2"

	"josex/web/modules/core/testhelpers"
	trackingJobs "josex/web/modules/tracking/jobs"
	"josex/web/modules/tracking/push"
)

// ============================================================================
// THE GUARDIAN'S TRIP PROGRESS (MOBILE-024)
// ============================================================================

type progressPush struct {
	Message struct {
		Data    map[string]string `json:"data"`
		Android struct {
			CollapseKey  string          `json:"collapse_key"`
			Priority     string          `json:"priority"`
			Notification json.RawMessage `json:"notification"`
		} `json:"android"`
	} `json:"message"`
}

// runProgress runs the trip's progress task once and answers the last message the fake FCM got.
func runProgress(t *testing.T, helper *testhelpers.ApiTestHelper, fcm *fakeFCM, server *httptest.Server, tripID string) progressPush {
	t.Helper()
	pusher := push.NewFCM("test-project", oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "t"}), server.URL)
	tenant := testTenant()
	payload, _ := json.Marshal(trackingJobs.TripProgress{
		TenantID: tenant.ID, TenantSlug: tenant.Slug, DatabaseURL: tenant.DatabaseURL, TripID: tripID,
	})
	err := trackingJobs.TripProgressHandler(helper.DB(), helper.Valkey(), pusher, nil)(t.Context(),
		asynq.NewTask(trackingJobs.TaskTripProgress, payload))
	assert.NoError(t, err)
	var sent progressPush
	raw, _ := fcm.last.Load().(string)
	if raw != "" {
		assert.NoError(t, json.Unmarshal([]byte(raw), &sent), raw)
	}
	return sent
}

// TestTripProgress_PerStopToTheRidersOwn - D1: the guardian's push is data-only at high priority; waiting,
// it counts the stops up to the pickup, on board up to the drop-off; muted, nothing is sent.
func TestTripProgress_PerStopToTheRidersOwn(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	fcm := &fakeFCM{status: http.StatusOK, body: `{"name":"projects/test-project/messages/1"}`}
	server := httptest.NewServer(fcm)
	defer server.Close()

	routeID, riderID := createTripRoute(t, helper, "07:00")
	LinkPortalGuardian(t, helper, riderID)
	guardian := portalUserID(t, helper, "portal@test.local")
	RegisterTestPhone(t, helper, guardian, "tok-progress-"+uuid.NewString()[:8])
	today := dayOffset(t, "UTC", 0)
	materialiseRoute(t, helper, routeID, today, today)
	tripID, _ := tripOn(t, helper, routeID, today, "07:00")
	detail := tripRequest(t, helper, "POST", "/tracking/trips/"+tripID+"/start", nil, http.StatusOK)

	sent := runProgress(t, helper, fcm, server, tripID)
	assert.Nil(t, sent.Message.Android.Notification, "data only")
	assert.Equal(t, "HIGH", sent.Message.Android.Priority)
	assert.Equal(t, "trip_progress", sent.Message.Data["type"])
	assert.Equal(t, riderID, sent.Message.Data["rider_id"])
	assert.Equal(t, "to_pickup", sent.Message.Data["phase"])
	assert.Equal(t, "0", sent.Message.Data["stops_done"])
	assert.Equal(t, "1", sent.Message.Data["stops_total"])

	tripRequest(t, helper, "POST", "/tracking/trip-stops/"+detail.Stops[0].ID.String()+"/arrive", nil, http.StatusOK)
	tripRequest(t, helper, "POST", "/tracking/trip-stop-tasks/"+taskOf(t, detail, riderID, "pickup")+"/done",
		map[string]interface{}{"client_op_id": uuid.NewString()}, http.StatusOK)
	sent = runProgress(t, helper, fcm, server, tripID)
	assert.Equal(t, "on_board", sent.Message.Data["phase"])
	assert.Equal(t, "1", sent.Message.Data["stops_done"])
	assert.Equal(t, "2", sent.Message.Data["stops_total"])

	MuteNotice(t, helper, guardian, riderID, "trip_progress")
	before := fcm.calls.Load()
	runProgress(t, helper, fcm, server, tripID)
	assert.Equal(t, before, fcm.calls.Load(), "a muted guardian gets nothing")
}
