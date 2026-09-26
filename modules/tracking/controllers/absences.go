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

// The absence and suggestion handlers of TRACK-022: one slice, one file.

// absenceStatus is the status each absence code answers; a code not listed is a 500.
var absenceStatus = map[string]int{
	trackingErrors.RiderNotFound:       http.StatusNotFound,
	trackingErrors.AbsenceNotFound:     http.StatusNotFound,
	trackingErrors.AbsenceOverlap:      http.StatusConflict,
	trackingErrors.AbsenceCutoff:       http.StatusConflict,
	trackingErrors.AbsenceInvalidRange: http.StatusBadRequest,
	trackingErrors.RiderNoHomeLocation: http.StatusConflict,
}

func absenceErrorResponse(c *gin.Context, err error) {
	if scopeRefused(c, err) {
		return
	}
	var trackingErr *trackingErrors.TrackingError
	if errors.As(err, &trackingErr) {
		if status, ok := absenceStatus[trackingErr.Code]; ok {
			c.JSON(status, coreErrors.BuildErrorSingle(c, trackingErr.Code))
			return
		}
	}
	c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
}

// ListRiderAbsences godoc
// @Summary List a rider's absences
// @Description Oldest first, those overlapping [from, to]. A guardian reads only their own riders.
// @Tags Tracking - Riders
// @Produce json
// @Security BearerAuth
// @Param rider_id path string true "Rider ID (UUID)"
// @Param from query string false "From (YYYY-MM-DD)"
// @Param to query string false "To (YYYY-MM-DD)"
// @Success 200 {array} models.RiderAbsence
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Router /tracking/riders/{rider_id}/absences [get]
func (ctrl *TrackingController) ListRiderAbsences(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}
	riderID, ok := pathUUID(c, "rider_id")
	if !ok {
		return
	}
	var query models.ListRiderAbsencesQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.AbsenceListFailed, utils.ExtractValidationError(c, err)))
		return
	}

	result, err := ctrl.trackingService.ListRiderAbsences(c.Request.Context(), tenantID, riderID, query, ctrl.scopeUserID(c), ctrl.guardianUserID(c))
	if err != nil {
		absenceErrorResponse(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// CreateRiderAbsence godoc
// @Summary Report a rider's absence
// @Description Cancels the rider's pending tasks on the range's trips that have not ended. A guardian may report only before the organization's cut-off.
// @Tags Tracking - Riders
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param rider_id path string true "Rider ID (UUID)"
// @Param absence body models.CreateRiderAbsenceDto true "Absence"
// @Success 201 {object} models.CreateRiderAbsenceResponse
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 409 {object} coreErrors.ErrorResponse
// @Router /tracking/riders/{rider_id}/absences [post]
func (ctrl *TrackingController) CreateRiderAbsence(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}
	riderID, ok := pathUUID(c, "rider_id")
	if !ok {
		return
	}
	var dto models.CreateRiderAbsenceDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.AbsenceCreateFailed, utils.ExtractValidationError(c, err)))
		return
	}

	result, err := ctrl.trackingService.CreateRiderAbsence(c.Request.Context(), tenantID, riderID, &dto,
		ctrl.actingUserID(c), ctrl.scopeUserID(c), ctrl.guardianUserID(c))
	if err != nil {
		absenceErrorResponse(c, err)
		return
	}
	c.JSON(http.StatusCreated, result)
}

// DeleteRiderAbsence godoc
// @Summary Delete a rider's absence
// @Description The rider is seated again on the planned trips the materialiser still owns. A guardian may delete only before the organization's cut-off.
// @Tags Tracking - Riders
// @Security BearerAuth
// @Param rider_id path string true "Rider ID (UUID)"
// @Param absence_id path string true "Absence ID (UUID)"
// @Success 204
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 409 {object} coreErrors.ErrorResponse
// @Router /tracking/riders/{rider_id}/absences/{absence_id} [delete]
func (ctrl *TrackingController) DeleteRiderAbsence(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}
	riderID, ok := pathUUID(c, "rider_id")
	if !ok {
		return
	}
	absenceID, ok := pathUUID(c, "absence_id")
	if !ok {
		return
	}

	if err := ctrl.trackingService.DeleteRiderAbsence(c.Request.Context(), tenantID, riderID, absenceID,
		ctrl.scopeUserID(c), ctrl.guardianUserID(c)); err != nil {
		absenceErrorResponse(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// SuggestRiderStops godoc
// @Summary Suggest stops near a rider's home
// @Description Stops within max_walk_m (straight line) of the rider's home on the current version of an active route of the direction, nearest first then most free seats.
// @Tags Tracking - Riders
// @Produce json
// @Security BearerAuth
// @Param rider_id path string true "Rider ID (UUID)"
// @Param direction query string true "outbound or inbound"
// @Param days query int false "Weekday bitmask, bit 0 Monday (default 127)"
// @Param max_walk_m query int false "Walk in metres (default 800, max 3000)"
// @Param limit query int false "Rows (default 10, max 50)"
// @Success 200 {object} models.SuggestRiderStopsResponse
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 409 {object} coreErrors.ErrorResponse
// @Router /tracking/riders/{rider_id}/suggestions [get]
func (ctrl *TrackingController) SuggestRiderStops(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}
	riderID, ok := pathUUID(c, "rider_id")
	if !ok {
		return
	}
	var query models.SuggestRiderStopsQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.RiderSuggestionsFailed, utils.ExtractValidationError(c, err)))
		return
	}

	result, err := ctrl.trackingService.SuggestRiderStops(c.Request.Context(), tenantID, riderID, query)
	if err != nil {
		absenceErrorResponse(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}
