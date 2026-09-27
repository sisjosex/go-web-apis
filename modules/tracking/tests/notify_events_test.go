//go:build integration
// +build integration

package tracking_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"
	"golang.org/x/oauth2"

	"josex/web/config"
	coreJobs "josex/web/modules/core/jobs"
	"josex/web/modules/core/testhelpers"
	trackingJobs "josex/web/modules/tracking/jobs"
	"josex/web/modules/tracking/push"
)

// noticeRows answers the feed rows of type for rider, per recipient email.
func noticeRows(t *testing.T, helper *testhelpers.ApiTestHelper, riderID, noticeType string) map[string]string {
	t.Helper()
	rows, err := helper.DB().Query(t.Context(), `
		SELECT u.email, n.id FROM tracking.notifications n JOIN auth.users u ON u.id = n.user_id
		WHERE n.rider_id = $1 AND n.type = $2`, riderID, noticeType)
	if err != nil {
		t.Fatalf("read notices: %v", err)
	}
	defer rows.Close()
	byEmail := map[string]string{}
	for rows.Next() {
		var email, id string
		if err := rows.Scan(&email, &id); err != nil {
			t.Fatalf("scan notice: %v", err)
		}
		byEmail[email] = id
	}
	return byEmail
}

// TestNotifyEvents_TaskDone - TRACK-012 step 4: a pickup done through the outbox and the worker is one
// `boarded` feed row per guardian; the guardian who muted it gets the row and no push-send, the other
// gets both; the same event again writes nothing.
func TestNotifyEvents_TaskDone(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	routeID, riderID := createTripRoute(t, helper, "07:00")
	execSQL(t, helper, `INSERT INTO tracking.rider_contacts (rider_id, relation, name, email, user_id, is_primary)
		SELECT $1, 'guardian', u.email, u.email, u.id, FALSE FROM auth.users u
		WHERE u.email IN ('portal@test.local', 'portal-unlinked@test.local')`, riderID)
	execSQL(t, helper, `INSERT INTO tracking.notification_settings (user_id, rider_id, type)
		SELECT u.id, $1, 'boarded' FROM auth.users u WHERE u.email = 'portal-unlinked@test.local'`, riderID)
	t.Cleanup(func() {
		execSQL(t, helper, `DELETE FROM tracking.rider_contacts WHERE rider_id = $1`, riderID)
		execSQL(t, helper, `DELETE FROM tracking.notification_settings WHERE rider_id = $1`, riderID)
		execSQL(t, helper, `DELETE FROM tracking.notifications WHERE rider_id = $1`, riderID)
	})
	today := dayOffset(t, "UTC", 0)
	materialiseRoute(t, helper, routeID, today, today)
	tripID, _ := tripOn(t, helper, routeID, today, "07:00")

	relay, inspector := startTestRelay(t, helper, TestTenantID)
	defer relay.Shutdown()
	defer inspector.Close()
	registry := coreJobs.NewRegistry()
	lister := func(context.Context) ([]coreJobs.Tenant, error) { return []coreJobs.Tenant{testTenant()}, nil }
	notify := trackingJobs.NewNotifier(helper.DB(), coreJobs.NewClient(helper.Valkey()), config.ModularAppConfig.Tracking)
	registry.Handle(trackingJobs.TaskTripChanged, trackingJobs.TripChangedHandler(helper.DB(), lister, nil, nil, notify))
	worker, err := coreJobs.StartWorker(helper.Valkey(), registry, 2)
	if err != nil {
		t.Fatalf("worker: %v", err)
	}
	defer worker.Shutdown()

	detail := tripRequest(t, helper, "POST", "/tracking/trips/"+tripID+"/start", nil, http.StatusOK)
	tripRequest(t, helper, "POST", "/tracking/trip-stop-tasks/"+taskOf(t, detail, riderID, "pickup")+"/done",
		map[string]interface{}{"client_op_id": uuid.NewString()}, http.StatusOK)

	var boarded map[string]string
	waitFor(t, 5*time.Second, "a boarded row per guardian", func() bool {
		boarded = noticeRows(t, helper, riderID, "boarded")
		return len(boarded) == 2
	})
	assert.Len(t, noticeRows(t, helper, riderID, "trip_started"), 2, "the start is a notice too")

	info, err := inspector.GetTaskInfo(coreJobs.QueueCritical, boarded["portal@test.local"])
	assert.NoError(t, err, "the guardian who did not mute it gets a push-send")
	if info != nil {
		assertPushedToTrip(t, helper, info.Payload, tripID, riderID)
	}
	_, err = inspector.GetTaskInfo(coreJobs.QueueCritical, boarded["portal-unlinked@test.local"])
	assert.ErrorIs(t, err, asynq.ErrTaskNotFound, "the guardian who muted it gets the row only")

	// The same event again — what an at-least-once redelivery runs — writes nothing.
	var replayed int
	if err := helper.DB().QueryRow(t.Context(), `
		SELECT count(*) FROM tracking.sp_notify_event($1, 'trip.changed', (
			SELECT o.payload FROM tracking.outbox o
			WHERE o.topic = 'trip.changed' AND o.payload->>'trip_id' = $2 AND o.payload->>'type' = 'done'), NULL)`,
		TestTenantID, tripID).Scan(&replayed); err != nil {
		t.Fatalf("replay: %v", err)
	}
	assert.Equal(t, 0, replayed)
	assert.Len(t, noticeRows(t, helper, riderID, "boarded"), 2)
}

// assertPushedToTrip sends an enqueued push-send through a fake FCM - MOBILE-004 step 1: the message
// names the tenant for the tap, shows on the `trip` channel at high priority, and is tagged with the
// trip and rider so the next notice of that trip replaces it.
func assertPushedToTrip(t *testing.T, helper *testhelpers.ApiTestHelper, payload []byte, tripID, riderID string) {
	t.Helper()
	RegisterTestPhone(t, helper, portalUserID(t, helper, "portal@test.local"), "tok-trip")
	fcm := &fakeFCM{status: http.StatusOK, body: `{"name":"projects/test-project/messages/1"}`}
	server := httptest.NewServer(fcm)
	defer server.Close()
	pusher := push.NewFCM("test-project", oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "t"}), server.URL)
	assert.NoError(t, trackingJobs.PushSendHandler(helper.DB(), pusher)(t.Context(), asynq.NewTask(trackingJobs.TaskPushSend, payload)))

	var sent struct {
		Message struct {
			Data    map[string]string `json:"data"`
			Android struct {
				Priority     string `json:"priority"`
				CollapseKey  string `json:"collapse_key"`
				Notification struct {
					Tag       string `json:"tag"`
					ChannelID string `json:"channel_id"`
				} `json:"notification"`
			} `json:"android"`
			APNS struct {
				Headers map[string]string `json:"headers"`
			} `json:"apns"`
		} `json:"message"`
	}
	raw, _ := fcm.last.Load().(string)
	assert.NoError(t, json.Unmarshal([]byte(raw), &sent))
	assert.Equal(t, "test-tenant", sent.Message.Data["tenant_slug"])
	assert.Equal(t, riderID, sent.Message.Data["rider_id"])
	assert.Equal(t, "HIGH", sent.Message.Android.Priority)
	assert.Equal(t, "trip", sent.Message.Android.Notification.ChannelID)
	rider := uuid.MustParse(riderID)
	collapse := uuid.NewSHA1(uuid.MustParse(tripID), rider[:]).String()
	assert.Equal(t, collapse, sent.Message.Android.Notification.Tag, "a later notice of the trip replaces it")
	assert.Equal(t, collapse, sent.Message.Android.CollapseKey)
	assert.LessOrEqual(t, len(sent.Message.APNS.Headers["apns-collapse-id"]), 64, "APNs refuses a longer collapse id")
}
