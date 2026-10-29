//go:build integration
// +build integration

package tracking_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"golang.org/x/oauth2"

	"josex/web/modules/core/utils"
	tenancyRepos "josex/web/modules/tenancy/repositories"
	trackingJobs "josex/web/modules/tracking/jobs"
	"josex/web/modules/tracking/models"
	"josex/web/modules/tracking/push"
)

// ============================================================================
// STAFF PUSH ON SERIOUS ALERTS (TRACK-071 D3)
// ============================================================================

// driverBreakdown reports a breakdown on the driver's trip and answers the alert's id.
func driverBreakdown(t *testing.T, trip models.TripDetail) string {
	t.Helper()
	driver := SetupDriverTest(t)
	defer driver.Close()
	w := driver.DoRequest("POST", "/mobile/driver/incidents", map[string]interface{}{
		"trip_id": trip.ID, "alert_type": "breakdown", "message": "Llanta pinchada", "lat": -17.3895, "lng": -66.1568,
	}, map[string]string{})
	if w.Code != http.StatusCreated {
		t.Fatalf("incident: %d %s", w.Code, w.Body.String())
	}
	var body models.DriverIncidentResponse
	decodeBody(t, w, &body)
	return body.Alert.ID.String()
}

// TestStaffPush_SeriousAlerts - a staff account with a phone gets one push for a driver's breakdown,
// with the alert's id and the Spanish title; a delay alert pushes nothing to the staff; an
// organization user's phone gets nothing.
func TestStaffPush_SeriousAlerts(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	fcm := &fakeFCM{status: http.StatusOK, body: `{"name":"projects/test-project/messages/1"}`}
	server := httptest.NewServer(fcm)
	defer server.Close()
	pusher := push.NewFCM("test-project", oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "t"}), server.URL)
	staff := tenancyRepos.NewTenantRepository(helper.DB()).StaffUserIDs
	pushAlert := func(alertID string) {
		t.Helper()
		if err := trackingJobs.PushStaffAlert(t.Context(), helper.DB(), testTenant(), uuid.MustParse(alertID), pusher, staff); err != nil {
			t.Fatalf("staff push: %v", err)
		}
	}

	// The words the server writes in the staff member's language ('es' by default), from tracking's file.
	saved := utils.Translations
	t.Cleanup(func() { utils.Translations = saved })
	raw, err := os.ReadFile("../lang/es.json")
	if err != nil {
		t.Fatalf("read lang: %v", err)
	}
	es := map[string]string{}
	if err := json.Unmarshal(raw, &es); err != nil {
		t.Fatalf("parse lang: %v", err)
	}
	utils.Translations = map[string]map[string]string{"es": es}

	RegisterTestPhone(t, helper, portalUserID(t, helper, "admin@test.local"), "tok-staff")
	trip := driverTrip(t, helper, 1, true)
	breakdown := driverBreakdown(t, trip)
	pushAlert(breakdown)
	if assert.Equal(t, int32(1), fcm.calls.Load()) {
		sent := fcm.last.Load().(string)
		assert.Contains(t, sent, `"alert_id":"`+breakdown+`"`)
		assert.Contains(t, sent, `"type":"staff_alert"`)
		assert.Contains(t, sent, "Avería · ")
		assert.Contains(t, sent, "tok-staff")
	}

	delay := raiseAlert(t, helper, trip.RouteID.String(), "delay", "medium", "Tráfico")
	pushAlert(delay)
	assert.Equal(t, int32(1), fcm.calls.Load(), "a delay is the families', not the staff's")

	// An organization user with a phone of its own: the next breakdown reaches the staff phone alone.
	RegisterTestPhone(t, helper, portalUserID(t, helper, "orguser@test.local"), "tok-org")
	pushAlert(driverBreakdown(t, trip))
	assert.Equal(t, int32(2), fcm.calls.Load(), "an organization user is not staff")
	assert.True(t, strings.Contains(fcm.last.Load().(string), "tok-staff"))
}
