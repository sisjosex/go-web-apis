//go:build integration
// +build integration

package tracking_test

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"josex/web/modules/core/testhelpers"
	"josex/web/modules/tracking/models"
)

// ============================================================================
// GPS DEVICE TOKEN FROM THE VEHICLE PAGE (TRACK-068)
// ============================================================================

func deviceStatus(t *testing.T, helper *testhelpers.ApiTestHelper, vehicleID string) models.GPSDeviceStatus {
	t.Helper()
	w := helper.DoRequest("GET", "/tracking/vehicles/"+vehicleID+"/gps-device", nil, map[string]string{})
	if w.Code != http.StatusOK {
		t.Fatalf("gps-device status: %d %s", w.Code, w.Body.String())
	}
	var status models.GPSDeviceStatus
	if err := json.Unmarshal(w.Body.Bytes(), &status); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	return status
}

func issueFromPage(t *testing.T, helper *testhelpers.ApiTestHelper, vehicleID string) string {
	t.Helper()
	w := helper.DoRequest("POST", "/tracking/vehicles/"+vehicleID+"/gps-device", nil, map[string]string{})
	if w.Code != http.StatusCreated {
		t.Fatalf("issue: %d %s", w.Code, w.Body.String())
	}
	var issued models.GPSDeviceIssued
	if err := json.Unmarshal(w.Body.Bytes(), &issued); err != nil || issued.Token == "" || issued.IssuedAt.IsZero() {
		t.Fatalf("issue body: %v %s", err, w.Body.String())
	}
	return issued.Token
}

// TestVehicleGPSDevice_IssueRotateRevoke - POST → 201 and the token posts 200 over OsmAnd; GET
// linked with last_seen_at; a second POST → the first token 401; DELETE → 204 and linked false; a
// vehicle of another tenant → 404.
func TestVehicleGPSDevice_IssueRotateRevoke(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	vehicleID := CreateTestVehicleOf(t, helper, MainCompanyID)
	device := testhelpers.SetupApiTest(t)
	defer device.Close()

	assert.False(t, deviceStatus(t, helper, vehicleID).Linked)
	first := issueFromPage(t, helper, vehicleID)
	w := device.DoRequest("GET", osmandURL(first, -17.39, time.Now()), nil, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	status := deviceStatus(t, helper, vehicleID)
	assert.True(t, status.Linked)
	assert.NotNil(t, status.IssuedAt)
	assert.NotNil(t, status.LastSeenAt)

	second := issueFromPage(t, helper, vehicleID)
	w = device.DoRequest("GET", osmandURL(first, -17.39, time.Now()), nil, map[string]string{})
	assert.Equal(t, http.StatusUnauthorized, w.Code, w.Body.String())
	w = device.DoRequest("GET", osmandURL(second, -17.39, time.Now()), nil, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())

	w = helper.DoRequest("DELETE", "/tracking/vehicles/"+vehicleID+"/gps-device", nil, map[string]string{})
	assert.Equal(t, http.StatusNoContent, w.Code, w.Body.String())
	assert.False(t, deviceStatus(t, helper, vehicleID).Linked)
	w = device.DoRequest("GET", osmandURL(second, -17.39, time.Now()), nil, map[string]string{})
	assert.Equal(t, http.StatusUnauthorized, w.Code, w.Body.String())

	_, _, otherVehicleID := SeedOtherTenantFleet(t, helper)
	for _, method := range []string{"GET", "POST", "DELETE"} {
		w = helper.DoRequest(method, "/tracking/vehicles/"+otherVehicleID+"/gps-device", nil, map[string]string{})
		assert.Equal(t, http.StatusNotFound, w.Code, method+": "+w.Body.String())
	}
}
