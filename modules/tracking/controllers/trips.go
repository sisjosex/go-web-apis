package controllers

import (
	"errors"
	"net/http"

	coreErrors "josex/web/modules/core/errors"
	"josex/web/modules/core/utils"
	trackingErrors "josex/web/modules/tracking/errors"
	"josex/web/modules/tracking/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// The trip handlers of TRACK-020, beside tracking_controller.go: one slice, one file.

// tripStatus is the status each trip code answers; a code not listed is a 500.
var tripStatus = map[string]int{
	trackingErrors.TripNotFound:          http.StatusNotFound,
	trackingErrors.TripStopNotFound:      http.StatusNotFound,
	trackingErrors.TripTaskNotFound:      http.StatusNotFound,
	trackingErrors.TripInvalidTransition: http.StatusConflict,
	trackingErrors.TripTaskTripNotActive: http.StatusConflict,
	trackingErrors.TripTaskOpConflict:    http.StatusConflict,
	trackingErrors.RouteCompanyMismatch:  http.StatusBadRequest,
}

// The actions each path's last segment may name, as the SPs spell them.
var (
	tripActions     = map[string]string{"start": "start", "complete": "complete", "cancel": "cancel"}
	tripStopActions = map[string]string{"arrive": "arrive", "skip": "skip"}
	tripTaskActions = map[string]string{"done": "done", "no-show": "no_show"}
)

// tripErrorResponse answers the trip codes with their status and anything else with a 500. company-mismatch carries the offending field as its detail.
func tripErrorResponse(c *gin.Context, err error) {
	var trackingErr *trackingErrors.TrackingError
	if errors.As(err, &trackingErr) {
		if status, ok := tripStatus[trackingErr.Code]; ok {
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

// pathUUID parses a path id, answering 400 when it is not one.
func pathUUID(c *gin.Context, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param(name))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
		return uuid.Nil, false
	}
	return id, true
}

// pathAction resolves the :action segment against one of the maps above; an action not listed is
// a path that does not exist, answered as gin answers any unknown path.
func pathAction(c *gin.Context, actions map[string]string) (string, bool) {
	action, ok := actions[c.Param("action")]
	if !ok {
		c.AbortWithStatus(http.StatusNotFound)
	}
	return action, ok
}

// ListTrips godoc
// @Summary List a day's trips
// @Description By planned start. date empty is each route's own today; organization_id narrows to the trips carrying one of its riders. Every row carries the route, plate and driver names, stops_count, tasks_total (not cancelled) and tasks_done (done or no_show)
// @Tags Tracking - Trips
// @Produce json
// @Security BearerAuth
// @Param date query string false "Service date (YYYY-MM-DD)"
// @Param route_id query string false "Route ID (UUID)"
// @Param status query string false "planned, in_progress, completed or cancelled"
// @Param organization_id query string false "Organization ID (UUID)"
// @Param page query int false "Page number (default 1)"
// @Param page_size query int false "Page size (default 20, max 100)"
// @Success 200 {object} models.ListTripsResponse
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/trips [get]
func (ctrl *TrackingController) ListTrips(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}
	var query models.ListTripsQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.TripListFailed, utils.ExtractValidationError(c, err)))
		return
	}

	result, err := ctrl.trackingService.ListTrips(c.Request.Context(), tenantID, query)
	if err != nil {
		tripErrorResponse(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// GetTrip godoc
// @Summary Get a trip with its stops and tasks
// @Tags Tracking - Trips
// @Produce json
// @Security BearerAuth
// @Param trip_id path string true "Trip ID (UUID)"
// @Success 200 {object} models.TripDetail
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/trips/{trip_id} [get]
func (ctrl *TrackingController) GetTrip(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}
	tripID, ok := pathUUID(c, "trip_id")
	if !ok {
		return
	}

	result, err := ctrl.trackingService.GetTrip(c.Request.Context(), tenantID, tripID)
	if err != nil {
		tripErrorResponse(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// GetTripStatus godoc
// @Summary Get a trip's live status
// @Description Rider counts, the next pending stop, the seconds it has been due, and the vehicle's last position
// @Tags Tracking - Trips
// @Produce json
// @Security BearerAuth
// @Param trip_id path string true "Trip ID (UUID)"
// @Success 200 {object} models.TripStatus
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/trips/{trip_id}/status [get]
func (ctrl *TrackingController) GetTripStatus(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}
	tripID, ok := pathUUID(c, "trip_id")
	if !ok {
		return
	}

	result, err := ctrl.trackingService.GetTripStatus(c.Request.Context(), tenantID, tripID)
	if err != nil {
		tripErrorResponse(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// UpdateTrip godoc
// @Summary Override a trip
// @Description Vehicle, driver or planned start of a trip not ended; a field left out keeps its value. The trip becomes is_overridden and the materialiser keeps its header
// @Tags Tracking - Trips
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param trip_id path string true "Trip ID (UUID)"
// @Param trip body models.UpdateTripDto true "The fields to change"
// @Success 200 {object} models.TripDetail
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 409 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/trips/{trip_id} [patch]
func (ctrl *TrackingController) UpdateTrip(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}
	tripID, ok := pathUUID(c, "trip_id")
	if !ok {
		return
	}
	var dto models.UpdateTripDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.TripUpdateFailed, utils.ExtractValidationError(c, err)))
		return
	}

	result, err := ctrl.trackingService.UpdateTrip(c.Request.Context(), tenantID, tripID, &dto, ctrl.actingUserID(c))
	if err != nil {
		tripErrorResponse(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// TransitionTrip godoc
// @Summary Start, complete or cancel a trip
// @Description start: planned → in_progress · complete: in_progress → completed · cancel: planned or in_progress → cancelled, its pending tasks cancelled and the trip is_overridden
// @Tags Tracking - Trips
// @Produce json
// @Security BearerAuth
// @Param trip_id path string true "Trip ID (UUID)"
// @Param action path string true "The transition" Enums(start, complete, cancel)
// @Success 200 {object} models.TripDetail
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 409 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/trips/{trip_id}/{action} [post]
func (ctrl *TrackingController) TransitionTrip(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}
	tripID, ok := pathUUID(c, "trip_id")
	if !ok {
		return
	}
	action, ok := pathAction(c, tripActions)
	if !ok {
		return
	}

	result, err := ctrl.trackingService.TransitionTrip(c.Request.Context(), tenantID, tripID, action, ctrl.actingUserID(c))
	if err != nil {
		tripErrorResponse(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// TransitionTripStop godoc
// @Summary Arrive at or skip a stop
// @Description Of an in-progress trip. skip turns the stop's pending pickups into no_show and its pending dropoffs into cancelled
// @Tags Tracking - Trips
// @Produce json
// @Security BearerAuth
// @Param stop_id path string true "Trip stop ID (UUID)"
// @Param action path string true "The transition" Enums(arrive, skip)
// @Success 200 {object} models.TripDetail
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 409 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/trip-stops/{stop_id}/{action} [post]
func (ctrl *TrackingController) TransitionTripStop(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}
	stopID, ok := pathUUID(c, "stop_id")
	if !ok {
		return
	}
	action, ok := pathAction(c, tripStopActions)
	if !ok {
		return
	}

	result, err := ctrl.trackingService.TransitionTripStop(c.Request.Context(), tenantID, stopID, action, ctrl.actingUserID(c))
	if err != nil {
		tripErrorResponse(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// TransitionTripTask godoc
// @Summary Mark a pickup or dropoff done, or the rider a no-show
// @Description Of an in-progress trip, at a stop not skipped. Idempotent on client_op_id: the same id again answers 200 and changes nothing; an id already used on another task is 409
// @Tags Tracking - Trips
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param task_id path string true "Trip stop task ID (UUID)"
// @Param action path string true "The transition" Enums(done, no-show)
// @Param transition body models.TripTaskTransitionDto true "client_op_id and an optional proof"
// @Success 200 {object} models.TripDetail
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 409 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/trip-stop-tasks/{task_id}/{action} [post]
func (ctrl *TrackingController) TransitionTripTask(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}
	taskID, ok := pathUUID(c, "task_id")
	if !ok {
		return
	}
	action, ok := pathAction(c, tripTaskActions)
	if !ok {
		return
	}
	var dto models.TripTaskTransitionDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.TripTransitionFailed, utils.ExtractValidationError(c, err)))
		return
	}

	result, err := ctrl.trackingService.TransitionTripTask(c.Request.Context(), tenantID, taskID, action, &dto, ctrl.actingUserID(c))
	if err != nil {
		tripErrorResponse(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}
