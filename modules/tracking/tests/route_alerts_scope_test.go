//go:build integration
// +build integration

package tracking_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"josex/web/modules/core/testhelpers"
	"josex/web/modules/tracking/models"
)

// ============================================================================
// TENANT-WIDE ALERTS, THRESHOLDS, THE ALERTS BOARD (TRACK-071)
// ============================================================================

// raiseTenantAlert posts an office alert with no route and answers its id; its notices are removed on
// cleanup, since it reaches every assigned rider of the tenant.
func raiseTenantAlert(t *testing.T, helper *testhelpers.ApiTestHelper, title string) models.AlertCreatedResponse {
	t.Helper()
	w := helper.DoRequest("POST", "/tracking/alerts", map[string]interface{}{
		"alert_type": "delay", "title": title, "message": "Raised for the whole operation",
	}, map[string]string{})
	if w.Code != http.StatusCreated {
		t.Fatalf("raise tenant alert: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var body models.AlertCreatedResponse
	decodeBody(t, w, &body)
	t.Cleanup(func() { DeleteAlertNotices(t, helper, body.AlertID.String()) })
	return body
}

// TestRouteAlerts_TenantWide - POST without route_id → 201 with route_id null, listed with no route,
// resolved like any other.
func TestRouteAlerts_TenantWide(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	created := raiseTenantAlert(t, helper, "Paro de transporte")
	assert.Nil(t, created.RouteID)

	var listed *models.RouteAlert
	for _, a := range listAlerts(t, helper, "page_size=100").Alerts {
		if a.ID == created.AlertID {
			listed = a
		}
	}
	if assert.NotNil(t, listed, "the tenant-wide alert is listed") {
		assert.Nil(t, listed.RouteID)
		assert.Nil(t, listed.RouteName)
	}
	w := helper.DoRequest("PATCH", "/tracking/alerts/"+created.AlertID.String()+"/resolve", nil, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
}

// TestNotifyTenantAlert_EveryRoute - a tenant-wide alert reaches the guardians of riders assigned today
// on two different routes, one row each, with no route name; an expired assignment gets nothing.
func TestNotifyTenantAlert_EveryRoute(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	_, first := createTripRoute(t, helper, "07:00")
	_, second := createTripRoute(t, helper, "08:00")
	otherRoute := CreateTestRoute(t, helper)
	expired := CreateTestRider(t, helper)
	seedRouteAssignment(t, helper, otherRoute, expired, EveryDayMask, "2026-01-01", dayOffset(t, "UTC", -1))
	addGuardians(t, helper, first, "portal@test.local", "portal-unlinked@test.local")
	addGuardians(t, helper, second, "portal@test.local")
	addGuardians(t, helper, expired, "portal@test.local")

	alert := raiseTenantAlert(t, helper, "Bloqueo en la ciudad")
	notifyAlertRaised(t, helper, alert.AlertID.String())
	got := alertNotices(t, helper, alert.AlertID.String())
	assert.Len(t, got[first], 2)
	assert.Len(t, got[second], 1)
	assert.Empty(t, got[expired])
	for _, payload := range got[first] {
		assert.NotContains(t, payload, "route_name")
		assert.Equal(t, "Bloqueo en la ciudad", payload["title"])
	}
}

// TestAlertSettings_PutGet - the defaults until saved; 10/200/4 saved and read back; 0 is a 400.
func TestAlertSettings_PutGet(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	defer ResetAlertSettings(t, helper)

	read := func() models.AlertSettings {
		t.Helper()
		w := helper.DoRequest("GET", "/tracking/alert-settings", nil, map[string]string{})
		if w.Code != http.StatusOK {
			t.Fatalf("alert settings: %d %s", w.Code, w.Body.String())
		}
		var s models.AlertSettings
		decodeBody(t, w, &s)
		return s
	}
	ResetAlertSettings(t, helper)
	assert.Equal(t, models.AlertSettings{DeviceSilentMin: 5, OffRouteM: 150, OffRouteFixes: 3}, read())

	want := models.AlertSettings{DeviceSilentMin: 10, OffRouteM: 200, OffRouteFixes: 4}
	w := helper.DoRequest("PUT", "/tracking/alert-settings", want, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, want, read())

	w = helper.DoRequest("PUT", "/tracking/alert-settings", map[string]int{"device_silent_min": 0, "off_route_m": 200, "off_route_fixes": 4}, map[string]string{})
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Equal(t, want, read(), "the refused write saved nothing")
}

// TestRouteAlerts_Board - an alert noticed to two guardians reads notified_count 2 and the driver;
// search by its plate finds it; the summary's active count does not follow the status chip.
func TestRouteAlerts_Board(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	routeID, riderID := createTripRoute(t, helper, "07:00")
	vehicleID := CreateTestVehicleOf(t, helper, MainCompanyID)
	plate := VehiclePlate(t, helper, vehicleID)
	addGuardians(t, helper, riderID, "portal@test.local", "portal-unlinked@test.local")

	w := helper.DoRequest("POST", "/tracking/alerts", map[string]interface{}{
		"route_id": routeID, "vehicle_id": vehicleID, "alert_type": "delay", "title": "Tráfico", "message": "Raised by a test",
	}, map[string]string{})
	if !assert.Equal(t, http.StatusCreated, w.Code, w.Body.String()) {
		return
	}
	var created models.AlertCreatedResponse
	decodeBody(t, w, &created)
	assert.Equal(t, 2, notifyAlertRaised(t, helper, created.AlertID.String()))

	found := listAlerts(t, helper, "search="+plate)
	if assert.Len(t, found.Alerts, 1) {
		a := found.Alerts[0]
		assert.Equal(t, created.AlertID, a.ID)
		assert.Equal(t, int32(2), a.NotifiedCount)
		assert.Equal(t, plate, *a.LicensePlate)
	}
	active := found.Summary.Active
	assert.GreaterOrEqual(t, active, int64(1))
	resolvedPage := listAlerts(t, helper, "search="+plate+"&status=resolved")
	assert.Empty(t, resolvedPage.Alerts)
	assert.Equal(t, active, resolvedPage.Summary.Active, "the summary ignores the status chip")
	assert.Equal(t, int64(0), resolvedPage.TotalCount)
}
