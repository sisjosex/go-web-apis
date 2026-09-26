//go:build integration
// +build integration

package tracking_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"josex/web/modules/tracking/models"
)

// ============================================================================
// ROUTE CAPACITY (TRACK-023 D2)
// ============================================================================

// TestRoute_CapacityOverride - a route reads its vehicle's capacity until it sets its own; 0 clears
// the override back to the vehicle's. The assignment warnings measure against the same number.
func TestRoute_CapacityOverride(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	routeID, _ := createTripRoute(t, helper, "07:00")
	if _, err := helper.DB().Execute(context.Background(),
		`UPDATE tracking.routes SET vehicle_id = $2 WHERE id = $1`, routeID, TestBusID); err != nil {
		t.Fatalf("set vehicle: %v", err)
	}
	patch := func(body map[string]interface{}) models.Route {
		t.Helper()
		body["vehicle_id"] = TestBusID
		w := helper.DoRequest("PATCH", "/tracking/routes/"+routeID, body, map[string]string{})
		assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
		var route models.Route
		decodeBody(t, w, &route)
		return route
	}

	route := patch(map[string]interface{}{})
	if assert.NotNil(t, route.Capacity) {
		assert.Equal(t, int32(50), *route.Capacity, "the seeded bus")
	}
	assert.Nil(t, route.CapacityOverride)

	route = patch(map[string]interface{}{"capacity": 1})
	if assert.NotNil(t, route.Capacity) && assert.NotNil(t, route.CapacityOverride) {
		assert.Equal(t, int32(1), *route.Capacity)
		assert.Equal(t, int32(1), *route.CapacityOverride)
	}

	body := AssignmentBody(CreateTestRider(t, helper), routeID, EveryDayMask)
	w := helper.DoRequest("POST", "/tracking/assignments", body, map[string]string{})
	assert.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var created models.AssignmentResponse
	decodeBody(t, w, &created)
	assert.NotEmpty(t, created.Warnings, "two riders on a route of one seat")

	route = patch(map[string]interface{}{"capacity": 0})
	assert.Nil(t, route.CapacityOverride)
	if assert.NotNil(t, route.Capacity) {
		assert.Equal(t, int32(50), *route.Capacity)
	}
}
