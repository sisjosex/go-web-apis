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

// The rider places handlers of TRACK-044 D4: one slice, one file.

// riderPlaceStatus is the status each rider place code answers; a code not listed is a 500.
var riderPlaceStatus = map[string]int{
	trackingErrors.RiderNotFound:      http.StatusNotFound,
	trackingErrors.RiderPlaceNotFound: http.StatusNotFound,
	trackingErrors.RiderPlaceDefault:  http.StatusConflict,
}

func riderPlaceErrorResponse(c *gin.Context, err error) error {
	if err := scopeRefused(c, err); err != nil {
		return err
	}
	var trackingErr *trackingErrors.TrackingError
	if errors.As(err, &trackingErr) {
		if status, ok := riderPlaceStatus[trackingErr.Code]; ok {
			return api.Fail(status, coreErrors.BuildErrorSingle(c, trackingErr.Code))
		}
	}
	return api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
}

// ListRiderPlacesInput is ListRiderPlaces's request.
type ListRiderPlacesInput struct {
	RiderID string `path:"rider_id"`
}

// ListRiderPlaces is GET /riders/{rider_id}/places: List a rider's saved places.
func (ctrl *TrackingController) ListRiderPlaces(ctx context.Context, _ *ListRiderPlacesInput) (*api.Out[*models.ListRiderPlacesResponse], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}
	riderID, err := pathUUID(c, "rider_id")
	if err != nil {
		return nil, err
	}

	result, err := ctrl.trackingService.ListRiderPlaces(c.Request.Context(), tenantID, riderID, ctrl.scopeUserID(c))
	if err != nil {
		return nil, riderPlaceErrorResponse(c, err)
	}
	return api.OK(result)
}

// CreateRiderPlaceInput is CreateRiderPlace's request.
type CreateRiderPlaceInput struct {
	RiderID string `path:"rider_id"`
	JSON    api.Body[models.CreateRiderPlaceDto]
}

// CreateRiderPlace is POST /riders/{rider_id}/places: Save a place for a rider.
func (ctrl *TrackingController) CreateRiderPlace(ctx context.Context, in *CreateRiderPlaceInput) (*api.Out[*models.RiderPlace], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}
	riderID, err := pathUUID(c, "rider_id")
	if err != nil {
		return nil, err
	}
	var dto models.CreateRiderPlaceDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.RiderPlaceCreateFailed, utils.ExtractValidationError(c, err)))
	}

	place, err := ctrl.trackingService.CreateRiderPlace(c.Request.Context(), tenantID, riderID, &dto, ctrl.scopeUserID(c))
	if err != nil {
		return nil, riderPlaceErrorResponse(c, err)
	}
	return api.OK(place)
}

// UpdateRiderPlaceInput is UpdateRiderPlace's request.
type UpdateRiderPlaceInput struct {
	PlaceID string `path:"place_id"`
	RiderID string `path:"rider_id"`
	JSON    api.Body[models.UpdateRiderPlaceDto]
}

// UpdateRiderPlace is PATCH /riders/{rider_id}/places/{place_id}: Edit a rider's place.
func (ctrl *TrackingController) UpdateRiderPlace(ctx context.Context, in *UpdateRiderPlaceInput) (*api.Out[*models.RiderPlace], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}
	riderID, err := pathUUID(c, "rider_id")
	if err != nil {
		return nil, err
	}
	placeID, err := pathUUID(c, "place_id")
	if err != nil {
		return nil, err
	}
	var dto models.UpdateRiderPlaceDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.RiderPlaceUpdateFailed, utils.ExtractValidationError(c, err)))
	}

	place, err := ctrl.trackingService.UpdateRiderPlace(c.Request.Context(), tenantID, riderID, placeID, &dto, ctrl.scopeUserID(c))
	if err != nil {
		return nil, riderPlaceErrorResponse(c, err)
	}
	return api.OK(place)
}

// DeleteRiderPlaceInput is DeleteRiderPlace's request.
type DeleteRiderPlaceInput struct {
	PlaceID string `path:"place_id"`
	RiderID string `path:"rider_id"`
}

// DeleteRiderPlace is DELETE /riders/{rider_id}/places/{place_id}: Delete a rider's place.
func (ctrl *TrackingController) DeleteRiderPlace(ctx context.Context, _ *DeleteRiderPlaceInput) (*api.Empty, error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}
	riderID, err := pathUUID(c, "rider_id")
	if err != nil {
		return nil, err
	}
	placeID, err := pathUUID(c, "place_id")
	if err != nil {
		return nil, err
	}

	if err := ctrl.trackingService.DeleteRiderPlace(c.Request.Context(), tenantID, riderID, placeID, ctrl.scopeUserID(c)); err != nil {
		return nil, riderPlaceErrorResponse(c, err)
	}
	return api.Done()
}
