//go:build integration
// +build integration

package tracking_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"josex/web/modules/core/testhelpers"
	"josex/web/modules/tracking/models"
)

// ============================================================================
// PRINTABLE RIDER CARDS (TRACK-072 D2)
// ============================================================================

// riderCards reads GET /tracking/riders/cards with query, asserting the status.
func riderCards(t *testing.T, helper *testhelpers.ApiTestHelper, query string, want int) []models.RiderCard {
	t.Helper()
	w := helper.DoRequest("GET", "/tracking/riders/cards"+query, nil, map[string]string{})
	expectStatus(t, w, want)
	var body models.RiderCards
	if want == http.StatusOK {
		decodeBody(t, w, &body)
	}
	return body.Cards
}

// namedRider creates a rider of organizationID with a first name to find it by.
func namedRider(t *testing.T, helper *testhelpers.ApiTestHelper, organizationID, first string) string {
	t.Helper()
	id := createRider(t, helper, map[string]interface{}{"organization_id": organizationID, "first_name": first})
	t.Cleanup(func() { helper.DoRequest("DELETE", "/tracking/riders/"+id, nil, map[string]string{}) })
	return id
}

// boardAt assigns riderID to routeID every day, picked up at stopPlaceID.
func boardAt(t *testing.T, helper *testhelpers.ApiTestHelper, routeID, riderID, stopPlaceID string) {
	t.Helper()
	body := AssignmentBody(riderID, routeID, 127)
	body["pickup_stop_place_id"] = stopPlaceID
	CreateAssignment(t, helper, body)
}

// TestRiderCards_RouteInStopOrder - a route's cards come in boarding order, each with its stop and a
// token the driver's phone resolves; a client member reads only their own client's cards.
func TestRiderCards_RouteInStopOrder(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	routeID := CreateTestRoute(t, helper)
	stops := cochabambaStops(t, helper)
	PublishRouteVersion(t, helper, routeID, dayOffset(t, "UTC", 0), stops)
	other := ExtractID(t, createPinnedClient(t, helper, nil))

	tag := uuid.NewString()[:5]
	third := namedRider(t, helper, MainSchoolID, "Tercero"+tag)
	first := namedRider(t, helper, MainSchoolID, "Primero"+tag)
	second := namedRider(t, helper, other, "Segundo"+tag)
	boardAt(t, helper, routeID, third, stops[2]["stop_place_id"].(string))
	boardAt(t, helper, routeID, first, stops[0]["stop_place_id"].(string))
	boardAt(t, helper, routeID, second, stops[1]["stop_place_id"].(string))

	cards := riderCards(t, helper, "?route_id="+routeID, http.StatusOK)
	if !assert.Len(t, cards, 3) {
		return
	}
	assert.Equal(t, []string{first, second, third},
		[]string{cards[0].RiderID.String(), cards[1].RiderID.String(), cards[2].RiderID.String()})
	assert.NotNil(t, cards[0].PickupStopName)
	assert.NotNil(t, cards[0].RouteName)

	LinkDriverAccount(t, helper, CreateTestDriver(t, helper))
	driver := SetupDriverTest(t)
	defer driver.Close()
	for _, card := range cards {
		assert.Len(t, card.QRToken, 32)
		assert.Equal(t, card.RiderID, resolve(t, driver, card.QRToken, http.StatusOK).Rider.ID)
	}

	member := SetupOrganizationTest(t)
	defer member.Close()
	mine := riderCards(t, member, "?route_id="+routeID, http.StatusOK)
	if assert.Len(t, mine, 2) {
		assert.Equal(t, first, mine[0].RiderID.String())
		assert.Equal(t, third, mine[1].RiderID.String())
	}
	assert.Empty(t, riderCards(t, member, "?rider_id="+second, http.StatusOK))

	one := riderCards(t, helper, "?rider_id="+second, http.StatusOK)
	if assert.Len(t, one, 1) {
		assert.Equal(t, cards[1].QRToken, one[0].QRToken)
	}
}

// TestRiderCards_ExactlyOneFilter - no filter, two filters or a malformed id is a 400.
func TestRiderCards_ExactlyOneFilter(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	riderCards(t, helper, "", http.StatusBadRequest)
	riderCards(t, helper, "?route_id="+uuid.NewString()+"&rider_id="+uuid.NewString(), http.StatusBadRequest)
	riderCards(t, helper, "?organization_id=nope", http.StatusBadRequest)
	assert.Empty(t, riderCards(t, helper, "?organization_id="+uuid.NewString(), http.StatusOK))
}
