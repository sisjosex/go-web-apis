//go:build integration
// +build integration

package tracking_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// TRACK-046: the routes list answers which routes still have seats.

// seatedRoute is a route on the seeded bus named name with seats seats and one rider aboard.
func seatedRoute(t *testing.T, name string, seats int) string {
	t.Helper()
	helper := SetupTrackingTest(t)
	defer helper.Close()
	routeID, _ := createTripRoute(t, helper, "07:00")
	if _, err := helper.DB().Execute(context.Background(),
		`UPDATE tracking.routes SET vehicle_id = $2, route_name = $3, capacity = $4 WHERE id = $1`,
		routeID, TestBusID, name, seats); err != nil {
		t.Fatalf("seat route: %v", err)
	}
	w := helper.DoRequest("POST", "/tracking/assignments",
		AssignmentBody(CreateHomedRider(t, helper), routeID, EveryDayMask), map[string]string{})
	expectStatus(t, w, http.StatusCreated)
	return routeID
}

// TestRoutes_FreeSeatsFilter - has_free_seats keeps the half-full route and drops the full one;
// sort=free_seats puts the emptiest first.
func TestRoutes_FreeSeatsFilter(t *testing.T) {
	prefix := "Cupos-" + uuid.NewString()[:8]
	full := seatedRoute(t, prefix+" llena", 1)
	half := seatedRoute(t, prefix+" media", 2)
	roomy := seatedRoute(t, prefix+" amplia", 10)

	helper := SetupTrackingTest(t)
	defer helper.Close()
	list := func(query string) []string {
		t.Helper()
		w := helper.DoRequest("GET", "/tracking/routes?search="+prefix+query, nil, map[string]string{})
		expectStatus(t, w, http.StatusOK)
		ids := []string{}
		for _, row := range ParseResponse(t, w.Body.Bytes())["routes"].([]interface{}) {
			ids = append(ids, row.(map[string]interface{})["id"].(string))
		}
		return ids
	}

	assert.ElementsMatch(t, []string{full, half, roomy}, list(""))
	assert.Equal(t, []string{roomy, half}, list("&has_free_seats=true&sort=free_seats"))

	w := helper.DoRequest("GET", "/tracking/routes?sort=seats", nil, map[string]string{})
	assert.Equal(t, http.StatusBadRequest, w.Code, "an unknown sort is refused")
}
