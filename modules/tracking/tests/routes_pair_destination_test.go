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

// TRACK-049 D3: the route form names the destination stop, or a pin that becomes one.

// postPair creates a pair for clientID with extra body fields and answers the status, the created
// body (nil unless 201) and the raw body.
func postPair(t *testing.T, helper *testhelpers.ApiTestHelper, clientID, name string, extra map[string]interface{}) (int, map[string]interface{}, string) {
	t.Helper()
	body := map[string]interface{}{
		"company_id":      MainCompanyID,
		"organization_id": clientID,
		"route_name":      name,
		"arrival_time":    "07:45",
		"departure_time":  "13:00",
	}
	for k, v := range extra {
		body[k] = v
	}
	w := helper.DoRequest("POST", "/tracking/routes", body, map[string]string{})
	if w.Code != http.StatusCreated {
		return w.Code, nil, w.Body.String()
	}
	return w.Code, ParseResponse(t, w.Body.Bytes()), w.Body.String()
}

// TestRoutePairDestination_ChosenStop - the stop sent is the outbound's last and the return's first,
// even when the client has a stop of its own; the return is "<name> (retorno)" (D1).
func TestRoutePairDestination_ChosenStop(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	client := createPinnedClient(t, helper, nil)
	chosen := CreateTestStopPlace(t, helper)
	name := "Ruta " + uuid.NewString()[:6]
	code, created, raw := postPair(t, helper, ExtractID(t, client), name, map[string]interface{}{"destination_stop_place_id": chosen})
	if !assert.Equal(t, http.StatusCreated, code, raw) {
		return
	}
	ret := created["return_route"].(map[string]interface{})
	assert.Equal(t, name+" (retorno)", ret["route_name"])

	outStops := ListRouteStops(t, helper, ExtractID(t, created), "")
	retStops := ListRouteStops(t, helper, ExtractID(t, ret), "")
	if assert.Len(t, outStops, 1) && assert.Len(t, retStops, 1) {
		assert.Equal(t, chosen, outStops[0]["stop_place_id"])
		assert.Equal(t, chosen, retStops[0]["stop_place_id"])
	}
}

// TestRoutePairDestination_Required - a client with a pin and no stop, and no stop or pin sent, is
// refused: its pin is not turned into a stop on its own.
func TestRoutePairDestination_Required(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	client := createPinnedClient(t, helper, map[string]interface{}{"create_stop": false})
	before := CountStopPlaces(t, helper)
	code, _, raw := postPair(t, helper, ExtractID(t, client), "Ruta "+uuid.NewString()[:6], nil)
	assert.Equal(t, http.StatusBadRequest, code, raw)
	assert.Contains(t, raw, "tracking.route.destination-required")
	assert.Equal(t, before, CountStopPlaces(t, helper), "no place is added")
}

// TestRoutePairDestination_PinMakesStop - a pin sent for a client without a stop ends the route at a
// new stop named after the client.
func TestRoutePairDestination_PinMakesStop(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	client := createPinnedClient(t, helper, map[string]interface{}{"create_stop": false})
	before := CountStopPlaces(t, helper)
	code, created, raw := postPair(t, helper, ExtractID(t, client), "Ruta "+uuid.NewString()[:6], map[string]interface{}{
		"destination_lat": client["latitude"], "destination_lng": client["longitude"],
	})
	if !assert.Equal(t, http.StatusCreated, code, raw) {
		return
	}
	assert.Equal(t, before+1, CountStopPlaces(t, helper), "one new place")
	outStops := StopNames(ListRouteStops(t, helper, ExtractID(t, created), ""))
	if assert.Len(t, outStops, 1) {
		assert.Equal(t, client["name"], outStops[0])
	}
}

// TestRoutePairDestination_UnknownStop - a stop the tenant does not have is not found.
func TestRoutePairDestination_UnknownStop(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	client := createPinnedClient(t, helper, nil)
	code, _, raw := postPair(t, helper, ExtractID(t, client), "Ruta "+uuid.NewString()[:6], map[string]interface{}{
		"destination_stop_place_id": uuid.NewString(),
	})
	assert.Equal(t, http.StatusNotFound, code, raw)
	assert.Contains(t, raw, "tracking.stop-place.not-found")
}
