package controllers

import (
	"errors"
	"net/http"

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
func planningErrorResponse(c *gin.Context, err error) {
	var trackingErr *trackingErrors.TrackingError
	if errors.As(err, &trackingErr) {
		if status, ok := planningStatus[trackingErr.Code]; ok {
			if trackingErr.Detail != nil {
				c.JSON(status, coreErrors.BuildErrorDetail(c, trackingErr.Code, trackingErr.Detail))
			} else {
				c.JSON(status, coreErrors.BuildErrorSingle(c, trackingErr.Code))
			}
			return
		}
	}
	c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
}

// GetPlanning godoc
// @Summary A destination's planning screen
// @Description Its riders (pickup point, legs, route each way), the Demanda counts and every vehicle free or busy at the planned hours, in one answer (TRACK-014 D2). An hour left out is the organization's running one, else 07:45 / 13:00
// @Tags Tracking - Planning
// @Produce json
// @Security BearerAuth
// @Param organization_id path string true "Organization ID (UUID)"
// @Param days query int false "Weekdays bitmask, bit 0 Monday (default 31: Mon–Fri)"
// @Param arrival_time query string false "Arrival at the destination, HH:MM"
// @Param departure_time query string false "Departure from the destination, HH:MM"
// @Success 200 {object} models.PlanningOverview
// @Failure 404 {object} coreErrors.ErrorResponse
// @Router /tracking/organizations/{organization_id}/planning [get]
func (ctrl *TrackingController) GetPlanning(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}
	organizationID, ok := pathUUID(c, "organization_id")
	if !ok {
		return
	}
	var query models.PlanningQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.PlanningFailed, utils.ExtractValidationError(c, err)))
		return
	}
	result, err := ctrl.trackingService.PlanningOverview(c.Request.Context(), tenantID, organizationID, &query)
	if err != nil {
		planningErrorResponse(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// CreateOptimizationRun godoc
// @Summary Propose routes for a destination's riders without one
// @Description Queues a VROOM run over the riders still without a route in the legs served and the chosen vehicles; the same problem within 24 h answers its earlier run (cached). Poll GET /tracking/optimization-runs/{id}
// @Tags Tracking - Planning
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param organization_id path string true "Organization ID (UUID)"
// @Param body body models.CreateOptimizationRunDto true "Days, hours, vehicles and the longest ride"
// @Success 202 {object} models.OptimizationRunCreated
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 409 {object} coreErrors.ErrorResponse
// @Router /tracking/organizations/{organization_id}/optimization-runs [post]
func (ctrl *TrackingController) CreateOptimizationRun(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}
	organizationID, ok := pathUUID(c, "organization_id")
	if !ok {
		return
	}
	var dto models.CreateOptimizationRunDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.OptimizationFailed, utils.ExtractValidationError(c, err)))
		return
	}
	result, err := ctrl.trackingService.CreateOptimizationRun(c.Request.Context(), tenantID, organizationID, &dto, ctrl.actingUserID(c))
	if err != nil {
		planningErrorResponse(c, err)
		return
	}
	c.JSON(http.StatusAccepted, result)
}

// GetOptimizationRun godoc
// @Summary A route proposal
// @Description queued|running, then done with its routes (vehicle, riders in order, stops with ETA, km, longest ride) and unassigned riders with their reason, or failed with error_code
// @Tags Tracking - Planning
// @Produce json
// @Security BearerAuth
// @Param run_id path string true "Run ID (UUID)"
// @Success 200 {object} models.OptimizationRun
// @Failure 404 {object} coreErrors.ErrorResponse
// @Router /tracking/optimization-runs/{run_id} [get]
func (ctrl *TrackingController) GetOptimizationRun(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}
	runID, ok := pathUUID(c, "run_id")
	if !ok {
		return
	}
	result, err := ctrl.trackingService.GetOptimizationRun(c.Request.Context(), tenantID, runID)
	if err != nil {
		planningErrorResponse(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// ApplyOptimizationRun godoc
// @Summary Apply a route proposal as adjusted
// @Description One route pair per vehicle with its riders assigned from effective_from, in one transaction (TRACK-014 D4): a rider that can no longer be assigned aborts it all and is named
// @Tags Tracking - Planning
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param run_id path string true "Run ID (UUID)"
// @Param body body models.ApplyOptimizationRunDto true "The routes as left by the operator"
// @Success 201 {object} models.ApplyOptimizationRunResponse
// @Failure 409 {object} coreErrors.ErrorResponse
// @Router /tracking/optimization-runs/{run_id}/apply [post]
func (ctrl *TrackingController) ApplyOptimizationRun(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}
	runID, ok := pathUUID(c, "run_id")
	if !ok {
		return
	}
	var dto models.ApplyOptimizationRunDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.OptimizationApplyFailed, utils.ExtractValidationError(c, err)))
		return
	}
	result, err := ctrl.trackingService.ApplyOptimizationRun(c.Request.Context(), tenantID, runID, &dto, ctrl.actingUserID(c))
	if err != nil {
		planningErrorResponse(c, err)
		return
	}
	c.JSON(http.StatusCreated, result)
}
