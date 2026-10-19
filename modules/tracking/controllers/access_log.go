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

// ListRiderAccessLog godoc
// @Summary Who outside the operator read a rider
// @Description Guardians' and organization users' reads of the rider's status, home, live map and absences, one entry per reader, action and hour, newest first (TRACK-063). Staff only.
// @Tags Tracking - Riders
// @Produce json
// @Security BearerAuth
// @Param rider_id path string true "Rider ID (UUID)"
// @Param from query string false "From (YYYY-MM-DD)"
// @Param to query string false "To (YYYY-MM-DD)"
// @Param page query int false "Page" default(1)
// @Param page_size query int false "Page size" default(20)
// @Success 200 {object} models.ListRiderAccessLogResponse
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Router /tracking/riders/{rider_id}/audit [get]
func (ctrl *TrackingController) ListRiderAccessLog(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}
	riderID, ok := pathUUID(c, "rider_id")
	if !ok {
		return
	}
	var query models.ListRiderAccessLogQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.RiderAccessLogFailed, utils.ExtractValidationError(c, err)))
		return
	}

	result, err := ctrl.trackingService.ListRiderAccessLog(c.Request.Context(), tenantID, riderID, query)
	if err != nil {
		var trackingErr *trackingErrors.TrackingError
		if errors.As(err, &trackingErr) && trackingErr.Code == trackingErrors.RiderNotFound {
			c.JSON(http.StatusNotFound, coreErrors.BuildErrorSingle(c, trackingErr.Code))
			return
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.JSON(http.StatusOK, result)
}
