//go:build integration
// +build integration

package tracking_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"josex/web/modules/core/testhelpers"
)

// ============================================================================
// BLOCKED DOCUMENTS REFUSE SERVICE (TRACK-064)
// ============================================================================

// blockSubject files an expired document of a blocks_service type against subjectID and answers the
// document's id and its type's name.
func blockSubject(t *testing.T, helper *testhelpers.ApiTestHelper, subjectType, subjectID string) (string, string) {
	t.Helper()
	typeID := CreateTestDocumentType(t, helper, subjectType, true)
	document := CreateTestDocument(t, helper, subjectType, subjectID, typeID, -1)
	return document["id"].(string), "Test " + subjectType + " document"
}

// renewDocument moves a document's expiry a year out.
func renewDocument(t *testing.T, helper *testhelpers.ApiTestHelper, documentID string) {
	t.Helper()
	w := helper.DoRequest("PATCH", "/tracking/documents/"+documentID, map[string]interface{}{
		"expires_on": time.Now().AddDate(1, 0, 0).Format("2006-01-02"),
	}, map[string]string{})
	expectStatus(t, w, http.StatusOK)
}

// expectBlocked asserts a 409 tracking.service.blocked naming field.
func expectBlocked(t *testing.T, helper *testhelpers.ApiTestHelper, method, path string, body interface{}, field string) {
	t.Helper()
	w := helper.DoRequest(method, path, body, map[string]string{})
	expectStatus(t, w, http.StatusConflict)
	resp := ParseResponse(t, w.Body.Bytes())
	assert.Equal(t, "tracking.service.blocked", resp["error"])
	assert.Equal(t, field, resp["detail"])
}

// TestServiceBlockedRefusesAssignment - D1: each way of putting a blocked bus or driver on a route
// or a day is refused, naming the field; a route already holding the bus still saves its other edits.
func TestServiceBlockedRefusesAssignment(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	vehicleID := CreateTestVehicleOf(t, helper, MainCompanyID)
	driverID := CreateTestDriver(t, helper)
	// A route that already holds the bus when its document expires.
	heldID := CreateTestRoute(t, helper)
	expectStatus(t, helper.DoRequest("PATCH", "/tracking/routes/"+heldID, map[string]interface{}{"vehicle_id": vehicleID}, map[string]string{}), http.StatusOK)
	blockSubject(t, helper, "vehicle", vehicleID)
	blockSubject(t, helper, "driver", driverID)

	routeID := CreateTestRoute(t, helper)
	expectBlocked(t, helper, "PATCH", "/tracking/routes/"+routeID, map[string]interface{}{"vehicle_id": vehicleID}, "vehicle_id")
	expectBlocked(t, helper, "PATCH", "/tracking/routes/"+routeID, map[string]interface{}{"default_driver_id": driverID}, "default_driver_id")
	expectBlocked(t, helper, "PUT", "/tracking/vehicles/"+TestBusID+"/driver", map[string]interface{}{"driver_id": driverID}, "driver_id")
	expectBlocked(t, helper, "POST", "/tracking/routes/"+routeID+"/exceptions", map[string]interface{}{
		"date_from": "2026-11-02", "date_to": "2026-11-02", "kind": "change_vehicle",
		"payload": map[string]interface{}{"vehicle_id": vehicleID},
	}, "vehicle_id")
	expectBlocked(t, helper, "POST", "/tracking/routes/"+routeID+"/exceptions", map[string]interface{}{
		"date_from": "2026-11-02", "date_to": "2026-11-02", "kind": "change_driver",
		"payload": map[string]interface{}{"driver_id": driverID},
	}, "driver_id")

	// The bus expired after it was assigned: renaming the route still saves.
	w := helper.DoRequest("PATCH", "/tracking/routes/"+heldID, map[string]interface{}{
		"route_name": "Renamed while blocked", "vehicle_id": vehicleID,
	}, map[string]string{})
	expectStatus(t, w, http.StatusOK)

	trip := driverTrip(t, helper, 1, false)
	expectBlocked(t, helper, "PATCH", "/tracking/trips/"+trip.ID.String(), map[string]interface{}{"vehicle_id": vehicleID}, "vehicle_id")
	expectBlocked(t, helper, "PATCH", "/tracking/trips/"+trip.ID.String(), map[string]interface{}{"driver_id": driverID}, "driver_id")
}

// TestServiceBlockedRoutePair - creating a destination's routes on a blocked bus is refused too.
func TestServiceBlockedRoutePair(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	vehicleID := CreateTestVehicleOf(t, helper, MainCompanyID)
	blockSubject(t, helper, "vehicle", vehicleID)

	w := helper.DoRequest("POST", "/tracking/routes", pairBody(t, helper, vehicleID), map[string]string{})
	expectStatus(t, w, http.StatusConflict)
	resp := ParseResponse(t, w.Body.Bytes())
	assert.Equal(t, "tracking.service.blocked", resp["error"])
	assert.Equal(t, "vehicle_id", resp["detail"])
}

// TestServiceBlockedTripFlagAndStart - D1 at the start and D2 on the reads: a trip whose bus carries
// an expired blocking document is flagged on the detail, the list and the driver's day (whose ETag
// moves), cannot be started from the web or the phone, and renewing the document clears all of it.
func TestServiceBlockedTripFlagAndStart(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	trip := driverTrip(t, helper, 1, false)
	driver := SetupDriverTest(t)
	defer driver.Close()
	tripPath := "/tracking/trips/" + trip.ID.String()

	assert.Nil(t, trip.ServiceBlock)
	before := driver.DoRequest("GET", "/mobile/driver/today", nil, map[string]string{})
	expectStatus(t, before, http.StatusOK)

	documentID, reason := blockSubject(t, helper, "vehicle", trip.VehicleID.String())

	detail := tripRequest(t, helper, "GET", tripPath, nil, http.StatusOK)
	if assert.NotNil(t, detail.ServiceBlock) {
		assert.Equal(t, "vehicle", detail.ServiceBlock.Subject)
		assert.Equal(t, reason, detail.ServiceBlock.Reason)
	}
	assert.Equal(t, map[string]interface{}{"subject": "vehicle", "reason": reason}, tripListRow(t, helper, trip.RouteID.String(), trip.ID.String())["service_block"])

	today := driver.DoRequest("GET", "/mobile/driver/today", nil, map[string]string{"If-None-Match": before.Header().Get("ETag")})
	expectStatus(t, today, http.StatusOK)
	trips := ParseResponse(t, today.Body.Bytes())["trips"].([]interface{})
	if assert.Len(t, trips, 1) {
		assert.Equal(t, map[string]interface{}{"subject": "vehicle", "reason": reason}, trips[0].(map[string]interface{})["service_block"])
	}

	w := helper.DoRequest("POST", tripPath+"/start", nil, map[string]string{})
	expectStatus(t, w, http.StatusConflict)
	assert.Equal(t, "tracking.trip.service-blocked", ParseResponse(t, w.Body.Bytes())["error"])
	w = driver.DoRequest("POST", "/mobile/driver/trips/"+trip.ID.String()+"/start", nil, map[string]string{})
	expectStatus(t, w, http.StatusConflict)
	assert.Equal(t, "tracking.trip.service-blocked", ParseResponse(t, w.Body.Bytes())["error"])

	renewDocument(t, helper, documentID)
	assert.Nil(t, tripRequest(t, helper, "GET", tripPath, nil, http.StatusOK).ServiceBlock)
	assert.Nil(t, tripListRow(t, helper, trip.RouteID.String(), trip.ID.String())["service_block"])
	started := tripRequest(t, helper, "POST", tripPath+"/start", nil, http.StatusOK)
	assert.Equal(t, "in_progress", started.Status)
}

// tripListRow reads one trip off its route's list of today.
func tripListRow(t *testing.T, helper *testhelpers.ApiTestHelper, routeID, tripID string) map[string]interface{} {
	t.Helper()
	w := helper.DoRequest("GET", "/tracking/trips?page_size=100&route_id="+routeID, nil, map[string]string{})
	expectStatus(t, w, http.StatusOK)
	for _, row := range ParseResponse(t, w.Body.Bytes())["trips"].([]interface{}) {
		trip := row.(map[string]interface{})
		if trip["id"] == tripID {
			return trip
		}
	}
	t.Fatalf("trip %s is not on today's list", tripID)
	return nil
}
