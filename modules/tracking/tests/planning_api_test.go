//go:build integration
// +build integration

package tracking_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"josex/web/config"
	"josex/web/modules/core/testhelpers"
	geoRouting "josex/web/modules/geo/services/routing"
	trackingJobs "josex/web/modules/tracking/jobs"
	"josex/web/modules/tracking/models"
	trackingRepos "josex/web/modules/tracking/repositories"
)

// ============================================================================
// PLANNING SCREEN AND ROUTE PROPOSALS (TRACK-014)
// ============================================================================

// planningFixture is a school with 25 riders around it (the first 5 outbound only), two free vehicles
// of 15 seats and a third busy with a route at 07:30 on weekdays.
type planningFixture struct {
	organizationID string
	riderIDs       []string
	freeVehicles   []string
	busyVehicle    string
	busyRoute      string
}

func newPlanningFixture(t *testing.T, helper *testhelpers.ApiTestHelper) planningFixture {
	t.Helper()
	w := helper.DoRequest("POST", "/tracking/organizations", map[string]interface{}{
		"name": "Planning School " + uuid.NewString()[:8], "kind": "school", "timezone": "UTC",
		"latitude": -17.3935, "longitude": -66.1570,
	}, map[string]string{})
	if w.Code != http.StatusCreated {
		t.Fatalf("create organization: %d %s", w.Code, w.Body.String())
	}
	f := planningFixture{organizationID: ExtractID(t, ParseResponse(t, w.Body.Bytes()))}
	for i := 0; i < 25; i++ {
		legs := "both"
		if i < 5 {
			legs = "outbound"
		}
		// A 5 × 5 grid of homes within ~2.5 km of the school.
		w := helper.DoRequest("POST", "/tracking/riders", map[string]interface{}{
			"organization_id": f.organizationID, "rider_type": "student",
			"first_name": "Rider", "last_name": fmt.Sprintf("Plan %02d", i), "service_legs": legs,
			"home_latitude":  -17.3935 + float64(i/5-2)*0.005,
			"home_longitude": -66.1570 + float64(i%5-2)*0.005,
		}, map[string]string{})
		if w.Code != http.StatusCreated {
			t.Fatalf("create rider %d: %d %s", i, w.Code, w.Body.String())
		}
		f.riderIDs = append(f.riderIDs, ExtractID(t, ParseResponse(t, w.Body.Bytes())))
	}
	// 25 riders per test would push the seeded ones off the riders list's first page elsewhere.
	t.Cleanup(func() {
		for _, id := range f.riderIDs {
			helper.DoRequest("DELETE", "/tracking/riders/"+id, nil, map[string]string{})
		}
	})
	for i := 0; i < 3; i++ {
		dto := ValidVehicleDto()
		dto.Capacity = 15
		w := helper.DoRequest("POST", "/tracking/vehicles", dto, map[string]string{})
		if w.Code != http.StatusCreated {
			t.Fatalf("create vehicle: %d %s", w.Code, w.Body.String())
		}
		f.freeVehicles = append(f.freeVehicles, ExtractID(t, ParseResponse(t, w.Body.Bytes())))
	}
	f.busyVehicle, f.freeVehicles = f.freeVehicles[2], f.freeVehicles[:2]
	f.busyRoute = CreateTestRoute(t, helper)
	w = helper.DoRequest("PATCH", "/tracking/routes/"+f.busyRoute, map[string]interface{}{"vehicle_id": f.busyVehicle}, map[string]string{})
	if w.Code != http.StatusOK {
		t.Fatalf("put the vehicle on the route: %d %s", w.Code, w.Body.String())
	}
	CreateRouteSchedule(t, helper, f.busyRoute, WeekdayScheduleBody("07:30", "2026-01-01"))
	return f
}

func getPlanning(t *testing.T, helper *testhelpers.ApiTestHelper, organizationID, query string) models.PlanningOverview {
	t.Helper()
	w := helper.DoRequest("GET", "/tracking/organizations/"+organizationID+"/planning"+query, nil, map[string]string{})
	if w.Code != http.StatusOK {
		t.Fatalf("planning: %d %s", w.Code, w.Body.String())
	}
	var overview models.PlanningOverview
	decodeBody(t, w, &overview)
	return overview
}

// TestPlanning_Overview - 25 riders, 5 outbound only, none routed: the Demanda numbers; the vehicle
// with a 07:30 route is busy for a 07:45 arrival and names that route, the others are free.
func TestPlanning_Overview(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	f := newPlanningFixture(t, helper)

	overview := getPlanning(t, helper, f.organizationID, "?days=31&arrival_time=07:45&departure_time=13:00")
	assert.Equal(t, models.PlanningDemand{Outbound: 25, Return: 20, UnassignedOutbound: 25, UnassignedReturn: 20}, overview.Demand)
	assert.Len(t, overview.Riders, 25)
	byID := map[string]models.FleetVehicle{}
	for _, v := range overview.Fleet {
		byID[v.ID.String()] = v
	}
	busy := byID[f.busyVehicle]
	assert.False(t, busy.FreeOutbound, "the 07:30 run overlaps a 07:45 arrival")
	assert.True(t, busy.FreeReturn)
	assert.Contains(t, string(busy.BusyWith), f.busyRoute)
	for _, id := range f.freeVehicles {
		assert.True(t, byID[id].FreeOutbound, id)
		assert.Equal(t, 15, byID[id].Seats)
	}

	defaults := getPlanning(t, helper, f.organizationID, "")
	assert.Equal(t, "07:45", defaults.ArrivalTime, "no running route: the default arrival")
	assert.Equal(t, "13:00", defaults.DepartureTime)
}

// requireVroom skips when no optimizer answers on GEO_VROOM_URL (`make geo-up` starts it).
func requireVroom(t *testing.T) {
	t.Helper()
	client := http.Client{Timeout: 2 * time.Second}
	res, err := client.Get(config.ModularAppConfig.Geo.VroomURL + "/health")
	if err != nil {
		t.Skipf("VROOM not reachable at %s: %v", config.ModularAppConfig.Geo.VroomURL, err)
	}
	_ = res.Body.Close()
}

// proposeAndSolve creates a run with the free vehicles and solves it in place, as the job does.
func proposeAndSolve(t *testing.T, helper *testhelpers.ApiTestHelper, f planningFixture, maxRide int) models.OptimizationRun {
	t.Helper()
	created := propose(t, helper, f, maxRide)
	repo := trackingRepos.NewTrackingRepository(helper.DB())
	optimizer := geoRouting.NewOptimizer(config.ModularAppConfig.Geo)
	if err := trackingJobs.RunOptimization(context.Background(), repo, optimizer, uuid.MustParse(TestTenantID), created.RunID); err != nil {
		t.Fatalf("solve: %v", err)
	}
	return getRun(t, helper, created.RunID.String())
}

func propose(t *testing.T, helper *testhelpers.ApiTestHelper, f planningFixture, maxRide int) models.OptimizationRunCreated {
	t.Helper()
	w := helper.DoRequest("POST", "/tracking/organizations/"+f.organizationID+"/optimization-runs", map[string]interface{}{
		"days": 31, "arrival_time": "07:45", "departure_time": "13:00",
		"vehicle_ids": f.freeVehicles, "max_ride_min": maxRide,
	}, map[string]string{})
	if w.Code != http.StatusAccepted {
		t.Fatalf("propose: %d %s", w.Code, w.Body.String())
	}
	var created models.OptimizationRunCreated
	decodeBody(t, w, &created)
	return created
}

func getRun(t *testing.T, helper *testhelpers.ApiTestHelper, runID string) models.OptimizationRun {
	t.Helper()
	w := helper.DoRequest("GET", "/tracking/optimization-runs/"+runID, nil, map[string]string{})
	if w.Code != http.StatusOK {
		t.Fatalf("run: %d %s", w.Code, w.Body.String())
	}
	var run models.OptimizationRun
	decodeBody(t, w, &run)
	return run
}

// applyBody is the run's proposal unchanged.
func applyBody(t *testing.T, run models.OptimizationRun) map[string]interface{} {
	t.Helper()
	routes := []map[string]interface{}{}
	for _, r := range run.Routes {
		routes = append(routes, map[string]interface{}{"vehicle_id": r.VehicleID, "rider_ids": r.RiderIDs})
	}
	return map[string]interface{}{"effective_from": dayOffset(t, "UTC", 0), "routes": routes}
}

// TestOptimizationRun_SolveCacheAndOutage - the fixture solved by the local VROOM is done, no route
// over its 15 seats and every rider placed; the same params answer the same run; with VROOM out of
// reach a run fails with geo.optimizer-unavailable.
func TestOptimizationRun_SolveCacheAndOutage(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	requireVroom(t)
	f := newPlanningFixture(t, helper)

	run := proposeAndSolve(t, helper, f, 60)
	if !assert.Equal(t, "done", run.Status, "error: %v", run.ErrorCode) || !assert.NotNil(t, run.OptimizationResult) {
		return
	}
	placed := 0
	for _, r := range run.Routes {
		assert.LessOrEqual(t, len(r.RiderIDs), 15, "no route over its seats")
		assert.Len(t, r.Stops, len(r.RiderIDs))
		placed += len(r.RiderIDs)
		if assert.NotEmpty(t, r.Stops) {
			assert.LessOrEqual(t, r.Stops[len(r.Stops)-1].Eta, "07:45")
		}
	}
	assert.Equal(t, 25, placed+len(run.Unassigned))
	assert.Equal(t, len(run.Routes), run.Metrics.VehiclesUsed)
	assert.Zero(t, run.Metrics.Deferred)

	again := propose(t, helper, f, 60)
	assert.Equal(t, run.ID, again.RunID, "the same problem answers the same run")
	assert.True(t, again.Cached)

	down := propose(t, helper, f, 61)
	repo := trackingRepos.NewTrackingRepository(helper.DB())
	unreachable := *config.ModularAppConfig.Geo
	unreachable.VroomURL = "http://127.0.0.1:1"
	if err := trackingJobs.RunOptimization(context.Background(), repo, geoRouting.NewOptimizer(&unreachable),
		uuid.MustParse(TestTenantID), down.RunID); err != nil {
		t.Fatalf("solve: %v", err)
	}
	failed := getRun(t, helper, down.RunID.String())
	assert.Equal(t, "failed", failed.Status)
	if assert.NotNil(t, failed.ErrorCode) {
		assert.Equal(t, "geo.optimizer-unavailable", *failed.ErrorCode)
	}
}

// TestOptimizationRun_Batch - a proposal capped at 10 riders takes 10 and defers the other 15 to
// the next one.
func TestOptimizationRun_Batch(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	requireVroom(t)
	f := newPlanningFixture(t, helper)
	w := helper.DoRequest("POST", "/tracking/organizations/"+f.organizationID+"/optimization-runs", map[string]interface{}{
		"arrival_time": "07:45", "vehicle_ids": f.freeVehicles, "max_riders": 10,
	}, map[string]string{})
	if w.Code != http.StatusAccepted {
		t.Fatalf("propose: %d %s", w.Code, w.Body.String())
	}
	var created models.OptimizationRunCreated
	decodeBody(t, w, &created)
	repo := trackingRepos.NewTrackingRepository(helper.DB())
	if err := trackingJobs.RunOptimization(context.Background(), repo, geoRouting.NewOptimizer(config.ModularAppConfig.Geo),
		uuid.MustParse(TestTenantID), created.RunID); err != nil {
		t.Fatalf("solve: %v", err)
	}
	run := getRun(t, helper, created.RunID.String())
	if !assert.Equal(t, "done", run.Status) || !assert.NotNil(t, run.OptimizationResult) {
		return
	}
	placed := len(run.Unassigned)
	for _, r := range run.Routes {
		placed += len(r.RiderIDs)
	}
	assert.Equal(t, 10, placed, "the batch")
	assert.Equal(t, 15, run.Metrics.Deferred, "left for the next proposal")
}

// TestOptimizationRun_Apply - applying a run creates its route pairs and assignments; applying it
// again is a 409; a second proposal over the same riders, applied after the first, is a 409 naming a
// rider assigned meanwhile, and writes nothing.
func TestOptimizationRun_Apply(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	requireVroom(t)
	f := newPlanningFixture(t, helper)
	first := proposeAndSolve(t, helper, f, 60)
	second := proposeAndSolve(t, helper, f, 59)
	if first.Status != "done" || second.Status != "done" {
		t.Fatalf("runs not done: %s %s", first.Status, second.Status)
	}

	w := helper.DoRequest("POST", "/tracking/optimization-runs/"+first.ID.String()+"/apply", applyBody(t, first), map[string]string{})
	if !assert.Equal(t, http.StatusCreated, w.Code, w.Body.String()) {
		return
	}
	var applied models.ApplyOptimizationRunResponse
	decodeBody(t, w, &applied)
	assert.Equal(t, len(first.Routes), applied.RoutesCreated)
	assert.Equal(t, 25-len(first.Unassigned), applied.AssignmentsCreated)

	overview := getPlanning(t, helper, f.organizationID, "?days=31&arrival_time=07:45&departure_time=13:00")
	assert.Equal(t, len(first.Unassigned), overview.Demand.UnassignedOutbound)
	routesBefore := map[uuid.UUID]bool{}
	for _, r := range overview.Riders {
		if r.OutboundRouteID != nil {
			routesBefore[*r.OutboundRouteID] = true
		}
	}
	assert.Len(t, routesBefore, len(first.Routes), "one outbound route per proposed vehicle")

	w = helper.DoRequest("POST", "/tracking/optimization-runs/"+first.ID.String()+"/apply", applyBody(t, first), map[string]string{})
	assert.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "tracking.optimization.applied")

	w = helper.DoRequest("POST", "/tracking/optimization-runs/"+second.ID.String()+"/apply", applyBody(t, second), map[string]string{})
	assert.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "tracking.optimization.stale")
	assert.Contains(t, w.Body.String(), "rider_id")
	routesAfter := map[uuid.UUID]bool{}
	for _, r := range getPlanning(t, helper, f.organizationID, "?days=31&arrival_time=07:45&departure_time=13:00").Riders {
		if r.OutboundRouteID != nil {
			routesAfter[*r.OutboundRouteID] = true
		}
	}
	assert.Equal(t, routesBefore, routesAfter, "the refused apply wrote nothing")
}
