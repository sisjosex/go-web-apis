package controllers

import (
	"context"
	"errors"
	"net/http"

	"josex/web/modules/core/api"
	coreErrors "josex/web/modules/core/errors"
	"josex/web/modules/core/utils"
	trackingErrors "josex/web/modules/tracking/errors"
	"josex/web/modules/tracking/models"

	"github.com/gin-gonic/gin"
)

// The planning handlers of TRACK-014: the screen's read, a proposal's run and its apply.

// planningStatus is the status each planning code answers; a code not listed is a 500.
var planningStatus = map[string]int{
	trackingErrors.OrganizationNotFound:        http.StatusNotFound,
	trackingErrors.OptimizationNotFound:        http.StatusNotFound,
	trackingErrors.VehicleNotFound:             http.StatusNotFound,
	trackingErrors.RouteDestinationRequired:    http.StatusBadRequest,
	trackingErrors.OptimizationNoVehicles:      http.StatusBadRequest,
	trackingErrors.OptimizationVehicleRepeated: http.StatusBadRequest,
	trackingErrors.OptimizationNoRiders:        http.StatusConflict,
	trackingErrors.OptimizationApplied:         http.StatusConflict,
	trackingErrors.OptimizationNotReady:        http.StatusConflict,
	trackingErrors.OptimizationStale:           http.StatusConflict,
}

// planningErrorResponse answers the planning codes with their status, a stale apply with the rider it
// names, and anything else with a 500.
func planningErrorResponse(c *gin.Context, err error) error {
	var trackingErr *trackingErrors.TrackingError
	if errors.As(err, &trackingErr) {
		if status, ok := planningStatus[trackingErr.Code]; ok {
			if trackingErr.Detail != nil {
				return api.Fail(status, coreErrors.BuildErrorDetail(c, trackingErr.Code, trackingErr.Detail))
			} else {
				return api.Fail(status, coreErrors.BuildErrorSingle(c, trackingErr.Code))
			}

		}
	}
	return api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
}

// GetPlanningInput is GetPlanning's request.
type GetPlanningInput struct {
	OrganizationID string `path:"organization_id"`
	Query          api.Query[models.PlanningQuery]
}

// GetPlanning is GET /organizations/{organization_id}/planning: A destination's planning screen.
func (ctrl *TrackingController) GetPlanning(ctx context.Context, in *GetPlanningInput) (*api.Out[*models.PlanningOverview], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}
	organizationID, err := pathUUID(c, "organization_id")
	if err != nil {
		return nil, err
	}
	var query models.PlanningQuery
	if err := in.Query.Into(ctx, &query); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.PlanningFailed, utils.ExtractValidationError(c, err)))
	}
	result, err := ctrl.trackingService.PlanningOverview(c.Request.Context(), tenantID, organizationID, &query)
	if err != nil {
		return nil, planningErrorResponse(c, err)
	}
	return api.OK(result)
}

// CreateOptimizationRunInput is CreateOptimizationRun's request.
type CreateOptimizationRunInput struct {
	OrganizationID string `path:"organization_id"`
	JSON           api.Body[models.CreateOptimizationRunDto]
}

// CreateOptimizationRun is POST /organizations/{organization_id}/optimization-runs: Propose routes for a destination's riders without one.
func (ctrl *TrackingController) CreateOptimizationRun(ctx context.Context, in *CreateOptimizationRunInput) (*api.Out[*models.OptimizationRunCreated], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}
	organizationID, err := pathUUID(c, "organization_id")
	if err != nil {
		return nil, err
	}
	var dto models.CreateOptimizationRunDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.OptimizationFailed, utils.ExtractValidationError(c, err)))
	}
	result, err := ctrl.trackingService.CreateOptimizationRun(c.Request.Context(), tenantID, organizationID, &dto, ctrl.actingUserID(c))
	if err != nil {
		return nil, planningErrorResponse(c, err)
	}
	return api.OK(result)
}

// GetOptimizationRunInput is GetOptimizationRun's request.
type GetOptimizationRunInput struct {
	RunID string `path:"run_id"`
}

// GetOptimizationRun is GET /optimization-runs/{run_id}: A route proposal.
func (ctrl *TrackingController) GetOptimizationRun(ctx context.Context, _ *GetOptimizationRunInput) (*api.Out[*models.OptimizationRun], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}
	runID, err := pathUUID(c, "run_id")
	if err != nil {
		return nil, err
	}
	result, err := ctrl.trackingService.GetOptimizationRun(c.Request.Context(), tenantID, runID)
	if err != nil {
		return nil, planningErrorResponse(c, err)
	}
	return api.OK(result)
}

// ApplyOptimizationRunInput is ApplyOptimizationRun's request.
type ApplyOptimizationRunInput struct {
	RunID string `path:"run_id"`
	JSON  api.Body[models.ApplyOptimizationRunDto]
}

// ApplyOptimizationRun is POST /optimization-runs/{run_id}/apply: Apply a route proposal as adjusted.
func (ctrl *TrackingController) ApplyOptimizationRun(ctx context.Context, in *ApplyOptimizationRunInput) (*api.Out[*models.ApplyOptimizationRunResponse], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}
	runID, err := pathUUID(c, "run_id")
	if err != nil {
		return nil, err
	}
	var dto models.ApplyOptimizationRunDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.OptimizationApplyFailed, utils.ExtractValidationError(c, err)))
	}
	result, err := ctrl.trackingService.ApplyOptimizationRun(c.Request.Context(), tenantID, runID, &dto, ctrl.actingUserID(c))
	if err != nil {
		return nil, planningErrorResponse(c, err)
	}
	return api.OK(result)
}
