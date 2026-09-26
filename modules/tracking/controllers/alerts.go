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

// The alert handlers of TRACK-004 — list and resolve — and a trip's event log: one slice, one file.

// alertStatus is the status each alert code answers; a code not listed is a 500.
var alertStatus = map[string]int{
	trackingErrors.AlertNotFound:        http.StatusNotFound,
	trackingErrors.AlertAlreadyResolved: http.StatusConflict,
}

func alertErrorResponse(c *gin.Context, err error) {
	var trackingErr *trackingErrors.TrackingError
	if errors.As(err, &trackingErr) {
		if status, ok := alertStatus[trackingErr.Code]; ok {
			c.JSON(status, coreErrors.BuildErrorSingle(c, trackingErr.Code))
			return
		}
	}
	c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
}

// ListRouteAlerts godoc
// @Summary List alerts
// @Description Newest first. status defaults to active; all reads every status. Every row carries the route name, the plate and who raised and resolved it; trip_id and lat/lng are set on a driver's incident
// @Tags Tracking - Alerts
// @Produce json
// @Security BearerAuth
// @Param route_id query string false "Route ID (UUID)"
// @Param status query string false "active (default), resolved or all"
// @Param severity query string false "low, medium, high or critical"
// @Param page query int false "Page number (default 1)"
// @Param page_size query int false "Page size (default 20, max 100)"
// @Success 200 {object} models.ListAlertsResponse
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/alerts [get]
func (ctrl *TrackingController) ListRouteAlerts(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}
	var query models.ListAlertsQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.AlertListFailed, utils.ExtractValidationError(c, err)))
		return
	}

	result, err := ctrl.trackingService.ListRouteAlerts(c.Request.Context(), tenantID, query)
	if err != nil {
		alertErrorResponse(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// ResolveRouteAlert godoc
// @Summary Resolve an alert
// @Description An active alert only; the signed-in user is recorded as resolved_by. Answers the row as the list does
// @Tags Tracking - Alerts
// @Produce json
// @Security BearerAuth
// @Param alert_id path string true "Alert ID (UUID)"
// @Success 200 {object} models.RouteAlert
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 409 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/alerts/{alert_id}/resolve [patch]
func (ctrl *TrackingController) ResolveRouteAlert(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}
	alertID, ok := pathUUID(c, "alert_id")
	if !ok {
		return
	}

	result, err := ctrl.trackingService.ResolveRouteAlert(c.Request.Context(), tenantID, alertID, ctrl.actingUserID(c))
	if err != nil {
		alertErrorResponse(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// ListTripEvents godoc
// @Summary A trip's event log
// @Description Oldest first: every transition, override and incident, with what it carried and who made it
// @Tags Tracking - Trips
// @Produce json
// @Security BearerAuth
// @Param trip_id path string true "Trip ID (UUID)"
// @Success 200 {array} models.TripEvent
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/trips/{trip_id}/events [get]
func (ctrl *TrackingController) ListTripEvents(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}
	tripID, ok := pathUUID(c, "trip_id")
	if !ok {
		return
	}

	result, err := ctrl.trackingService.ListTripEvents(c.Request.Context(), tenantID, tripID)
	if err != nil {
		tripErrorResponse(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}
