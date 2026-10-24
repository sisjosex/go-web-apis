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
	"github.com/google/uuid"
)

// The exception and preview handlers of TRACK-018.

// exceptionErrorResponse maps the exception and preview codes to statuses and reports whether it has
// already written the response.
func exceptionErrorResponse(c *gin.Context, err error) error {
	var trackingErr *trackingErrors.TrackingError
	if !errors.As(err, &trackingErr) {
		return nil
	}
	switch trackingErr.Code {
	case trackingErrors.RouteNotFound, trackingErrors.ExceptionNotFound,
		trackingErrors.VehicleNotFound, trackingErrors.DriverNotFound, trackingErrors.StopPlaceNotFound:
		return api.Fail(http.StatusNotFound, coreErrors.BuildErrorSingle(c, trackingErr.Code))
	case trackingErrors.ExceptionPayload, trackingErrors.ExceptionRange, trackingErrors.RoutePreviewRange:
		return api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, trackingErr.Code))
	case trackingErrors.ServiceBlocked:
		return api.Fail(http.StatusConflict, coreErrors.BuildErrorDetail(c, trackingErr.Code, trackingErr.Detail))
	}
	return nil
}

// ListRouteExceptionsInput is ListRouteExceptions's request.
type ListRouteExceptionsInput struct {
	RouteID string `path:"route_id"`
	Query   api.Query[models.ListRouteExceptionsQuery]
}

// ListRouteExceptions is GET /routes/{route_id}/exceptions: List a route's exceptions.
func (ctrl *TrackingController) ListRouteExceptions(ctx context.Context, in *ListRouteExceptionsInput) (*api.Out[[]*models.RouteException], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}
	routeID, err := uuid.Parse(in.RouteID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
	}
	var query models.ListRouteExceptionsQuery
	if err := in.Query.Into(ctx, &query); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.ExceptionListFailed, utils.ExtractValidationError(c, err)))
	}

	exceptions, err := ctrl.trackingService.ListRouteExceptions(c.Request.Context(), tenantID, routeID, query.DateFrom, query.DateTo)
	if err != nil {
		if err := exceptionErrorResponse(c, err); err != nil {
			return nil, err
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.OK(exceptions)
}

// CreateRouteExceptionInput is CreateRouteException's request.
type CreateRouteExceptionInput struct {
	RouteID string `path:"route_id"`
	JSON    api.Body[models.CreateRouteExceptionDto]
}

// CreateRouteException is POST /routes/{route_id}/exceptions: Record what happens differently over a range of dates.
func (ctrl *TrackingController) CreateRouteException(ctx context.Context, in *CreateRouteExceptionInput) (*api.Out[*models.RouteException], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}
	routeID, err := uuid.Parse(in.RouteID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
	}
	var dto models.CreateRouteExceptionDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.ExceptionCreateFailed, utils.ExtractValidationError(c, err)))
	}

	exception, err := ctrl.trackingService.CreateRouteException(c.Request.Context(), tenantID, routeID, &dto, ctrl.actingUserID(c))
	if err != nil {
		if err := exceptionErrorResponse(c, err); err != nil {
			return nil, err
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.OK(exception)
}

// DeleteRouteExceptionInput is DeleteRouteException's request.
type DeleteRouteExceptionInput struct {
	ExceptionID string `path:"exception_id"`
	RouteID     string `path:"route_id"`
}

// DeleteRouteException is DELETE /routes/{route_id}/exceptions/{exception_id}: Take an exception back.
func (ctrl *TrackingController) DeleteRouteException(ctx context.Context, in *DeleteRouteExceptionInput) (*api.Empty, error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}
	exceptionID, err := uuid.Parse(in.ExceptionID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
	}

	if err := ctrl.trackingService.DeleteRouteException(c.Request.Context(), tenantID, exceptionID); err != nil {
		if err := exceptionErrorResponse(c, err); err != nil {
			return nil, err
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.Done()
}

// PreviewRouteInput is PreviewRoute's request.
type PreviewRouteInput struct {
	RouteID string `path:"route_id"`
	Query   api.Query[models.RoutePreviewQuery]
}

// PreviewRoute is GET /routes/{route_id}/preview: The plan a route runs over a window.
func (ctrl *TrackingController) PreviewRoute(ctx context.Context, in *PreviewRouteInput) (*api.Out[[]*models.RoutePreviewDay], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}
	routeID, err := uuid.Parse(in.RouteID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
	}
	var query models.RoutePreviewQuery
	if err := in.Query.Into(ctx, &query); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.RoutePreviewFailed, utils.ExtractValidationError(c, err)))
	}

	days, err := ctrl.trackingService.PreviewRoute(c.Request.Context(), tenantID, routeID, query.From, query.To)
	if err != nil {
		if err := exceptionErrorResponse(c, err); err != nil {
			return nil, err
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.OK(days)
}
