//go:build integration
// +build integration

package tracking_test

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"
	"golang.org/x/oauth2"

	"josex/web/modules/core/testhelpers"
	"josex/web/modules/core/utils"
	trackingJobs "josex/web/modules/tracking/jobs"
	"josex/web/modules/tracking/push"
)

// fakeFCM answers every send with status and body, and keeps the last request body.
type fakeFCM struct {
	status int
	body   string
	calls  atomic.Int32
	last   atomic.Value
}

func (f *fakeFCM) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.calls.Add(1)
	raw, _ := io.ReadAll(r.Body)
	f.last.Store(string(raw))
	w.WriteHeader(f.status)
	_, _ = w.Write([]byte(f.body))
}

// pushSendFixture registers one phone for the seeded guardian and writes a feed row for it, and
// answers the push-send task for that row.
func pushSendFixture(t *testing.T, helper *testhelpers.ApiTestHelper, token string) (*asynq.Task, string) {
	t.Helper()
	guardian := portalUserID(t, helper, "portal@test.local")
	RegisterTestPhone(t, helper, guardian, token)
	var id string
	if err := helper.DB().QueryRow(t.Context(), `INSERT INTO tracking.notifications (user_id, rider_id, type, dedupe_key)
		VALUES ($1, $2, 'boarded', $3) RETURNING id`, guardian, TestRiderJohnID, token).Scan(&id); err != nil {
		t.Fatalf("feed row: %v", err)
	}
	t.Cleanup(func() {
		execSQL(t, helper, `DELETE FROM tracking.notifications WHERE id = $1`, id)
	})
	payload, _ := json.Marshal(trackingJobs.PushSend{
		NotificationID: id, UserID: guardian, Type: "boarded",
		Args: []string{"John Doe", "Morning Route"}, Data: map[string]string{"notification_id": id},
	})
	return asynq.NewTask(trackingJobs.TaskPushSend, payload), id
}

func pushRows(t *testing.T, helper *testhelpers.ApiTestHelper, token, notificationID string) (int, int) {
	t.Helper()
	var tokens, feed int
	if err := helper.DB().QueryRow(t.Context(), `
		SELECT (SELECT count(*) FROM auth.push_devices WHERE token = $1),
		       (SELECT count(*) FROM tracking.notifications WHERE id = $2)`, token, notificationID).Scan(&tokens, &feed); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	return tokens, feed
}

// TestPushSend_FCMAnswers - TRACK-012 step 3: a delivered message carries the loc keys and args, or
// the words written in the guardian's language when the server has them (TENANCY-005); an
// UNREGISTERED token is deleted and the task done; a 503 fails the task for asynq to retry, with the
// token and the feed row untouched.
func TestPushSend_FCMAnswers(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	fcm := &fakeFCM{}
	server := httptest.NewServer(fcm)
	defer server.Close()
	pusher := push.NewFCM("test-project", oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "t"}), server.URL)
	handler := trackingJobs.PushSendHandler(helper.DB(), pusher)

	t.Run("delivered", func(t *testing.T) {
		// No words for the type on the server: the phone localizes the loc keys (TENANCY-005 fallback).
		saved := utils.Translations
		t.Cleanup(func() { utils.Translations = saved })
		utils.Translations = map[string]map[string]string{}
		fcm.status, fcm.body = http.StatusOK, `{"name":"projects/test-project/messages/1"}`
		task, id := pushSendFixture(t, helper, "tok-ok")
		assert.NoError(t, handler(t.Context(), task))
		var sent struct {
			Message struct {
				Token   string `json:"token"`
				Android struct {
					Notification struct {
						TitleLocKey  string   `json:"title_loc_key"`
						TitleLocArgs []string `json:"title_loc_args"`
						BodyLocArgs  []string `json:"body_loc_args"`
						Tag          string   `json:"tag"`
					} `json:"notification"`
				} `json:"android"`
			} `json:"message"`
		}
		assert.NoError(t, json.Unmarshal([]byte(fcm.last.Load().(string)), &sent))
		assert.Equal(t, "tok-ok", sent.Message.Token)
		assert.Equal(t, "notice_boarded", sent.Message.Android.Notification.TitleLocKey)
		assert.Equal(t, []string{"John Doe", "Morning Route"}, sent.Message.Android.Notification.BodyLocArgs)
		assert.Equal(t, []string{"John Doe", "Morning Route"}, sent.Message.Android.Notification.TitleLocArgs, "the title names the rider too")
		assert.Equal(t, id, sent.Message.Android.Notification.Tag, "a resend replaces the notice shown")
	})

	t.Run("written in the recipient's language", func(t *testing.T) {
		// TENANCY-005 D1: with the words in tracking's lang files, the phone gets them written in the
		// guardian's own language (`auth.users.locale`, 'es' by default) and no loc keys at all.
		saved := utils.Translations
		t.Cleanup(func() { utils.Translations = saved })
		utils.Translations = map[string]map[string]string{"es": {
			"notice.boarded.title": "%1$s subió", "notice.boarded.body": "%1$s subió a %2$s.",
		}}
		fcm.status, fcm.body = http.StatusOK, `{"name":"projects/test-project/messages/2"}`
		task, _ := pushSendFixture(t, helper, "tok-written")
		assert.NoError(t, handler(t.Context(), task))
		var sent struct {
			Message struct {
				Android struct {
					Notification map[string]interface{} `json:"notification"`
				} `json:"android"`
			} `json:"message"`
		}
		assert.NoError(t, json.Unmarshal([]byte(fcm.last.Load().(string)), &sent))
		notice := sent.Message.Android.Notification
		assert.Equal(t, "John Doe subió", notice["title"])
		assert.Equal(t, "John Doe subió a Morning Route.", notice["body"])
		assert.NotContains(t, notice, "title_loc_key", "a loc key would win over the written title")
	})

	t.Run("unregistered", func(t *testing.T) {
		fcm.status = http.StatusNotFound
		fcm.body = `{"error":{"code":404,"status":"NOT_FOUND","details":[{"@type":"type.googleapis.com/google.firebase.fcm.v1.FcmError","errorCode":"UNREGISTERED"}]}}`
		task, id := pushSendFixture(t, helper, "tok-dead")
		assert.NoError(t, handler(t.Context(), task))
		tokens, feed := pushRows(t, helper, "tok-dead", id)
		assert.Equal(t, 0, tokens, "the dead token is gone")
		assert.Equal(t, 1, feed)
	})

	t.Run("outage", func(t *testing.T) {
		fcm.status, fcm.body = http.StatusServiceUnavailable, `{"error":{"code":503,"status":"UNAVAILABLE"}}`
		task, id := pushSendFixture(t, helper, "tok-later")
		err := handler(t.Context(), task)
		assert.Error(t, err, "the task fails, so asynq retries it")
		assert.False(t, errors.Is(err, asynq.SkipRetry))
		tokens, feed := pushRows(t, helper, "tok-later", id)
		assert.Equal(t, 1, tokens, "an outage keeps the token")
		assert.Equal(t, 1, feed, "and the feed row")
	})
}

// TestDayChanged_SilentPush - TRACK-030 D3: the driver's signal reaches its phone as a data-only
// message — no notification, collapsed per driver, NORMAL priority, the tenant in the data.
func TestDayChanged_SilentPush(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	fcm := &fakeFCM{status: http.StatusOK, body: `{"name":"projects/test-project/messages/1"}`}
	server := httptest.NewServer(fcm)
	defer server.Close()
	pusher := push.NewFCM("test-project", oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "t"}), server.URL)
	user := portalUserID(t, helper, "driver@test.local")
	RegisterTestPhone(t, helper, user, "tok-driver")

	payload, _ := json.Marshal(trackingJobs.DayChanged{TenantSlug: "test-company", DriverID: "d-1", UserID: user})
	assert.NoError(t, trackingJobs.DayChangedHandler(helper.DB(), nil, pusher)(t.Context(), asynq.NewTask(trackingJobs.TaskDayChanged, payload)))
	var sent struct {
		Message struct {
			Data    map[string]string `json:"data"`
			Android struct {
				CollapseKey  string          `json:"collapse_key"`
				Priority     string          `json:"priority"`
				Notification json.RawMessage `json:"notification"`
			} `json:"android"`
		} `json:"message"`
	}
	raw, _ := fcm.last.Load().(string)
	if assert.NoError(t, json.Unmarshal([]byte(raw), &sent), raw) {
		assert.Nil(t, sent.Message.Android.Notification, "data only")
		assert.Equal(t, "day:d-1", sent.Message.Android.CollapseKey)
		assert.Equal(t, "NORMAL", sent.Message.Android.Priority)
		assert.Equal(t, map[string]string{"type": "day_changed", "tenant_slug": "test-company"}, sent.Message.Data)
	}
}
