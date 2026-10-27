//go:build integration
// +build integration

package tracking_test

import (
	"net/http"
	"net/url"
	"testing"
	"time"

	"josex/web/modules/core/testhelpers"
	"josex/web/modules/tracking/models"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TRACK-075: the vehicles, routes and stop places lists carry the figures their screens show.

// createFleetCompany is a carrier of its own, so a test's counts are only its vehicles.
func createFleetCompany(t *testing.T, helper *testhelpers.ApiTestHelper) string {
	t.Helper()
	w := helper.DoRequest("POST", "/tracking/companies", ValidCompanyDto(), map[string]string{})
	expectStatus(t, w, http.StatusCreated)
	return ExtractID(t, ParseResponse(t, w.Body.Bytes()))
}

func listVehicleIDs(res models.ListVehiclesResponse) []string {
	ids := []string{}
	for _, v := range res.Vehicles {
		ids = append(ids, v.ID.String())
	}
	return ids
}

// TestVehicles_ViewsAndFigures - a vehicle with a fix 30 s old and a trip running reads last_seen_at
// and today's route and is the on_route view; one never seen with a document due in 5 days is the
// no_gps and docs_due views; the counts add up over the company, and an empty page still has them.
func TestVehicles_ViewsAndFigures(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	companyID := createFleetCompany(t, helper)
	running := CreateTestVehicleOf(t, helper, companyID)
	unseen := CreateTestVehicleOf(t, helper, companyID)

	ingest(t, helper, pointsJSON(t, running, time.Now().UTC().Add(-30*time.Second), [2]float64{-17.39, -66.15}))
	routeID, _, tripID := todaysTrip(t, helper)
	RunTripOnVehicle(t, helper, tripID, running)
	CreateTestDocument(t, helper, "vehicle", unseen, CreateTestDocumentType(t, helper, "vehicle", false), 5)

	all := ListVehicles(t, helper, "?company_id="+companyID)
	assert.Equal(t, models.VehicleCounts{All: 2, OnRoute: 1, NoGPS: 1, DocsDue: 1}, all.Counts)
	byID := map[string]*models.Vehicle{}
	for _, v := range all.Vehicles {
		byID[v.ID.String()] = v
	}
	require.Contains(t, byID, running)
	require.Contains(t, byID, unseen)
	if assert.NotNil(t, byID[running].LastSeenAt) {
		assert.WithinDuration(t, time.Now().Add(-30*time.Second), *byID[running].LastSeenAt, 5*time.Second)
	}
	routeName := readField(t, helper, "/tracking/routes/"+routeID, "route_name")
	if assert.NotNil(t, byID[running].TodayRouteName) {
		assert.Equal(t, routeName, *byID[running].TodayRouteName)
	}
	assert.Nil(t, byID[running].NextDocument)
	assert.Nil(t, byID[unseen].LastSeenAt)
	if assert.NotNil(t, byID[unseen].NextDocument) {
		assert.Equal(t, "expiring", byID[unseen].NextDocument.Status)
	}

	assert.Equal(t, []string{running}, listVehicleIDs(ListVehicles(t, helper, "?company_id="+companyID+"&view=on_route")))
	assert.Equal(t, []string{unseen}, listVehicleIDs(ListVehicles(t, helper, "?company_id="+companyID+"&view=no_gps")))
	assert.Equal(t, []string{unseen}, listVehicleIDs(ListVehicles(t, helper, "?company_id="+companyID+"&view=docs_due")))

	empty := ListVehicles(t, helper, "?company_id="+companyID+"&page=5")
	assert.Empty(t, empty.Vehicles)
	assert.Equal(t, int64(2), empty.Counts.All, "an empty page keeps the counts")

	w := helper.DoRequest("GET", "/tracking/vehicles?view=parked", nil, map[string]string{})
	assert.Equal(t, http.StatusBadRequest, w.Code, "an unknown view is refused")
}

// TestRoutes_WithoutDriver - of two routes, the one with no driver of its own nor a vehicle's is
// counts.without_driver and the only row of without_driver=true; organization_kind rides on the row.
func TestRoutes_WithoutDriver(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	prefix := "SinConductor-" + uuid.NewString()[:8]
	driverless := namedRoute(t, helper, prefix+" a")
	driven := namedRoute(t, helper, prefix+" b")
	SetRouteDefaultDriver(t, helper, driven, CreateTestDriver(t, helper))

	list := func(query string) models.ListRoutesResponse {
		t.Helper()
		w := helper.DoRequest("GET", "/tracking/routes?search="+prefix+query, nil, map[string]string{})
		expectStatus(t, w, http.StatusOK)
		var res models.ListRoutesResponse
		decodeBody(t, w, &res)
		return res
	}

	all := list("")
	assert.Equal(t, models.RouteCounts{All: 2, Outbound: 2, WithoutDriver: 1}, all.Counts)
	only := list("&without_driver=true")
	if assert.Len(t, only.Routes, 1) {
		assert.Equal(t, driverless, only.Routes[0].ID.String())
	}
	assert.Equal(t, int64(1), only.Counts.WithoutDriver, "the chip's own filter does not narrow its count")

	empty := list("&direction=inbound")
	assert.Empty(t, empty.Routes)
	assert.Equal(t, int64(2), empty.Counts.All, "an empty page keeps the counts")
}

func namedRoute(t *testing.T, helper *testhelpers.ApiTestHelper, name string) string {
	t.Helper()
	dto := ValidRouteDto()
	w := helper.DoRequest("POST", "/tracking/routes", map[string]interface{}{
		"company_id": dto.CompanyID, "route_name": name, "origin_address": dto.OriginAddress,
		"destination_address": dto.DestinationAddress, "timezone": *dto.Timezone,
	}, map[string]string{})
	expectStatus(t, w, http.StatusCreated)
	return ExtractID(t, ParseResponse(t, w.Body.Bytes()))
}

// TestStopPlaces_Usage - a stop four riders board at reads riders_count 4 and its one route; the
// route's last stop is a destination, the first is not.
func TestStopPlaces_Usage(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	pickup := CreateTestStopPlace(t, helper)
	school := CreateTestStopPlace(t, helper)
	routeID := CreateTestRoute(t, helper)
	PublishRouteVersion(t, helper, routeID, "2026-01-01", []map[string]interface{}{
		{"stop_place_id": pickup, "sequence": 1, "planned_offset_min": 0},
		{"stop_place_id": school, "sequence": 2, "planned_offset_min": 20},
	})
	for i := 0; i < 4; i++ {
		body := AssignmentBody(CreateTestRider(t, helper), routeID, EveryDayMask)
		body["pickup_stop_place_id"] = pickup
		body["dropoff_stop_place_id"] = school
		CreateAssignment(t, helper, body)
	}

	byID := map[string]*models.StopPlace{}
	for _, id := range []string{pickup, school} {
		name := readField(t, helper, "/tracking/stop-places/"+id, "name")
		w := helper.DoRequest("GET", "/tracking/stop-places?page_size=100&search="+url.QueryEscape(name), nil, map[string]string{})
		expectStatus(t, w, http.StatusOK)
		var res models.ListStopPlacesResponse
		decodeBody(t, w, &res)
		require.Len(t, res.StopPlaces, 1)
		byID[id] = res.StopPlaces[0]
	}
	for id, wantDestination := range map[string]bool{pickup: false, school: true} {
		place := byID[id]
		assert.Equal(t, int32(4), *place.RidersCount)
		assert.Equal(t, int32(1), *place.RoutesCount)
		assert.NotNil(t, place.RouteName)
		assert.Equal(t, wantDestination, *place.IsDestination)
	}
}

// readField is one string field of the record a GET answers.
func readField(t *testing.T, helper *testhelpers.ApiTestHelper, path, field string) string {
	t.Helper()
	w := helper.DoRequest("GET", path, nil, map[string]string{})
	expectStatus(t, w, http.StatusOK)
	value, _ := ParseResponse(t, w.Body.Bytes())[field].(string)
	return value
}
