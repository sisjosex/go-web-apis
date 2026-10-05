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

// The rider places handlers of TRACK-044 D4: one slice, one file.

// riderPlaceStatus is the status each rider place code answers; a code not listed is a 500.
var riderPlaceStatus = map[string]int{
	trackingErrors.RiderNotFound:      http.StatusNotFound,
	trackingErrors.RiderPlaceNotFound: http.StatusNotFound,
	trackingErrors.RiderPlaceDefault:  http.StatusConflict,
}

func riderPlaceErrorResponse(c *gin.Context, err error) {
	if scopeRefused(c, err) {
		return
	}
	var trackingErr *trackingErrors.TrackingError
	if errors.As(err, &trackingErr) {
		if status, ok := riderPlaceStatus[trackingErr.Code]; ok {
			c.JSON(status, coreErrors.BuildErrorSingle(c, trackingErr.Code))
			return
		}
	}
	c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
}

// ListRiderPlaces godoc
// @Summary List a rider's saved places
// @Description The default first: it is the rider's home.
// @Tags Tracking - Riders
// @Produce json
// @Security BearerAuth
// @Param rider_id path string true "Rider ID (UUID)"
// @Success 200 {object} models.ListRiderPlacesResponse
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Router /tracking/riders/{rider_id}/places [get]
func (ctrl *TrackingController) ListRiderPlaces(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}
	riderID, ok := pathUUID(c, "rider_id")
	if !ok {
		return
	}

	result, err := ctrl.trackingService.ListRiderPlaces(c.Request.Context(), tenantID, riderID, ctrl.scopeUserID(c))
	if err != nil {
		riderPlaceErrorResponse(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// CreateRiderPlace godoc
// @Summary Save a place for a rider
// @Description The rider's first place, or one sent with is_default, becomes the rider's home.
// @Tags Tracking - Riders
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param rider_id path string true "Rider ID (UUID)"
// @Param place body models.CreateRiderPlaceDto true "Place"
// @Success 201 {object} models.RiderPlace
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Router /tracking/riders/{rider_id}/places [post]
func (ctrl *TrackingController) CreateRiderPlace(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}
	riderID, ok := pathUUID(c, "rider_id")
	if !ok {
		return
	}
	var dto models.CreateRiderPlaceDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.RiderPlaceCreateFailed, utils.ExtractValidationError(c, err)))
		return
	}

	place, err := ctrl.trackingService.CreateRiderPlace(c.Request.Context(), tenantID, riderID, &dto, ctrl.scopeUserID(c))
	if err != nil {
		riderPlaceErrorResponse(c, err)
		return
	}
	c.JSON(http.StatusCreated, place)
}

// UpdateRiderPlace godoc
// @Summary Edit a rider's place
// @Description A field left out keeps its value; is_default true makes the place the rider's home.
// @Tags Tracking - Riders
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param rider_id path string true "Rider ID (UUID)"
// @Param place_id path string true "Place ID (UUID)"
// @Param place body models.UpdateRiderPlaceDto true "The fields to change"
// @Success 200 {object} models.RiderPlace
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Router /tracking/riders/{rider_id}/places/{place_id} [patch]
func (ctrl *TrackingController) UpdateRiderPlace(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}
	riderID, ok := pathUUID(c, "rider_id")
	if !ok {
		return
	}
	placeID, ok := pathUUID(c, "place_id")
	if !ok {
		return
	}
	var dto models.UpdateRiderPlaceDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.RiderPlaceUpdateFailed, utils.ExtractValidationError(c, err)))
		return
	}

	place, err := ctrl.trackingService.UpdateRiderPlace(c.Request.Context(), tenantID, riderID, placeID, &dto, ctrl.scopeUserID(c))
	if err != nil {
		riderPlaceErrorResponse(c, err)
		return
	}
	c.JSON(http.StatusOK, place)
}

// DeleteRiderPlace godoc
// @Summary Delete a rider's place
// @Description The default place goes only when it is the last one, and takes the home pin with it.
// @Tags Tracking - Riders
// @Security BearerAuth
// @Param rider_id path string true "Rider ID (UUID)"
// @Param place_id path string true "Place ID (UUID)"
// @Success 204
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 409 {object} coreErrors.ErrorResponse
// @Router /tracking/riders/{rider_id}/places/{place_id} [delete]
func (ctrl *TrackingController) DeleteRiderPlace(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}
	riderID, ok := pathUUID(c, "rider_id")
	if !ok {
		return
	}
	placeID, ok := pathUUID(c, "place_id")
	if !ok {
		return
	}

	if err := ctrl.trackingService.DeleteRiderPlace(c.Request.Context(), tenantID, riderID, placeID, ctrl.scopeUserID(c)); err != nil {
		riderPlaceErrorResponse(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
