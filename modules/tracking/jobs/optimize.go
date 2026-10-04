package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"time"

	coreJobs "josex/web/modules/core/jobs"
	coreServices "josex/web/modules/core/services"
	geoErrors "josex/web/modules/geo/errors"
	geoInterfaces "josex/web/modules/geo/interfaces"
	geoModels "josex/web/modules/geo/models"
	trackingErrors "josex/web/modules/tracking/errors"
	"josex/web/modules/tracking/models"
	trackingRepos "josex/web/modules/tracking/repositories"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

// topicOptimizationRequested is the outbox row sp_create_optimization_run writes with a new run.
const topicOptimizationRequested = "optimization.requested"

// TaskOptimize is the outbox task that row becomes: solve the run (TRACK-014 step 2).
var TaskOptimize = coreJobs.OutboxTask(topicOptimizationRequested)

// The problem's fixed terms: a minute to board, and the vehicles' day opening four hours before the
// arrival — any route that fits the longest ride starts inside it.
const (
	boardingSeconds = 60
	windowSeconds   = 4 * 3600
)

// OptimizeHandler solves a run: claims it, asks the optimizer, and records routes and unassigned
// riders, or failed with geo.optimizer-unavailable when VROOM is down. A run is final either way —
// the operator proposes again — so only a database failure is retried.
func OptimizeHandler(db coreServices.DatabaseService, tenants *coreJobs.TenantDirectory, optimizer geoInterfaces.Optimizer) asynq.HandlerFunc {
	return func(ctx context.Context, task *asynq.Task) error {
		var payload struct {
			RunID uuid.UUID `json:"run_id"`
		}
		if err := json.Unmarshal(task.Payload(), &payload); err != nil {
			return fmt.Errorf("optimization.requested payload: %v: %w", err, asynq.SkipRetry)
		}
		tenant, ok, err := taskTenant(ctx, tenants)
		if err != nil || !ok {
			return err
		}
		tenantID, err := uuid.Parse(tenant.ID)
		if err != nil {
			return fmt.Errorf("tenant id %q: %v: %w", tenant.ID, err, asynq.SkipRetry)
		}
		tenantCtx := context.WithValue(ctx, coreServices.TenantDatabaseURLKey, tenant.DatabaseURL)
		return RunOptimization(tenantCtx, trackingRepos.NewTrackingRepository(db), optimizer, tenantID, payload.RunID)
	}
}

// RunOptimization is the handler's work for one run, callable without a queue.
func RunOptimization(ctx context.Context, repo *trackingRepos.TrackingRepository, optimizer geoInterfaces.Optimizer, tenantID, runID uuid.UUID) error {
	input, err := repo.ClaimOptimizationRun(ctx, tenantID, runID)
	if err != nil || input == nil {
		return err
	}
	problem, err := optimizationProblem(input)
	if err != nil {
		return failRun(ctx, repo, tenantID, runID, trackingErrors.OptimizationFailed)
	}
	started := time.Now()
	solution, err := optimizer.Optimize(ctx, problem)
	if err != nil {
		log.Printf("⚠️  optimization %s: %v", runID, err)
		code := trackingErrors.OptimizationFailed
		if errors.Is(err, geoInterfaces.ErrOptimizerUnavailable) {
			code = geoErrors.OptimizerUnavailable
		}
		return failRun(ctx, repo, tenantID, runID, code)
	}
	log.Printf("✅ optimization %s: %d riders, %d vehicles in %s", runID, len(input.Riders), len(input.Vehicles), time.Since(started).Round(time.Millisecond))
	return repo.FinishOptimizationRun(ctx, tenantID, runID, "done", nil, optimizationResult(input, problem, solution))
}

func failRun(ctx context.Context, repo *trackingRepos.TrackingRepository, tenantID, runID uuid.UUID, code string) error {
	return repo.FinishOptimizationRun(ctx, tenantID, runID, "failed", &code, nil)
}

// optimizationProblem turns a run's input into the optimizer's terms: vehicle i+1 and rider j+1, each
// vehicle ending at the destination by the arrival time, carrying its seats, its whole drive no longer
// than the longest ride — with an open start that is the first rider's ride.
func optimizationProblem(input *models.OptimizationInput) (*geoModels.OptimizeProblem, error) {
	arrival, err := clockSeconds(input.ArrivalTime)
	if err != nil {
		return nil, err
	}
	end := geoModels.LatLng{Lat: input.Destination.Lat, Lng: input.Destination.Lng}
	problem := &geoModels.OptimizeProblem{
		Costing:  "bus",
		Vehicles: make([]geoModels.OptimizeVehicle, 0, len(input.Vehicles)),
		Jobs:     make([]geoModels.OptimizeJob, 0, len(input.Riders)),
	}
	for i, v := range input.Vehicles {
		problem.Vehicles = append(problem.Vehicles, geoModels.OptimizeVehicle{
			ID: i + 1, End: end, Capacity: v.Seats,
			TimeWindow: [2]int{max(0, arrival-windowSeconds), arrival},
			MaxTravelS: input.MaxRideMin * 60,
		})
	}
	for j, r := range input.Riders {
		problem.Jobs = append(problem.Jobs, geoModels.OptimizeJob{
			ID: j + 1, Location: geoModels.LatLng{Lat: r.Lat, Lng: r.Lng}, ServiceS: boardingSeconds,
		})
	}
	return problem, nil
}

// optimizationResult maps the solution back to vehicles and riders. VROOM starts each route as early
// as its window allows; the times are shifted so every route reaches the destination at the arrival
// time, which is when the schedule will run it.
func optimizationResult(input *models.OptimizationInput, problem *geoModels.OptimizeProblem, solution *geoModels.OptimizeSolution) *models.OptimizationResult {
	arrival := problem.Vehicles[0].TimeWindow[1]
	result := &models.OptimizationResult{Routes: []models.ProposedRoute{}, Unassigned: []models.UnassignedRider{}}
	for _, r := range solution.Routes {
		shift := arrival - r.EndS
		route := models.ProposedRoute{
			VehicleID: input.Vehicles[r.VehicleID-1].ID,
			Km:        math.Round(r.DistanceM/100) / 10,
			RideMin:   int(math.Ceil(float64(r.DurationS) / 60)),
		}
		for _, s := range r.Stops {
			rider := input.Riders[s.JobID-1]
			route.RiderIDs = append(route.RiderIDs, rider.ID)
			route.Stops = append(route.Stops, models.ProposedStop{
				RiderID: rider.ID, Lat: rider.Lat, Lng: rider.Lng, Eta: clockLabel(s.ArrivalS + shift),
			})
		}
		result.Routes = append(result.Routes, route)
		result.Metrics.TotalKm += route.Km
		result.Metrics.MaxRideMin = max(result.Metrics.MaxRideMin, route.RideMin)
	}
	result.Metrics.VehiclesUsed = len(result.Routes)
	result.Metrics.Deferred = input.Deferred
	result.Metrics.TotalKm = math.Round(result.Metrics.TotalKm*10) / 10
	reason := unassignedReason(input)
	for _, id := range solution.Unassigned {
		result.Unassigned = append(result.Unassigned, models.UnassignedRider{RiderID: input.Riders[id-1].ID, Reason: reason})
	}
	return result
}

// unassignedReason is why VROOM left riders out, which it does not say: more riders than seats is
// `capacity`; otherwise they lie beyond the longest ride, `max-ride`.
func unassignedReason(input *models.OptimizationInput) string {
	seats := 0
	for _, v := range input.Vehicles {
		seats += v.Seats
	}
	if len(input.Riders) > seats {
		return "capacity"
	}
	return "max-ride"
}

func clockSeconds(hhmm string) (int, error) {
	t, err := time.Parse("15:04", hhmm)
	if err != nil {
		return 0, err
	}
	return t.Hour()*3600 + t.Minute()*60, nil
}

func clockLabel(seconds int) string {
	seconds = ((seconds % 86400) + 86400) % 86400
	return fmt.Sprintf("%02d:%02d", seconds/3600, seconds%3600/60)
}
