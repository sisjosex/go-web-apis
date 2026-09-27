//go:build integration
// +build integration

package tracking_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"josex/web/modules/core/testhelpers"
	"josex/web/modules/tracking/models"
)

// ============================================================================
// GUARDIAN'S NOTICES (TRACK-012)
// ============================================================================

// portalUserID is the seeded guardian's account id.
func portalUserID(t *testing.T, helper *testhelpers.ApiTestHelper, email string) string {
	t.Helper()
	var id string
	if err := helper.DB().QueryRow(t.Context(), `SELECT id FROM auth.users WHERE email = $1`, email).Scan(&id); err != nil {
		t.Fatalf("user %s: %v", email, err)
	}
	return id
}

// seedNotice writes one feed row as sp_notify_event would.
func seedNotice(t *testing.T, helper *testhelpers.ApiTestHelper, userID, riderID, noticeType, dedupe string) {
	t.Helper()
	execSQL(t, helper, `INSERT INTO tracking.notifications (user_id, rider_id, type, payload, dedupe_key)
		VALUES ($1, $2, $3, '{"route_name":"Morning Route"}', $4)`, userID, riderID, noticeType, dedupe)
}

func listNotices(t *testing.T, helper *testhelpers.ApiTestHelper, query string) models.ListNotificationsResponse {
	t.Helper()
	w := helper.DoRequest("GET", "/mobile/portal/notifications"+query, nil, map[string]string{})
	if w.Code != http.StatusOK {
		t.Fatalf("list notifications: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var page models.ListNotificationsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode notifications: %v", err)
	}
	return page
}

// TestPortalNotifications_OwnRowsOnly - the feed answers the caller's rows only, newest first, with
// the unread badge; marking read narrows the badge, and another account's id changes nothing.
func TestPortalNotifications_OwnRowsOnly(t *testing.T) {
	helper := SetupPortalTest(t)
	defer helper.Close()
	guardian := portalUserID(t, helper, "portal@test.local")
	stranger := portalUserID(t, helper, "portal-unlinked@test.local")
	t.Cleanup(func() {
		execSQL(t, helper, `DELETE FROM tracking.notifications WHERE user_id = ANY($1::UUID[])`, []string{guardian, stranger})
	})
	seedNotice(t, helper, guardian, TestRiderJohnID, "trip_started", "a")
	seedNotice(t, helper, guardian, TestRiderJohnID, "boarded", "b")
	seedNotice(t, helper, stranger, TestRiderJaneID, "trip_started", "c")

	page := listNotices(t, helper, "")
	assert.Equal(t, int64(2), page.TotalCount)
	assert.Equal(t, int64(2), page.UnreadCount)
	if assert.Len(t, page.Notifications, 2) {
		for _, n := range page.Notifications {
			assert.Equal(t, TestRiderJohnID, n.RiderID.String(), "only the guardian's own rows")
			assert.NotEmpty(t, n.RiderName)
		}
	}

	var strangerRow string
	if err := helper.DB().QueryRow(t.Context(), `SELECT id FROM tracking.notifications WHERE user_id = $1`, stranger).Scan(&strangerRow); err != nil {
		t.Fatalf("stranger row: %v", err)
	}
	w := helper.DoRequest("POST", "/mobile/portal/notifications/read",
		map[string]interface{}{"ids": []string{page.Notifications[0].ID.String(), strangerRow}}, map[string]string{})
	assert.Equal(t, http.StatusNoContent, w.Code, w.Body.String())

	unread := listNotices(t, helper, "?unread=true")
	assert.Equal(t, int64(1), unread.TotalCount)
	assert.Equal(t, int64(1), unread.UnreadCount)
	var strangerRead bool
	if err := helper.DB().QueryRow(t.Context(), `SELECT read_at IS NOT NULL FROM tracking.notifications WHERE id = $1`, strangerRow).Scan(&strangerRead); err != nil {
		t.Fatalf("stranger row: %v", err)
	}
	assert.False(t, strangerRead, "another account's row stays unread")

	w = helper.DoRequest("POST", "/mobile/portal/notifications/read", map[string]interface{}{"all": true}, map[string]string{})
	assert.Equal(t, http.StatusNoContent, w.Code, w.Body.String())
	assert.Equal(t, int64(0), listNotices(t, helper, "?page=5").UnreadCount, "a page past the end still carries the badge")
}

// TestPortalNotificationSettings_Scope - the settings list the guardian's riders; muting one of them
// round-trips, and a rider the account is not a contact of is 404 with nothing saved.
func TestPortalNotificationSettings_Scope(t *testing.T) {
	helper := SetupPortalTest(t)
	defer helper.Close()
	guardian := portalUserID(t, helper, "portal@test.local")
	t.Cleanup(func() { execSQL(t, helper, `DELETE FROM tracking.notification_settings WHERE user_id = $1`, guardian) })

	w := helper.DoRequest("PUT", "/mobile/portal/notification-settings", []map[string]interface{}{
		{"rider_id": TestRiderJohnID, "muted": []string{"approaching", "boarded"}},
	}, map[string]string{})
	if w.Code != http.StatusOK {
		t.Fatalf("put settings: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var settings []models.NotificationSetting
	if err := json.Unmarshal(w.Body.Bytes(), &settings); err != nil {
		t.Fatalf("decode settings: %v", err)
	}
	if assert.Len(t, settings, 1) {
		assert.Equal(t, TestRiderJohnID, settings[0].RiderID.String())
		assert.Equal(t, []string{"approaching", "boarded"}, settings[0].Muted)
	}

	w = helper.DoRequest("PUT", "/mobile/portal/notification-settings", []map[string]interface{}{
		{"rider_id": TestRiderJohnID, "muted": []string{}},
		{"rider_id": TestRiderJaneID, "muted": []string{"delay"}},
	}, map[string]string{})
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())

	w = helper.DoRequest("GET", "/mobile/portal/notification-settings", nil, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &settings))
	if assert.Len(t, settings, 1) {
		assert.Equal(t, []string{"approaching", "boarded"}, settings[0].Muted, "the refused write saved nothing")
	}

	w = helper.DoRequest("PUT", "/mobile/portal/notification-settings", []map[string]interface{}{
		{"rider_id": TestRiderJohnID, "muted": []string{"teleported"}},
	}, map[string]string{})
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())

	// MOBILE-004: a rider sent without `muted` — what the phone sends for "every notice on" — clears it.
	w = helper.DoRequest("PUT", "/mobile/portal/notification-settings", []map[string]interface{}{
		{"rider_id": TestRiderJohnID},
	}, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &settings))
	if assert.Len(t, settings, 1) {
		assert.Empty(t, settings[0].Muted, "no muted list is every notice on")
	}

	// The operator's account is not the portal level.
	admin := SetupTrackingTest(t)
	defer admin.Close()
	w = admin.DoRequest("GET", "/mobile/portal/notification-settings", nil, map[string]string{})
	assert.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
}
