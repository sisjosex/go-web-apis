//go:build integration
// +build integration

package tracking_test

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"josex/web/config"
	"josex/web/modules/core/testhelpers"
	geoInterfaces "josex/web/modules/geo/interfaces"
	geoModels "josex/web/modules/geo/models"
	geoServices "josex/web/modules/geo/services"
	trackingJobs "josex/web/modules/tracking/jobs"
	"josex/web/modules/tracking/push"
	trackingRepos "josex/web/modules/tracking/repositories"
	trackingServices "josex/web/modules/tracking/services"
)

// ============================================================================
// ETA FROM HISTORY AND DELAY NOTICES (TRACK-066)
// ============================================================================

// pairStop is a stop of pairRoute: its place and where it is.
type pairStop struct {
	ID       string
	Lat, Lng float64
}

// pairRoute is a fresh route through three new stops a few hundred metres apart, at a spot of
// Cochabamba no other test uses — the history matches a point to the nearest stop within 40 m —
// running every day at 07:00, stops planned 0, 10 and 20 minutes after the start.
func pairRoute(t *testing.T, helper *testhelpers.ApiTestHelper) (string, []pairStop) {
	t.Helper()
	lat, lng := -17.45-rand.Float64()*0.05, -66.20-rand.Float64()*0.05
	stops := make([]pairStop, 3)
	for i := range stops {
		stops[i] = pairStop{Lat: lat - float64(i)*0.004, Lng: lng + float64(i)*0.004}
		w := helper.DoRequest("POST", "/tracking/stop-places", map[string]interface{}{
			"name": "Pair Stop " + uuid.NewString()[:8], "address": "Cochabamba", "latitude": stops[i].Lat, "longitude": stops[i].Lng,
		}, map[string]string{})
		if w.Code != http.StatusCreated {
			t.Fatalf("create stop place: %d %s", w.Code, w.Body.String())
		}
		stops[i].ID = ExtractID(t, ParseResponse(t, w.Body.Bytes()))
	}
	routeID := CreateTestRoute(t, helper)
	version := make([]map[string]interface{}, len(stops))
	for i, s := range stops {
		version[i] = map[string]interface{}{"stop_place_id": s.ID, "sequence": i + 1, "planned_offset_min": i * 10}
	}
	PublishRouteVersion(t, helper, routeID, "2026-01-01", version)
	CreateRouteSchedule(t, helper, routeID, map[string]interface{}{"days_of_week": EveryDayMask, "start_time": "07:00", "valid_from": "2026-01-01"})
	return routeID, stops
}

// startPairTrip starts today's trip of routeID on a fresh vehicle. It answers the trip and the vehicle.
func startPairTrip(t *testing.T, helper *testhelpers.ApiTestHelper, routeID string) (string, string) {
	t.Helper()
	today := dayOffset(t, "UTC", 0)
	materialiseRoute(t, helper, routeID, today, today)
	tripID, _ := tripOn(t, helper, routeID, today, "07:00")
	vehicleID := CreateTestVehicleOf(t, helper, MainCompanyID)
	tripRequest(t, helper, "PATCH", "/tracking/trips/"+tripID, map[string]interface{}{"vehicle_id": vehicleID}, http.StatusOK)
	tripRequest(t, helper, "POST", "/tracking/trips/"+tripID+"/start", nil, http.StatusOK)
	return tripID, vehicleID
}

// downRouter is Valhalla out: every route and trace answers ErrRouteUnavailable.
type downRouter struct{ geoInterfaces.Router }

func (downRouter) Route(context.Context, []geoModels.LatLng, string) (*geoModels.Route, error) {
	return nil, fmt.Errorf("%w: valhalla answered 500", geoInterfaces.ErrRouteUnavailable)
}

func (downRouter) Trace(context.Context, []geoModels.LatLng, string) (*geoModels.Trace, error) {
	return nil, geoInterfaces.ErrRouteUnavailable
}

// historyLive is the live service as the servers wire it, over router, with the historical source.
func historyLive(helper *testhelpers.ApiTestHelper, router geoInterfaces.Router) *trackingServices.LiveService {
	geoConf := config.ModularAppConfig.Geo
	history := trackingRepos.NewHistoricalDurations(helper.DB())
	eta := trackingServices.NewEtaService(router, geoServices.NewGeoService(nil, router, history, geoConf), helper.Valkey())
	return trackingServices.NewLiveService(trackingRepos.NewTrackingRepository(helper.DB()), eta)
}

// ---- step 2: samples and the historical source ----

// TestStopPairSamples_RecordedAtComplete - a completed trip whose 3 stops were all reached leaves 2
// stop-to-stop samples with their seconds; the completion replayed leaves 2.
func TestStopPairSamples_RecordedAtComplete(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	routeID, _ := pairRoute(t, helper)
	tripID, _ := startPairTrip(t, helper, routeID)

	detail := tripRequest(t, helper, "GET", "/tracking/trips/"+tripID, nil, http.StatusOK)
	for _, stop := range detail.Stops {
		tripRequest(t, helper, "POST", "/tracking/trip-stops/"+stop.ID.String()+"/arrive", nil, http.StatusOK)
	}
	// Arrivals 4 and 6 minutes apart, as a real drive would leave them.
	SetStopArrivals(t, helper, tripID, 10, 6, 0)
	tripRequest(t, helper, "POST", "/tracking/trips/"+tripID+"/complete", nil, http.StatusOK)

	for i := 0; i < 2; i++ {
		if _, err := trackingJobs.ApplyTripTrace(context.Background(), helper.DB(), nil, testTenant(), uuid.MustParse(tripID)); err != nil {
			t.Fatalf("trace %d: %v", i, err)
		}
		assert.Equal(t, 2, StopPairSampleCount(t, helper, tripID, 0))
	}
	assert.Equal(t, 1, StopPairSampleCount(t, helper, tripID, 240))
	assert.Equal(t, 1, StopPairSampleCount(t, helper, tripID, 360))
}

// seedPairSamples stores n samples of seconds each between stops from and to, on n other trips of the
// route (days ahead of today), in the band of now — what n completed trips would have left.
func seedPairSamples(t *testing.T, helper *testhelpers.ApiTestHelper, routeID, from, to string, seconds ...int) {
	t.Helper()
	materialiseRoute(t, helper, routeID, dayOffset(t, "UTC", 1), dayOffset(t, "UTC", len(seconds)))
	for i, s := range seconds {
		tripID, _ := tripOn(t, helper, routeID, dayOffset(t, "UTC", i+1), "07:00")
		SeedStopPairSample(t, helper, tripID, from, to, s)
	}
}

// TestTripLive_HistoricalEtaWhenRouterDown - Valhalla down, a bus standing at its arrived first stop:
// stop 2 is three completed trips' median away, flagged historical, and stop 3 (no history) is the
// straight line; with one sample left, stop 2 falls back to the straight line too.
func TestTripLive_HistoricalEtaWhenRouterDown(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	routeID, stops := pairRoute(t, helper)
	seedPairSamples(t, helper, routeID, stops[0].ID, stops[1].ID, 300, 420, 360)
	tripID, vehicleID := startPairTrip(t, helper, routeID)
	detail := tripRequest(t, helper, "GET", "/tracking/trips/"+tripID, nil, http.StatusOK)
	tripRequest(t, helper, "POST", "/tracking/trip-stops/"+detail.Stops[0].ID.String()+"/arrive", nil, http.StatusOK)
	ingest(t, helper, pointsJSON(t, vehicleID, time.Now().UTC(), [2]float64{stops[0].Lat + 0.0001, stops[0].Lng}))

	ctx := context.Background()
	live := historyLive(helper, downRouter{})
	read := func() ([]string, time.Duration) {
		_ = helper.Valkey().Client().Del(ctx, "eta:"+tripID).Err()
		_, eta, err := live.Trip(ctx, uuid.MustParse(TestTenantID), uuid.MustParse(tripID), nil)
		if err != nil {
			t.Fatalf("trip live: %v", err)
		}
		if !assert.Len(t, eta.Eta, 2) {
			t.FailNow()
		}
		return []string{eta.Eta[0].EstimateSource, eta.Eta[1].EstimateSource}, eta.Eta[0].EtaAt.Sub(eta.ComputedAt)
	}

	sources, toStop2 := read()
	assert.Equal(t, []string{geoModels.EstimateHistorical, geoModels.EstimateFallback}, sources)
	assert.InDelta(t, 360, toStop2.Seconds(), 1, "the median of 300, 360, 420")

	KeepStopPairSamples(t, helper, stops[0].ID, 360)
	sources, _ = read()
	assert.Equal(t, geoModels.EstimateFallback, sources[0])
}

// ---- step 3: the delay notice ----

// legsRouter answers a route whose legs take the given seconds, in order (the last repeats).
type legsRouter struct {
	geoInterfaces.Router
	seconds []float64
}

func (r legsRouter) Route(_ context.Context, locations []geoModels.LatLng, _ string) (*geoModels.Route, error) {
	route := &geoModels.Route{}
	for i := 1; i < len(locations); i++ {
		s := r.seconds[min(i-1, len(r.seconds)-1)]
		route.Legs = append(route.Legs, geoModels.RouteLeg{DistanceM: 1000, DurationS: s})
		route.DurationS += s
	}
	return route, nil
}

// setDelayNotice sets the school's delay threshold through PATCH, as its form does, and answers the row.
func setDelayNotice(t *testing.T, helper *testhelpers.ApiTestHelper, minutes int) map[string]interface{} {
	t.Helper()
	w := helper.DoRequest("PATCH", "/tracking/organizations/"+MainSchoolID, map[string]interface{}{"delay_notice_min": minutes}, map[string]string{})
	if w.Code != http.StatusOK {
		t.Fatalf("patch organization: %d %s", w.Code, w.Body.String())
	}
	return ParseResponse(t, w.Body.Bytes())
}

// TestTripDelay_NoticeOncePastThreshold - a 12-minute leg to stop 2, planned for now, with the school's
// threshold at 10: one late-ETA delay event, and one feed row with delay_min 12 for the guardian of
// the rider boarding there — none for the rider whose stop the bus already reached; a second fresh
// ETA adds no event; the threshold at 15 stamps nothing. PATCH sets the threshold.
func TestTripDelay_NoticeOncePastThreshold(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	routeID, stops := pairRoute(t, helper)
	boardsAtStop2, boardedAtStop1 := CreateTestRider(t, helper), CreateTestRider(t, helper)
	for rider, pickup := range map[string]string{boardsAtStop2: stops[1].ID, boardedAtStop1: stops[0].ID} {
		body := AssignmentBody(rider, routeID, EveryDayMask)
		body["pickup_stop_place_id"], body["dropoff_stop_place_id"] = pickup, stops[2].ID
		CreateAssignment(t, helper, body)
		addGuardians(t, helper, rider, "portal@test.local")
	}
	t.Cleanup(func() { setDelayNotice(t, helper, 10) })
	setDelayNotice(t, helper, 10)

	tripID, vehicleID := startPairTrip(t, helper, routeID)
	detail := tripRequest(t, helper, "GET", "/tracking/trips/"+tripID, nil, http.StatusOK)
	stop2 := detail.Stops[1].ID.String()
	PlanStopsAroundNow(t, helper, tripID)
	tripRequest(t, helper, "POST", "/tracking/trip-stops/"+detail.Stops[0].ID.String()+"/arrive", nil, http.StatusOK)
	ingest(t, helper, pointsJSON(t, vehicleID, time.Now().UTC(), [2]float64{stops[0].Lat, stops[0].Lng}))

	ctx := context.Background()
	live := historyLive(helper, legsRouter{seconds: []float64{720, 60}})
	freshEta := func() {
		_ = helper.Valkey().Client().Del(ctx, "eta:"+tripID).Err()
		if _, _, err := live.Trip(ctx, uuid.MustParse(TestTenantID), uuid.MustParse(tripID), nil); err != nil {
			t.Fatalf("trip live: %v", err)
		}
	}
	delayEvents := func() int { return TripDelayEvents(t, helper, tripID) }

	freshEta()
	if !assert.Equal(t, 1, delayEvents()) {
		t.FailNow()
	}
	assert.True(t, StopDelayNotified(t, helper, stop2))
	NotifyTripDelays(t, helper, tripID)
	notice := DelayNotices(t, helper, boardsAtStop2)["portal@test.local"]
	if assert.NotNil(t, notice, "the guardian of the rider boarding at stop 2") {
		assert.EqualValues(t, 12, notice["delay_min"])
		assert.NotEmpty(t, notice["eta_at"])
		assert.NotEmpty(t, notice["eta_time"])
		assert.NotEmpty(t, notice["stop_name"])
	}
	assert.Empty(t, DelayNotices(t, helper, boardedAtStop1), "stop 1 is behind the bus")
	title, body := push.NoticeText("es", "delay_eta", []string{"Ana", "Ruta 1", "Plaza", "12", "07:42"})
	assert.Equal(t, "Retraso de 12 min", title)
	assert.Equal(t, "Ana llega ~07:42 a Plaza.", body)

	freshEta()
	assert.Equal(t, 1, delayEvents(), "a stop is announced once")

	ClearStopDelayNotice(t, helper, stop2)
	setDelayNotice(t, helper, 15)
	freshEta()
	assert.Equal(t, 1, delayEvents(), "12 minutes late is under a 15-minute threshold")

	assert.EqualValues(t, 20, setDelayNotice(t, helper, 20)["delay_notice_min"])
}
