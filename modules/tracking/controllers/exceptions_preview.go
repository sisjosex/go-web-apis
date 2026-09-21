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

// The exception and preview handlers of TRACK-018.

// exceptionErrorResponse maps the exception and preview codes to statuses and reports whether it has
// already written the response.
func exceptionErrorResponse(c *gin.Context, err error) bool {
	var trackingErr *trackingErrors.TrackingError
	if !errors.As(err, &trackingErr) {
		return false
	}
	switch trackingErr.Code {
	case trackingErrors.RouteNotFound, trackingErrors.ExceptionNotFound,
		trackingErrors.VehicleNotFound, trackingErrors.DriverNotFound, trackingErrors.StopPlaceNotFound:
		c.JSON(http.StatusNotFound, coreErrors.BuildErrorSingle(c, trackingErr.Code))
		return true
	case trackingErrors.ExceptionPayload, trackingErrors.RoutePreviewRange:
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, trackingErr.Code))
		return true
	}
	return false
}

// ListRouteExceptions godoc
// @Summary List a route's exceptions
// @Description What happens differently on one day or a few; an exception overlapping the window is part of the answer
// @Tags Tracking - Exceptions
// @Produce json
// @Security BearerAuth
// @Param route_id path string true "Route ID (UUID)"
// @Param date_from query string false "Earliest date (YYYY-MM-DD)"
// @Param date_to query string false "Latest date (YYYY-MM-DD)"
// @Success 200 {array} models.RouteException
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/routes/{route_id}/exceptions [get]
func (ctrl *TrackingController) ListRouteExceptions(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}
	routeID, err := uuid.Parse(c.Param("route_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
		return
	}
	var query models.ListRouteExceptionsQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.ExceptionListFailed, utils.ExtractValidationError(c, err)))
		return
	}

	exceptions, err := ctrl.trackingService.ListRouteExceptions(c.Request.Context(), tenantID, routeID, query.DateFrom, query.DateTo)
	if err != nil {
		if exceptionErrorResponse(c, err) {
			return
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.JSON(http.StatusOK, exceptions)
}

// CreateRouteException godoc
// @Summary Record what happens differently over a range of dates
// @Description kind is one of cancel, change_time, change_vehicle, change_driver, skip_stop, add_stop, detour; payload carries exactly the keys that kind defines
// @Tags Tracking - Exceptions
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param route_id path string true "Route ID (UUID)"
// @Param exception body models.CreateRouteExceptionDto true "The dates, the kind and its payload"
// @Success 201 {object} models.RouteException
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/routes/{route_id}/exceptions [post]
func (ctrl *TrackingController) CreateRouteException(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}
	routeID, err := uuid.Parse(c.Param("route_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
		return
	}
	var dto models.CreateRouteExceptionDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.ExceptionCreateFailed, utils.ExtractValidationError(c, err)))
		return
	}

	exception, err := ctrl.trackingService.CreateRouteException(c.Request.Context(), tenantID, routeID, &dto, ctrl.actingUserID(c))
	if err != nil {
		if exceptionErrorResponse(c, err) {
			return
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.JSON(http.StatusCreated, exception)
}

// DeleteRouteException godoc
// @Summary Take an exception back
// @Tags Tracking - Exceptions
// @Produce json
// @Security BearerAuth
// @Param route_id path string true "Route ID (UUID)"
// @Param exception_id path string true "Exception ID (UUID)"
// @Success 204 "No Content"
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/routes/{route_id}/exceptions/{exception_id} [delete]
func (ctrl *TrackingController) DeleteRouteException(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}
	exceptionID, err := uuid.Parse(c.Param("exception_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
		return
	}

	if err := ctrl.trackingService.DeleteRouteException(c.Request.Context(), tenantID, exceptionID); err != nil {
		if exceptionErrorResponse(c, err) {
			return
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.Status(http.StatusNoContent)
}

// PreviewRoute godoc
// @Summary The plan a route runs over a window
// @Description Schedules, calendar, stop-list version and exceptions resolved into one row per departure; writes nothing, and a window wider than 92 days is refused
// @Tags Tracking - Schedules
// @Produce json
// @Security BearerAuth
// @Param route_id path string true "Route ID (UUID)"
// @Param from query string true "First date (YYYY-MM-DD)"
// @Param to query string true "Last date (YYYY-MM-DD)"
// @Success 200 {array} models.RoutePreviewDay
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/routes/{route_id}/preview [get]
func (ctrl *TrackingController) PreviewRoute(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}
	routeID, err := uuid.Parse(c.Param("route_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
		return
	}
	var query models.RoutePreviewQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.RoutePreviewFailed, utils.ExtractValidationError(c, err)))
		return
	}

	days, err := ctrl.trackingService.PreviewRoute(c.Request.Context(), tenantID, routeID, query.From, query.To)
	if err != nil {
		if exceptionErrorResponse(c, err) {
			return
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.JSON(http.StatusOK, days)
}
