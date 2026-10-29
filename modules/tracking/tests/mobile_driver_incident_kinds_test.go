//go:build integration
// +build integration

package tracking_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"golang.org/x/oauth2"

	"josex/web/modules/core/testhelpers"
	"josex/web/modules/core/utils"
	trackingJobs "josex/web/modules/tracking/jobs"
	"josex/web/modules/tracking/models"
	"josex/web/modules/tracking/push"
)

// ============================================================================
// THE DRIVER'S SIX KINDS (MOBILE-037)
// ============================================================================

// reportIncident posts an incident as the seeded driver and answers the status and the alert.
func reportIncident(t *testing.T, body map[string]interface{}) (int, models.DriverIncident) {
	t.Helper()
	driver := SetupDriverTest(t)
	defer driver.Close()
	body["lat"], body["lng"] = -17.3895, -66.1568
	w := driver.DoRequest("POST", "/mobile/driver/incidents", body, map[string]string{})
	var out models.DriverIncidentResponse
	if w.Code == http.StatusCreated {
		decodeBody(t, w, &out)
	}
	return w.Code, out.Alert
}

// spanishNotices makes the server write in Spanish from tracking's own file, for the test's length.
func spanishNotices(t *testing.T) {
	t.Helper()
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
}

// alertNoticeBodies runs the alert.changed worker's notify for a raise and answers, per notice, the body
// its push is written with in Spanish.
func alertNoticeBodies(t *testing.T, helper *testhelpers.ApiTestHelper, alertID string) []string {
	t.Helper()
	rows, err := helper.DB().Query(t.Context(), `
		SELECT type, payload FROM tracking.sp_notify_event($1, 'alert.changed', jsonb_build_object('type', 'raised', 'alert_id', $2::TEXT), NULL)`,
		TestTenantID, alertID)
	if err != nil {
		t.Fatalf("notify alert: %v", err)
	}
	defer rows.Close()
	var bodies []string
	for rows.Next() {
		var kind string
		var raw []byte
		if err := rows.Scan(&kind, &raw); err != nil {
			t.Fatalf("scan notice: %v", err)
		}
		var payload map[string]any
		_ = json.Unmarshal(raw, &payload)
		textType, args := trackingJobs.PushText(kind, payload)
		_, body := push.NoticeText("es", textType, args)
		bodies = append(bodies, body)
	}
	return bodies
}

// TestDriverIncident_Kinds - a closed road 15 minutes late reaches the rider's guardian as "…llegará unos
// 15 min tarde" with its minutes stored; notify false lists the alert and tells no family; an accident is
// critical; minutes on a breakdown are a 400.
func TestDriverIncident_Kinds(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	spanishNotices(t)
	trip := driverTrip(t, helper, 1, true)
	LinkPortalGuardian(t, helper, trip.Stops[0].Tasks[0].SubjectID.String())

	code, road := reportIncident(t, map[string]interface{}{"trip_id": trip.ID, "alert_type": "road_closed", "delay_min": 15})
	if !assert.Equal(t, http.StatusCreated, code) {
		return
	}
	assert.Equal(t, "medium", road.Severity)
	assert.True(t, road.NotifyFamilies)
	if assert.NotNil(t, road.EstimatedDelayMinutes) {
		assert.Equal(t, int32(15), *road.EstimatedDelayMinutes)
	}
	if bodies := alertNoticeBodies(t, helper, road.ID.String()); assert.Len(t, bodies, 1) {
		assert.Equal(t, "El bus tomará otra ruta y llegará unos 15 min tarde", bodies[0])
	}

	code, quiet := reportIncident(t, map[string]interface{}{"trip_id": trip.ID, "alert_type": "delay", "delay_min": 10, "notify": false})
	if assert.Equal(t, http.StatusCreated, code) {
		assert.False(t, quiet.NotifyFamilies)
		listed := false
		for _, a := range listAlerts(t, helper, "status=active&page_size=100").Alerts {
			listed = listed || a.ID == quiet.ID
		}
		assert.True(t, listed, "the office still sees it")
		assert.Zero(t, notifyAlertRaised(t, helper, quiet.ID.String()), "notify false tells no family")
	}

	code, accident := reportIncident(t, map[string]interface{}{"trip_id": trip.ID, "alert_type": "accident"})
	if assert.Equal(t, http.StatusCreated, code) {
		assert.Equal(t, "critical", accident.Severity)
		assert.Nil(t, accident.EstimatedDelayMinutes)
	}

	code, _ = reportIncident(t, map[string]interface{}{"trip_id": trip.ID, "alert_type": "breakdown", "delay_min": 15})
	assert.Equal(t, http.StatusBadRequest, code, "minutes belong to a delay or a closed road")
	code, _ = reportIncident(t, map[string]interface{}{"trip_id": trip.ID, "alert_type": "delay", "delay_min": 200})
	assert.Equal(t, http.StatusBadRequest, code, "at most 120 minutes")
}

// TestClientPush_DriverIncident - the rider's school member with a phone gets one push for a closed road,
// with the kind's line and the client_alert data; a member of another organization gets nothing; an
// incident with notify false pushes to no one (MOBILE-037 D2).
func TestClientPush_DriverIncident(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	spanishNotices(t)
	fcm := &fakeFCM{status: http.StatusOK, body: `{"name":"projects/test-project/messages/1"}`}
	server := httptest.NewServer(fcm)
	defer server.Close()
	pusher := push.NewFCM("test-project", oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "t"}), server.URL)

	RegisterTestPhone(t, helper, portalUserID(t, helper, "orguser@test.local"), "tok-school")
	// A member of another client, invited as the office would.
	invite := accessPerson(newAccessEmail())
	invite["role"] = "viewer"
	w := helper.DoRequest("POST", fmt.Sprintf("/tracking/organizations/%s/members", EmptyEmployerID), invite, map[string]string{})
	if w.Code != http.StatusCreated {
		t.Fatalf("invite member: %d %s", w.Code, w.Body.String())
	}
	other := ParseResponse(t, w.Body.Bytes())["user_id"].(string)
	t.Cleanup(func() {
		helper.DoRequest("DELETE", fmt.Sprintf("/tracking/organizations/%s/members/%s", EmptyEmployerID, other), nil, map[string]string{})
	})
	RegisterTestPhone(t, helper, other, "tok-other")
	trip := driverTrip(t, helper, 1, true)
	pushAlert := func(alertID uuid.UUID) {
		t.Helper()
		if err := trackingJobs.PushClientAlert(t.Context(), helper.DB(), testTenant(), alertID, trip.ID.String(), pusher); err != nil {
			t.Fatalf("client push: %v", err)
		}
	}

	code, road := reportIncident(t, map[string]interface{}{"trip_id": trip.ID, "alert_type": "road_closed", "delay_min": 15})
	if !assert.Equal(t, http.StatusCreated, code) {
		return
	}
	pushAlert(road.ID)
	if assert.Equal(t, int32(1), fcm.calls.Load(), "the school's member alone") {
		sent := fcm.last.Load().(string)
		assert.Contains(t, sent, "tok-school")
		assert.Contains(t, sent, `"type":"client_alert"`)
		assert.Contains(t, sent, `"alert_id":"`+road.ID.String()+`"`)
		assert.Contains(t, sent, "El bus tomará otra ruta y llegará unos 15 min tarde")
	}

	_, quiet := reportIncident(t, map[string]interface{}{"trip_id": trip.ID, "alert_type": "breakdown", "notify": false})
	pushAlert(quiet.ID)
	assert.Equal(t, int32(1), fcm.calls.Load(), "notify false pushes to no one")
}
