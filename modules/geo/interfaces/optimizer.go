package interfaces

import (
	"context"
	"errors"

	"josex/web/modules/geo/models"
)

// Optimizer is the route-optimization port (TRACK-014 D1): VROOM today. It shares Valhalla's road
// network but not its breaker — a solve can fail while routing works.
type Optimizer interface {
	// Optimize assigns the jobs to the vehicles and orders each vehicle's stops, or answers
	// ErrOptimizerUnavailable.
	Optimize(ctx context.Context, problem *models.OptimizeProblem) (*models.OptimizeSolution, error)
}

// ErrOptimizerUnavailable is the optimizer down, slow, refusing the problem or behind an open breaker.
var ErrOptimizerUnavailable = errors.New("geo: optimizer unavailable")
