//go:build integration
// +build integration

package tracking_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"josex/web/config"
	"josex/web/modules/core/testhelpers"
	"josex/web/modules/tracking/models"
)

// ============================================================================
// SHARE LINKS (TRACK-060)
// ============================================================================

// shareStart is a start time two hours ahead, so today's trip is still planned when the test runs.
func shareStart() string {
	return time.Now().UTC().Add(2 * time.Hour).Format("15:04")
}

// guardianTrip materialises today's trip of the portal guardian's rider and answers its id.
func guardianTrip(t *testing.T, operator *testhelpers.ApiTestHelper) string {
	t.Helper()
	start := shareStart()
	routeID := guardianRoute(t, operator, start)
	tripID, _ := tripOn(t, operator, routeID, dayOffset(t, "UTC", 0), start)
	return tripID
}

func postShareLink(t *testing.T, helper *testhelpers.ApiTestHelper, path string, body map[string]interface{}, want int) models.CreateShareLinkResponse {
	t.Helper()
	w := helper.DoRequest("POST", path, body, map[string]string{})
	assert.Equal(t, want, w.Code, w.Body.String())
	var created models.CreateShareLinkResponse
	if w.Code == http.StatusCreated {
		decodeBody(t, w, &created)
	}
	return created
}

func listShareLinks(t *testing.T, helper *testhelpers.ApiTestHelper, tripID string) []*models.ShareLink {
	t.Helper()
	w := helper.DoRequest("GET", "/tracking/trips/"+tripID+"/share-links", nil, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var list models.ListShareLinksResponse
	decodeBody(t, w, &list)
	return list.ShareLinks
}

// TestShareLinks_Operator - the operator shares a trip: the URL opens the public page under the
// tenant's slug, the list shows it, revoking takes it off the list; an ended trip cannot be shared.
func TestShareLinks_Operator(t *testing.T) {
	operator := SetupTrackingTest(t)
	defer operator.Close()
	tripID := guardianTrip(t, operator)

	created := postShareLink(t, operator, "/tracking/trips/"+tripID+"/share-links", map[string]interface{}{"ttl_minutes": 60}, http.StatusCreated)
	assert.True(t, strings.HasPrefix(created.URL, config.ModularAppConfig.Core.FrontendURL+"/s/test-company."), created.URL)
	assert.WithinDuration(t, time.Now().Add(time.Hour), created.ExpiresAt, time.Minute)

	// No body: until the trip's planned end + 60 minutes, at most a day.
	byDefault := postShareLink(t, operator, "/tracking/trips/"+tripID+"/share-links", nil, http.StatusCreated)
	assert.True(t, byDefault.ExpiresAt.After(time.Now().Add(time.Hour)), "past the planned end")
	assert.True(t, byDefault.ExpiresAt.Before(time.Now().Add(24*time.Hour+time.Minute)), "at most a day")

	ids := func() []string {
		var out []string
		for _, l := range listShareLinks(t, operator, tripID) {
			out = append(out, l.ID.String())
			assert.Equal(t, "web", l.CreatedVia)
		}
		return out
	}
	assert.Contains(t, ids(), created.ID.String())

	w := operator.DoRequest("DELETE", "/tracking/share-links/"+created.ID.String(), nil, map[string]string{})
	assert.Equal(t, http.StatusNoContent, w.Code, w.Body.String())
	assert.NotContains(t, ids(), created.ID.String())
	assert.Contains(t, ids(), byDefault.ID.String())

	w = operator.DoRequest("DELETE", "/tracking/share-links/"+created.ID.String(), nil, map[string]string{})
	assert.Equal(t, http.StatusNotFound, w.Code, "revoked twice: %s", w.Body.String())

	tripRequest(t, operator, "POST", "/tracking/trips/"+tripID+"/cancel", nil, http.StatusOK)
	w = operator.DoRequest("POST", "/tracking/trips/"+tripID+"/share-links", nil, map[string]string{})
	assert.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "tracking.share.no-trip")
}

// TestShareLinks_Guardian - a guardian shares its own rider's trip of today, never another family's,
// and revokes only what it shared.
func TestShareLinks_Guardian(t *testing.T) {
	operator := SetupTrackingTest(t)
	defer operator.Close()
	tripID := guardianTrip(t, operator)
	operatorLink := postShareLink(t, operator, "/tracking/trips/"+tripID+"/share-links", nil, http.StatusCreated)

	portal := SetupPortalTest(t)
	defer portal.Close()

	created := postShareLink(t, portal, "/tracking/riders/"+TestRiderJohnID+"/share-links", nil, http.StatusCreated)
	assert.Contains(t, created.URL, "/s/test-company.")
	postShareLink(t, portal, "/tracking/riders/"+TestRiderJaneID+"/share-links", nil, http.StatusNotFound)

	assert.Equal(t, "portal", ShareLinkVia(t, operator, created.ID.String()))

	w := portal.DoRequest("DELETE", "/tracking/share-links/"+operatorLink.ID.String(), nil, map[string]string{})
	assert.Equal(t, http.StatusNotFound, w.Code, "the operator's link: %s", w.Body.String())
	w = portal.DoRequest("DELETE", "/tracking/share-links/"+created.ID.String(), nil, map[string]string{})
	assert.Equal(t, http.StatusNoContent, w.Code, w.Body.String())
}

// ---- step 2: the public read ----

// publicShare reads a link's page data the way a browser does: no session, no tenant header.
func publicShare(helper *testhelpers.ApiTestHelper, url string) *httptest.ResponseRecorder {
	_, token, _ := strings.Cut(url, "/s/")
	return helper.DoRootRequest("GET", "/api/v1/public/share/"+token)
}

// TestShareLinks_PublicTrip - the operator's link opens the running trip with no session: the bus,
// every stop, no name anywhere; revoked, it is the same 404 as a token that never existed.
func TestShareLinks_PublicTrip(t *testing.T) {
	operator := SetupTrackingTest(t)
	defer operator.Close()
	tripID, _ := cochabambaTrip(t, operator)
	created := postShareLink(t, operator, "/tracking/trips/"+tripID+"/share-links", nil, http.StatusCreated)

	w := publicShare(operator, created.URL)
	if w.Code != http.StatusOK {
		t.Fatalf("public share: %d %s", w.Code, w.Body.String())
	}
	for _, key := range []string{"driver_name", "rider_name", "first_name", "last_name"} {
		assert.NotContains(t, w.Body.String(), key, "no driver or passenger name")
	}
	var shared models.SharedTrip
	decodeBody(t, w, &shared)
	assert.Equal(t, "trip", shared.Scope)
	assert.Equal(t, "in_progress", shared.Status)
	assert.Nil(t, shared.RiderFirstName)
	assert.Len(t, shared.Stops, 2)
	if assert.NotNil(t, shared.Position) {
		assert.InDelta(t, -17.3935, shared.Position.Lat, 1e-4)
	}

	w = operator.DoRequest("DELETE", "/tracking/share-links/"+created.ID.String(), nil, map[string]string{})
	assert.Equal(t, http.StatusNoContent, w.Code, w.Body.String())
	w = publicShare(operator, created.URL)
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "tracking.share.not-found")

	for _, bad := range []string{"test-company.nope", "no-such-tenant.abc", "nodot"} {
		w = operator.DoRootRequest("GET", "/api/v1/public/share/"+bad)
		assert.Equal(t, http.StatusNotFound, w.Code, "%s: %s", bad, w.Body.String())
	}
}

// TestShareLinks_PublicRider - a guardian's link shows that rider's stops and first name only.
func TestShareLinks_PublicRider(t *testing.T) {
	operator := SetupTrackingTest(t)
	defer operator.Close()
	guardianTrip(t, operator)

	portal := SetupPortalTest(t)
	defer portal.Close()
	created := postShareLink(t, portal, "/tracking/riders/"+TestRiderJohnID+"/share-links", nil, http.StatusCreated)

	w := publicShare(portal, created.URL)
	if w.Code != http.StatusOK {
		t.Fatalf("public share: %d %s", w.Code, w.Body.String())
	}
	var shared models.SharedTrip
	decodeBody(t, w, &shared)
	assert.Equal(t, "rider", shared.Scope)
	if assert.NotNil(t, shared.RiderFirstName) {
		assert.Equal(t, "John", *shared.RiderFirstName)
	}
	assert.NotEmpty(t, shared.Stops)
	assert.LessOrEqual(t, len(shared.Stops), 2, "the rider's pickup and dropoff only")
}
