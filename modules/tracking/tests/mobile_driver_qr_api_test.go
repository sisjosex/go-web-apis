//go:build integration
// +build integration

package tracking_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"josex/web/modules/core/testhelpers"
	"josex/web/modules/tracking/models"
)

// ============================================================================
// RIDER CARD AND INCIDENTS (TRACK-027)
// ============================================================================

// pickupOf answers the trip's first pickup task: its rider is the rider driverTrip assigned.
func pickupOf(t *testing.T, trip models.TripDetail) models.TripTask {
	t.Helper()
	for _, stop := range trip.Stops {
		for _, task := range stop.Tasks {
			if task.Kind == "pickup" {
				return task
			}
		}
	}
	t.Fatalf("no pickup on trip %s", trip.ID)
	return models.TripTask{}
}

// qrToken reads (GET) or rotates (POST) a rider's card code as the operator.
func qrToken(t *testing.T, helper *testhelpers.ApiTestHelper, method, riderID string) string {
	t.Helper()
	w := helper.DoRequest(method, "/tracking/riders/"+riderID+"/qr-token", nil, map[string]string{})
	if w.Code != http.StatusOK {
		t.Fatalf("%s qr-token: expected 200, got %d: %s", method, w.Code, w.Body.String())
	}
	var body models.RiderQRToken
	decodeBody(t, w, &body)
	return body.QRToken
}

// resolve scans a card code as the driver, asserting the status.
func resolve(t *testing.T, driver *testhelpers.ApiTestHelper, code string, want int) models.DriverRider {
	t.Helper()
	w := driver.DoRequest("GET", "/mobile/driver/riders/"+code, nil, map[string]string{})
	var body models.DriverRider
	if assert.Equal(t, want, w.Code, w.Body.String()) && want == http.StatusOK {
		decodeBody(t, w, &body)
	}
	if want == http.StatusNotFound {
		assert.Contains(t, w.Body.String(), "tracking.rider.not-found")
	}
	return body
}

// ---- resolve ----

// TestDriverResolveRider_PendingPickup - a card of a rider on the driver's trip: no task while the trip
// is planned, the rider's pickup once the driver starts it; a code typed in lowercase with blanks
// resolves the same.
func TestDriverResolveRider_PendingPickup(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	trip := driverTrip(t, helper, 1, false)
	pickup := pickupOf(t, trip)
	riderID := pickup.SubjectID.String()
	driver := SetupDriverTest(t)
	defer driver.Close()

	token := qrToken(t, helper, "GET", riderID)
	assert.Len(t, token, 32)
	assert.Equal(t, token, strings.ToUpper(token))

	planned := resolve(t, driver, token, http.StatusOK)
	assert.Equal(t, riderID, planned.Rider.ID.String())
	assert.Equal(t, "Test Rider", planned.Rider.Name)
	assert.Nil(t, planned.Task)

	tripRequest(t, driver, "POST", "/mobile/driver/trips/"+trip.ID.String()+"/start", nil, http.StatusOK)
	typed := strings.ToLower(token[:16]) + "%20" + strings.ToLower(token[16:])
	started := resolve(t, driver, typed, http.StatusOK)
	if assert.NotNil(t, started.Task) {
		assert.Equal(t, pickup.ID, started.Task.ID)
		assert.Equal(t, trip.ID, started.Task.TripID)
		assert.Equal(t, "pickup", started.Task.Kind)
		assert.Equal(t, "pending", started.Task.Status)
	}
}

// TestDriverResolveRider_RotatedAndForeign - a rotated card resolves nothing and the new one does; a
// token of another tenant's rider is the same 404 as an unknown one.
func TestDriverResolveRider_RotatedAndForeign(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	trip := driverTrip(t, helper, 1, true)
	riderID := pickupOf(t, trip).SubjectID.String()
	driver := SetupDriverTest(t)
	defer driver.Close()

	old := qrToken(t, helper, "GET", riderID)
	rotated := qrToken(t, helper, "POST", riderID)
	assert.NotEqual(t, old, rotated)
	assert.Equal(t, rotated, qrToken(t, helper, "GET", riderID))
	resolve(t, driver, old, http.StatusNotFound)
	assert.Equal(t, riderID, resolve(t, driver, rotated, http.StatusOK).Rider.ID.String())

	orgID := uuid.New()
	execSQL(t, helper, `INSERT INTO tracking.organizations (id, tenant_id, kind, name, timezone, is_active)
		VALUES ($1, $2, 'other', 'Foreign school', 'UTC', true)`, orgID, uuid.New())
	t.Cleanup(func() { execSQL(t, helper, `DELETE FROM tracking.organizations WHERE id = $1`, orgID) })
	var foreign string
	if err := helper.DB().QueryRow(context.Background(), `INSERT INTO tracking.riders (organization_id, first_name, last_name)
		VALUES ($1, 'Foreign', 'Rider') RETURNING qr_token`, orgID).Scan(&foreign); err != nil {
		t.Fatalf("foreign rider: %v", err)
	}
	resolve(t, driver, foreign, http.StatusNotFound)
	resolve(t, driver, "NOTACARDCODE", http.StatusNotFound)

	w := helper.DoRequest("GET", "/tracking/riders/"+uuid.NewString()+"/qr-token", nil, map[string]string{})
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
}

// TestDriverFindRiders_NameOrCode - the search finds the driver's rider by part of their name and by
// their code, never a rider off the driver's trip; one character is a 400.
func TestDriverFindRiders_NameOrCode(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	trip := driverTrip(t, helper, 1, true)
	pickup := pickupOf(t, trip)
	riderID := pickup.SubjectID.String()
	execSQL(t, helper, `UPDATE tracking.riders SET first_name = 'Zoraida', last_name = 'Quispe' WHERE id = $1`, riderID)
	offTrip := CreateTestRider(t, helper)
	execSQL(t, helper, `UPDATE tracking.riders SET first_name = 'Zoraida', last_name = 'Mamani' WHERE id = $1`, offTrip)
	t.Cleanup(func() { execSQL(t, helper, `DELETE FROM tracking.riders WHERE id = $1`, offTrip) })
	driver := SetupDriverTest(t)
	defer driver.Close()

	find := func(q string) []models.DriverRider {
		t.Helper()
		w := driver.DoRequest("GET", "/mobile/driver/riders?q="+q, nil, map[string]string{})
		if w.Code != http.StatusOK {
			t.Fatalf("find %q: expected 200, got %d: %s", q, w.Code, w.Body.String())
		}
		var body models.DriverFindRidersResponse
		decodeBody(t, w, &body)
		return body.Riders
	}

	for _, q := range []string{"zorai", "quispe", strings.ToLower(qrToken(t, helper, "GET", riderID))} {
		riders := find(q)
		if assert.Len(t, riders, 1, q) {
			assert.Equal(t, riderID, riders[0].Rider.ID.String())
			if assert.NotNil(t, riders[0].Task) {
				assert.Equal(t, pickup.ID, riders[0].Task.ID)
			}
		}
	}
	assert.Empty(t, find("mamani"))
	assert.Empty(t, find(strings.ToLower(qrToken(t, helper, "GET", offTrip))))

	w := driver.DoRequest("GET", "/mobile/driver/riders?q=z", nil, map[string]string{})
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
}

// ---- incidents ----

// TestDriverIncident_AlertAndEvent - an emergency on the driver's trip: 201 with a critical alert on
// the trip's route and vehicle at the phone's position, one incident trip_events row, one trip.changed
// outbox row, and one more active alert on the route's status.
func TestDriverIncident_AlertAndEvent(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	trip := driverTrip(t, helper, 1, true)
	driver := SetupDriverTest(t)
	defer driver.Close()

	activeAlerts := func() int32 {
		t.Helper()
		w := helper.DoRequest("GET", "/tracking/routes/"+trip.RouteID.String()+"/status", nil, map[string]string{})
		if w.Code != http.StatusOK {
			t.Fatalf("route status: expected 200, got %d: %s", w.Code, w.Body.String())
		}
		var status models.RouteRealtimeStatusResponse
		decodeBody(t, w, &status)
		return status.ActiveAlerts
	}
	before := activeAlerts()

	w := driver.DoRequest("POST", "/mobile/driver/incidents", map[string]interface{}{
		"trip_id": trip.ID, "alert_type": "emergency", "message": "Rider fainted", "lat": -17.3895, "lng": -66.1568,
	}, map[string]string{})
	if !assert.Equal(t, http.StatusCreated, w.Code, w.Body.String()) {
		return
	}
	var body models.DriverIncidentResponse
	decodeBody(t, w, &body)
	alert := body.Alert
	assert.Equal(t, trip.ID, alert.TripID)
	assert.Equal(t, trip.RouteID, alert.RouteID)
	assert.Equal(t, trip.VehicleID, alert.VehicleID)
	assert.Equal(t, "emergency", alert.AlertType)
	assert.Equal(t, "critical", alert.Severity)
	assert.Equal(t, "Rider fainted", alert.Message)
	assert.Equal(t, "active", alert.Status)
	assert.InDelta(t, -17.3895, alert.Lat, 1e-9)
	assert.InDelta(t, -66.1568, alert.Lng, 1e-9)

	assert.Equal(t, 1, countRows(t, helper, `SELECT count(*) FROM tracking.route_alerts
		WHERE trip_id = $1 AND abs(ST_Y(location::geometry) + 17.3895) < 1e-9`, trip.ID))
	assert.Equal(t, 1, countRows(t, helper, `SELECT count(*) FROM tracking.trip_events
		WHERE trip_id = $1 AND type = 'incident' AND payload->>'alert_id' = $2`, trip.ID, alert.ID.String()))
	assert.Equal(t, 1, tripChangedRows(t, helper, trip.ID.String(), "incident"))
	assert.Equal(t, before+1, activeAlerts())
}

// TestDriverIncident_NotMyTrip - another driver's trip is 404 trip.not-found and writes nothing; a
// type the phone may not report, or no position, is a 400.
func TestDriverIncident_NotMyTrip(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	driverTrip(t, helper, 1, true)
	_, _, otherID := todaysTrip(t, helper)
	driver := SetupDriverTest(t)
	defer driver.Close()

	w := driver.DoRequest("POST", "/mobile/driver/incidents", map[string]interface{}{
		"trip_id": otherID, "alert_type": "breakdown", "lat": -17.39, "lng": -66.15,
	}, map[string]string{})
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "tracking.trip.not-found")
	assert.Equal(t, 0, countRows(t, helper, `SELECT count(*) FROM tracking.route_alerts WHERE trip_id = $1`, otherID))

	for _, bad := range []map[string]interface{}{
		{"trip_id": otherID, "alert_type": "cancellation", "lat": -17.39, "lng": -66.15},
		{"trip_id": otherID, "alert_type": "delay"},
	} {
		w := driver.DoRequest("POST", "/mobile/driver/incidents", bad, map[string]string{})
		assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	}
}
