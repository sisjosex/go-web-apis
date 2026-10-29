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

// The alert handlers of TRACK-004 — list and resolve — and a trip's event log: one slice, one file.

// alertStatus is the status each alert code answers; a code not listed is a 500.
var alertStatus = map[string]int{
	trackingErrors.AlertNotFound:        http.StatusNotFound,
	trackingErrors.AlertAlreadyResolved: http.StatusConflict,
	trackingErrors.AlertSettingsInvalid: http.StatusBadRequest,
}

func alertErrorResponse(c *gin.Context, err error) error {
	var trackingErr *trackingErrors.TrackingError
	if errors.As(err, &trackingErr) {
		if status, ok := alertStatus[trackingErr.Code]; ok {
			return api.Fail(status, coreErrors.BuildErrorSingle(c, trackingErr.Code))
		}
	}
	return api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
}

// ListRouteAlertsInput is ListRouteAlerts's request.
type ListRouteAlertsInput struct {
	Query api.Query[models.ListAlertsQuery]
}

// ListRouteAlerts is GET /alerts: List alerts.
func (ctrl *TrackingController) ListRouteAlerts(ctx context.Context, in *ListRouteAlertsInput) (*api.Out[*models.ListAlertsResponse], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}
	var query models.ListAlertsQuery
	if err := in.Query.Into(ctx, &query); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.AlertListFailed, utils.ExtractValidationError(c, err)))
	}

	result, err := ctrl.trackingService.ListRouteAlerts(c.Request.Context(), tenantID, query)
	if err != nil {
		return nil, alertErrorResponse(c, err)
	}
	return api.OK(result)
}

// ResolveRouteAlertInput is ResolveRouteAlert's request.
type ResolveRouteAlertInput struct {
	AlertID string `path:"alert_id"`
}

// ResolveRouteAlert is PATCH /alerts/{alert_id}/resolve: Resolve an alert.
func (ctrl *TrackingController) ResolveRouteAlert(ctx context.Context, _ *ResolveRouteAlertInput) (*api.Out[*models.RouteAlert], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}
	alertID, err := pathUUID(c, "alert_id")
	if err != nil {
		return nil, err
	}

	result, err := ctrl.trackingService.ResolveRouteAlert(c.Request.Context(), tenantID, alertID, ctrl.actingUserID(c))
	if err != nil {
		return nil, alertErrorResponse(c, err)
	}
	return api.OK(result)
}

// AlertSettings is GET /alert-settings: The alert thresholds.
func (ctrl *TrackingController) AlertSettings(ctx context.Context, _ *api.None) (*api.Out[*models.AlertSettings], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}
	settings, err := ctrl.trackingService.AlertSettings(c.Request.Context(), tenantID)
	if err != nil {
		return nil, alertErrorResponse(c, err)
	}
	return api.OK(settings)
}

// SaveAlertSettingsInput is SaveAlertSettings's request.
type SaveAlertSettingsInput struct {
	JSON api.Body[models.AlertSettings]
}

// SaveAlertSettings is PUT /alert-settings: Save the alert thresholds.
func (ctrl *TrackingController) SaveAlertSettings(ctx context.Context, in *SaveAlertSettingsInput) (*api.Out[*models.AlertSettings], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}
	var body models.AlertSettings
	if err := in.JSON.Into(ctx, &body); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.AlertSettingsInvalid, utils.ExtractValidationError(c, err)))
	}
	settings, err := ctrl.trackingService.SaveAlertSettings(c.Request.Context(), tenantID, body)
	if err != nil {
		return nil, alertErrorResponse(c, err)
	}
	return api.OK(settings)
}

// ListTripEventsInput is ListTripEvents's request.
type ListTripEventsInput struct {
	TripID string `path:"trip_id"`
}

// ListTripEvents is GET /trips/{trip_id}/events: A trip's event log.
func (ctrl *TrackingController) ListTripEvents(ctx context.Context, _ *ListTripEventsInput) (*api.Out[[]*models.TripEvent], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}
	tripID, err := pathUUID(c, "trip_id")
	if err != nil {
		return nil, err
	}

	result, err := ctrl.trackingService.ListTripEvents(c.Request.Context(), tenantID, tripID)
	if err != nil {
		return nil, tripErrorResponse(c, err)
	}
	return api.OK(result)
}
