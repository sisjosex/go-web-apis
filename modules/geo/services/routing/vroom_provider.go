package routing

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	geoConfig "josex/web/modules/geo/config"
	"josex/web/modules/geo/interfaces"
	"josex/web/modules/geo/models"

	"github.com/sony/gobreaker/v2"
)

// NewOptimizer is the route optimizer (TRACK-014 D1): vroom-express, which routes through Valhalla.
func NewOptimizer(cfg *geoConfig.GeoConfig) interfaces.Optimizer {
	return newVroomProvider(cfg.VroomURL, cfg.OptimizeTimeout)
}

type vroomProvider struct {
	baseURL string
	client  *http.Client
	// Its own breaker, on Valhalla's policy: a solve failing does not stop route and trace calls.
	breaker *gobreaker.CircuitBreaker[any]
}

func newVroomProvider(baseURL string, timeout time.Duration) *vroomProvider {
	return &vroomProvider{
		baseURL: baseURL,
		client:  &http.Client{Timeout: timeout},
		breaker: gobreaker.NewCircuitBreaker[any](gobreaker.Settings{
			Name:        "vroom",
			Timeout:     breakerOpenFor,
			ReadyToTrip: func(c gobreaker.Counts) bool { return c.ConsecutiveFailures >= breakerFailures },
		}),
	}
}

type vroomVehicle struct {
	ID            int        `json:"id"`
	Profile       string     `json:"profile"`
	End           [2]float64 `json:"end"`
	Capacity      []int      `json:"capacity"`
	TimeWindow    [2]int     `json:"time_window"`
	MaxTravelTime int        `json:"max_travel_time,omitempty"`
}

type vroomJob struct {
	ID       int        `json:"id"`
	Location [2]float64 `json:"location"`
	Pickup   []int      `json:"pickup"`
	Service  int        `json:"service"`
}

type vroomRequest struct {
	Vehicles []vroomVehicle `json:"vehicles"`
	Jobs     []vroomJob     `json:"jobs"`
	// g asks for the routes' geometry, which is what makes VROOM report their distance.
	Options struct {
		G bool `json:"g"`
	} `json:"options"`
}

type vroomResponse struct {
	Code       int `json:"code"`
	Unassigned []struct {
		ID int `json:"id"`
	} `json:"unassigned"`
	Routes []struct {
		Vehicle  int     `json:"vehicle"`
		Duration int     `json:"duration"`
		Distance float64 `json:"distance"`
		Steps    []struct {
			Type    string `json:"type"`
			Job     int    `json:"job"`
			Arrival int    `json:"arrival"`
		} `json:"steps"`
	} `json:"routes"`
}

func (p *vroomProvider) Optimize(ctx context.Context, problem *models.OptimizeProblem) (*models.OptimizeSolution, error) {
	res, err := p.breaker.Execute(func() (any, error) { return p.optimize(ctx, problem) })
	if err != nil {
		return nil, fmt.Errorf("%w: %w", interfaces.ErrOptimizerUnavailable, err)
	}
	solution, _ := res.(*models.OptimizeSolution)
	return solution, nil
}

func (p *vroomProvider) optimize(ctx context.Context, problem *models.OptimizeProblem) (*models.OptimizeSolution, error) {
	body := vroomRequest{
		Vehicles: make([]vroomVehicle, 0, len(problem.Vehicles)),
		Jobs:     make([]vroomJob, 0, len(problem.Jobs)),
	}
	body.Options.G = true
	for _, v := range problem.Vehicles {
		body.Vehicles = append(body.Vehicles, vroomVehicle{
			ID: v.ID, Profile: problem.Costing, End: [2]float64{v.End.Lng, v.End.Lat},
			Capacity: []int{v.Capacity}, TimeWindow: v.TimeWindow, MaxTravelTime: v.MaxTravelS,
		})
	}
	for _, j := range problem.Jobs {
		body.Jobs = append(body.Jobs, vroomJob{
			ID: j.ID, Location: [2]float64{j.Location.Lng, j.Location.Lat}, Pickup: []int{1}, Service: j.ServiceS,
		})
	}
	var out vroomResponse
	if err := p.post(ctx, body, &out); err != nil {
		return nil, err
	}
	if out.Code != 0 {
		return nil, fmt.Errorf("vroom answered code %d", out.Code)
	}
	return toSolution(&out), nil
}

// post sends the problem; anything but a 2xx — VROOM's input, routing and internal errors alike — is
// a failure the caller cannot fix by retrying with the same problem, so it counts toward the breaker.
func (p *vroomProvider) post(ctx context.Context, body, out any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode >= http.StatusBadRequest {
		_, _ = io.Copy(io.Discard, res.Body)
		return fmt.Errorf("vroom answered %d", res.StatusCode)
	}
	return json.NewDecoder(res.Body).Decode(out)
}

func toSolution(out *vroomResponse) *models.OptimizeSolution {
	solution := &models.OptimizeSolution{Routes: make([]models.OptimizeRoute, 0, len(out.Routes))}
	for _, r := range out.Routes {
		route := models.OptimizeRoute{VehicleID: r.Vehicle, DistanceM: r.Distance}
		for _, s := range r.Steps {
			switch s.Type {
			case "job", "pickup":
				route.Stops = append(route.Stops, models.OptimizeStop{JobID: s.Job, ArrivalS: s.Arrival})
			case "end":
				route.EndS = s.Arrival
			}
		}
		if len(route.Stops) > 0 {
			route.DurationS = route.EndS - route.Stops[0].ArrivalS
		}
		solution.Routes = append(solution.Routes, route)
	}
	for _, u := range out.Unassigned {
		solution.Unassigned = append(solution.Unassigned, u.ID)
	}
	return solution
}
