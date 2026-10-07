//go:build integration
// +build integration

package tracking_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"josex/web/modules/core/testhelpers"
)

// TRACK-045: a client's location is required and becomes its stop.

// createPinnedClient creates a client through the API with body's extra fields and answers it.
func createPinnedClient(t *testing.T, helper *testhelpers.ApiTestHelper, extra map[string]interface{}) map[string]interface{} {
	t.Helper()
	body := OrganizationBody(ValidOrganizationDto())
	for k, v := range extra {
		body[k] = v
	}
	w := helper.DoRequest("POST", "/tracking/organizations", body, map[string]string{})
	expectStatus(t, w, http.StatusCreated)
	return ParseResponse(t, w.Body.Bytes())
}

// TestOrganizationStop_PinRequired - a client without a pin is refused (D2).
func TestOrganizationStop_PinRequired(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	w := helper.DoRequest("POST", "/tracking/organizations", map[string]interface{}{
		"name": "Sin pin " + uuid.NewString()[:6], "kind": "school",
	}, map[string]string{})
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
}

// TestOrganizationStop_CreatedAndFollowsClient - saving a pinned client makes its stop, named after it;
// moving the pin and renaming the client move and rename that same stop (D3).
func TestOrganizationStop_CreatedAndFollowsClient(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	client := createPinnedClient(t, helper, nil)
	stopID, ok := client["stop_place_id"].(string)
	if !assert.True(t, ok, "the client answers its stop") {
		return
	}
	lat, lng := StopPlacePoint(t, helper, stopID)
	assert.InDelta(t, client["latitude"], lat, 1e-6)
	assert.InDelta(t, client["longitude"], lng, 1e-6)

	newLat, newLng := lat-0.01, lng-0.01
	newName := "Colegio movido " + uuid.NewString()[:6]
	w := helper.DoRequest("PATCH", "/tracking/organizations/"+ExtractID(t, client), map[string]interface{}{
		"name": newName, "latitude": newLat, "longitude": newLng,
	}, map[string]string{})
	expectStatus(t, w, http.StatusOK)
	assert.Equal(t, stopID, ParseResponse(t, w.Body.Bytes())["stop_place_id"], "the same stop, not a new one")

	lat, lng = StopPlacePoint(t, helper, stopID)
	assert.InDelta(t, newLat, lat, 1e-6)
	assert.InDelta(t, newLng, lng, 1e-6)
	w = helper.DoRequest("GET", "/tracking/stop-places/"+stopID, nil, map[string]string{})
	expectStatus(t, w, http.StatusOK)
	assert.Equal(t, newName, ParseResponse(t, w.Body.Bytes())["name"])
}

// TestOrganizationStop_OptOut - create_stop false leaves the client without a stop.
func TestOrganizationStop_OptOut(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	client := createPinnedClient(t, helper, map[string]interface{}{"create_stop": false})
	assert.Nil(t, client["stop_place_id"])
}

// TestOrganizationStop_RoutePairReusesIt - a route pair for the client calls at its stop as the
// outbound's last and the return's first, and adds no place.
func TestOrganizationStop_RoutePairReusesIt(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	client := createPinnedClient(t, helper, nil)
	stopID := client["stop_place_id"].(string)
	before := CountStopPlaces(t, helper)

	w := helper.DoRequest("POST", "/tracking/routes", map[string]interface{}{
		"company_id":                MainCompanyID,
		"organization_id":           ExtractID(t, client),
		"route_name":                "Ruta " + uuid.NewString()[:6],
		"arrival_time":              "07:45",
		"departure_time":            "13:00",
		"destination_stop_place_id": stopID,
	}, map[string]string{})
	expectStatus(t, w, http.StatusCreated)
	created := ParseResponse(t, w.Body.Bytes())
	ret := created["return_route"].(map[string]interface{})

	assert.Equal(t, before, CountStopPlaces(t, helper), "the pair adds no place")
	outStops := ListRouteStops(t, helper, ExtractID(t, created), "")
	retStops := ListRouteStops(t, helper, ExtractID(t, ret), "")
	if assert.Len(t, outStops, 1) && assert.Len(t, retStops, 1) {
		assert.Equal(t, stopID, outStops[0]["stop_place_id"])
		assert.Equal(t, stopID, retStops[0]["stop_place_id"])
	}
}
