//go:build integration
// +build integration

package tracking_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"josex/web/modules/core/testhelpers"
	geoInterfaces "josex/web/modules/geo/interfaces"
	geoModels "josex/web/modules/geo/models"
	geoRouting "josex/web/modules/geo/services/routing"
	trackingErrors "josex/web/modules/tracking/errors"
	trackingJobs "josex/web/modules/tracking/jobs"
	"josex/web/modules/tracking/models"
	trackingRepos "josex/web/modules/tracking/repositories"
)

// ============================================================================
// PLANNED ROUTE LINE (TRACK-028)
// ============================================================================

// stopAt creates a stop place at lat, lng and answers it as a version's sequence-th stop.
func stopAt(t *testing.T, helper *testhelpers.ApiTestHelper, sequence int, lat, lng float64) map[string]interface{} {
	t.Helper()
	body := ValidStopPlaceBody()
	body["latitude"], body["longitude"] = lat, lng
	w := helper.DoRequest("POST", "/tracking/stop-places", body, map[string]string{})
	if w.Code != http.StatusCreated {
		t.Fatalf("create stop place: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	return map[string]interface{}{"stop_place_id": ExtractID(t, ParseResponse(t, w.Body.Bytes())), "sequence": sequence}
}

// cochabambaStops are three stop entries along Av. Heroínas, west to east.
func cochabambaStops(t *testing.T, helper *testhelpers.ApiTestHelper) []map[string]interface{} {
	t.Helper()
	return []map[string]interface{}{
		stopAt(t, helper, 1, -17.3935, -66.1570),
		stopAt(t, helper, 2, -17.3940, -66.1540),
		stopAt(t, helper, 3, -17.3946, -66.1510),
	}
}

// storePath writes a line on a version as the route.changed handler would.
func storePath(t *testing.T, helper *testhelpers.ApiTestHelper, versionID, polyline string) {
	t.Helper()
	path := &models.RoutePath{Polyline6: polyline, DistanceM: 650, Source: "valhalla",
		Legs: []models.RoutePathLeg{{DistanceM: 320, DurationS: 60}, {DistanceM: 330, DurationS: 65}}}
	if err := trackingRepos.NewTrackingRepository(helper.DB()).SetRouteVersionPath(context.Background(),
		uuid.MustParse(TestTenantID), uuid.MustParse(versionID), path, "hash-"+polyline); err != nil {
		t.Fatalf("store path: %v", err)
	}
}

// getPath reads the route's line, sending etag as If-None-Match when set.
func getPath(helper *testhelpers.ApiTestHelper, routeID, query, etag string) (int, string, models.RoutePathResponse) {
	headers := map[string]string{}
	if etag != "" {
		headers["If-None-Match"] = etag
	}
	w := helper.DoRequest("GET", "/tracking/routes/"+routeID+"/path"+query, nil, headers)
	var body models.RoutePathResponse
	if w.Code == http.StatusOK {
		_ = json.Unmarshal(w.Body.Bytes(), &body)
	}
	return w.Code, w.Header().Get("ETag"), body
}

// TestRoutePath_StoredThen304 - a version with no line answers path null; once a line is stored it
// answers it under a new ETag, and that ETag sent back is 304.
func TestRoutePath_StoredThen304(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	routeID := CreateTestRoute(t, helper)
	versionID := PublishRouteVersion(t, helper, routeID, dayOffset(t, "UTC", 0), cochabambaStops(t, helper))

	code, before, body := getPath(helper, routeID, "", "")
	assert.Equal(t, http.StatusOK, code)
	assert.Nil(t, body.Path)

	storePath(t, helper, versionID, "abc")
	code, etag, body := getPath(helper, routeID, "", "")
	assert.Equal(t, http.StatusOK, code)
	assert.NotEqual(t, before, etag)
	if assert.NotNil(t, body.Path) {
		assert.Equal(t, versionID, body.Path.VersionID.String())
		assert.Equal(t, "abc", body.Path.Polyline6)
		assert.Equal(t, "valhalla", body.Path.Source)
		assert.Len(t, body.Path.Legs, 2)
	}

	code, _, _ = getPath(helper, routeID, "", etag)
	assert.Equal(t, http.StatusNotModified, code)
}

// TestRoutePath_PastDateAnswersThatDaysVersion - a date before the current version started answers
// the version in force then.
func TestRoutePath_PastDateAnswersThatDaysVersion(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	routeID := CreateTestRoute(t, helper)
	oldVersion := PublishRouteVersion(t, helper, routeID, dayOffset(t, "UTC", -10), cochabambaStops(t, helper))
	newVersion := PublishRouteVersion(t, helper, routeID, dayOffset(t, "UTC", 0), cochabambaStops(t, helper))
	storePath(t, helper, oldVersion, "old")
	storePath(t, helper, newVersion, "new")

	_, _, past := getPath(helper, routeID, "?date="+dayOffset(t, "UTC", -5), "")
	_, _, today := getPath(helper, routeID, "", "")

	if assert.NotNil(t, past.Path) && assert.NotNil(t, today.Path) {
		assert.Equal(t, "old", past.Path.Polyline6)
		assert.Equal(t, "new", today.Path.Polyline6)
	}
}

// TestRoutePath_OtherTenantNotFound - another tenant's route, or none at all, is 404.
func TestRoutePath_OtherTenantNotFound(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	routeID := CreateTestRoute(t, helper)

	_, err := trackingRepos.NewTrackingRepository(helper.DB()).GetRoutePath(context.Background(), uuid.New(), uuid.MustParse(routeID), nil, nil)
	var trackingErr *trackingErrors.TrackingError
	if assert.True(t, errors.As(err, &trackingErr), "%v", err) {
		assert.Equal(t, trackingErrors.RouteNotFound, trackingErr.Code)
	}

	code, _, _ := getPath(helper, uuid.NewString(), "", "")
	assert.Equal(t, http.StatusNotFound, code)
}

// ---- step 2: the job ----

// stubRouter answers every route through the locations as given, counting its calls; err set, it
// answers that instead.
type stubRouter struct {
	calls int
	err   error
}

func (s *stubRouter) Route(_ context.Context, locations []geoModels.LatLng, _ string) (*geoModels.Route, error) {
	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	route := &geoModels.Route{Polyline6: geoRouting.EncodePolyline6(locations)}
	for i := 1; i < len(locations); i++ {
		route.Legs = append(route.Legs, geoModels.RouteLeg{DistanceM: 400, DurationS: 90})
		route.DistanceM += 400
	}
	return route, nil
}

func (s *stubRouter) Trace(context.Context, []geoModels.LatLng, string) (*geoModels.Trace, error) {
	return nil, geoInterfaces.ErrRouteUnavailable
}

// applyPaths runs the route.changed handler's path half for routeID over all its versions.
func applyPaths(t *testing.T, helper *testhelpers.ApiTestHelper, router geoInterfaces.Router, routeID string) error {
	t.Helper()
	planner := trackingJobs.PathPlanner{Router: router, FallbackKmh: 25}
	return planner.ApplyRoutePaths(context.Background(), helper.DB(), testTenant(), uuid.MustParse(routeID), nil)
}

// TestRoutePaths_ReplaceStoresNewLine - replacing a version's stops stores a new line by streets and
// moves the ETag; each change is one router call.
func TestRoutePaths_ReplaceStoresNewLine(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	routeID := CreateTestRoute(t, helper)
	versionID := PublishRouteVersion(t, helper, routeID, dayOffset(t, "UTC", 0), cochabambaStops(t, helper))
	router := &stubRouter{}

	assert.NoError(t, applyPaths(t, helper, router, routeID))
	_, first, before := getPath(helper, routeID, "", "")

	w := helper.DoRequest("PUT", "/tracking/routes/"+routeID+"/versions/"+versionID+"/stops", map[string]interface{}{
		"stops": []map[string]interface{}{stopAt(t, helper, 1, -17.3800, -66.1600), stopAt(t, helper, 2, -17.3850, -66.1580)},
	}, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.NoError(t, applyPaths(t, helper, router, routeID))
	_, second, after := getPath(helper, routeID, "", "")

	assert.Equal(t, 2, router.calls)
	assert.NotEqual(t, first, second)
	if assert.NotNil(t, before.Path) && assert.NotNil(t, after.Path) {
		assert.NotEqual(t, before.Path.Polyline6, after.Path.Polyline6)
		assert.Equal(t, trackingJobs.PathValhalla, after.Path.Source)
		assert.Len(t, after.Path.Legs, 1)
		assert.Equal(t, 400, after.Path.DistanceM)
	}
}

// TestRoutePaths_RenameCostsNoCall - renaming a stop of the route leaves its points as they were: the
// handler calls the router 0 times and the ETag stays.
func TestRoutePaths_RenameCostsNoCall(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	routeID := CreateTestRoute(t, helper)
	stops := cochabambaStops(t, helper)
	PublishRouteVersion(t, helper, routeID, dayOffset(t, "UTC", 0), stops)
	assert.NoError(t, applyPaths(t, helper, &stubRouter{}, routeID))
	_, before, _ := getPath(helper, routeID, "", "")

	w := helper.DoRequest("PATCH", "/tracking/stop-places/"+stops[0]["stop_place_id"].(string), map[string]interface{}{"name": "Plaza 14 de Septiembre"}, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	router := &stubRouter{}
	assert.NoError(t, applyPaths(t, helper, router, routeID))
	_, after, _ := getPath(helper, routeID, "", "")

	assert.Equal(t, 0, router.calls)
	assert.Equal(t, before, after)
}

// TestRoutePaths_RouterDownStoresFallback - a router that errors stores the straight line through the
// stops flagged fallback, timed per leg, and fails the task so it is retried; the retry against the
// same outage writes nothing new.
func TestRoutePaths_RouterDownStoresFallback(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	routeID := CreateTestRoute(t, helper)
	PublishRouteVersion(t, helper, routeID, dayOffset(t, "UTC", 0), cochabambaStops(t, helper))
	down := &stubRouter{err: fmt.Errorf("%w: valhalla answered 500", geoInterfaces.ErrRouteUnavailable)}

	assert.ErrorIs(t, applyPaths(t, helper, down, routeID), trackingJobs.ErrPathFallback)
	_, etag, body := getPath(helper, routeID, "", "")
	if assert.NotNil(t, body.Path) {
		assert.Equal(t, trackingJobs.PathFallback, body.Path.Source)
		assert.NotEmpty(t, body.Path.Polyline6)
		if assert.Len(t, body.Path.Legs, 2) {
			assert.Greater(t, body.Path.Legs[0].DistanceM, 0)
			assert.Greater(t, body.Path.Legs[0].DurationS, 0)
		}
	}

	assert.ErrorIs(t, applyPaths(t, helper, down, routeID), trackingJobs.ErrPathFallback)
	_, retried, _ := getPath(helper, routeID, "", "")
	assert.Equal(t, etag, retried)
	assert.Equal(t, 2, down.calls)
}

// TestRoutePaths_BackfillQueuesUncomputed - the backfill writes a route.changed row for a route whose
// version has no line, and none once the line is computed.
func TestRoutePaths_BackfillQueuesUncomputed(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	routeID := CreateTestRoute(t, helper)
	PublishRouteVersion(t, helper, routeID, dayOffset(t, "UTC", 0), cochabambaStops(t, helper))
	repo := trackingRepos.NewTrackingRepository(helper.DB())

	_, err := repo.RoutePathsBackfill(context.Background(), uuid.MustParse(TestTenantID))
	assert.NoError(t, err)
	assert.Equal(t, 1, BackfillRowsFor(t, helper, routeID))

	assert.NoError(t, applyPaths(t, helper, &stubRouter{}, routeID))
	_, err = repo.RoutePathsBackfill(context.Background(), uuid.MustParse(TestTenantID))
	assert.NoError(t, err)
	assert.Equal(t, 1, BackfillRowsFor(t, helper, routeID))
}
