package controllers

import (
	"errors"
	"net/http"

	coreErrors "josex/web/modules/core/errors"
	trackingErrors "josex/web/modules/tracking/errors"
	"josex/web/modules/tracking/models"

	"github.com/gin-gonic/gin"
)

// The rider card of TRACK-027, as the web reads it: the code a driver scans.

func riderQRErrorResponse(c *gin.Context, err error) {
	if scopeRefused(c, err) {
		return
	}
	var trackingErr *trackingErrors.TrackingError
	if errors.As(err, &trackingErr) && trackingErr.Code == trackingErrors.RiderNotFound {
		c.JSON(http.StatusNotFound, coreErrors.BuildErrorSingle(c, trackingErr.Code))
		return
	}
	c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
}

// RiderQRToken godoc
// @Summary Read or rotate a rider's card code
// @Description The code printed on the rider's card, which a driver scans. POST replaces it, so the old card resolves nothing (riders:write). An organization user reaches only their organizations' riders
// @Tags Tracking - Riders
// @Produce json
// @Security BearerAuth
// @Param rider_id path string true "Rider ID (UUID)"
// @Success 200 {object} models.RiderQRToken
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Router /tracking/riders/{rider_id}/qr-token [get]
// @Router /tracking/riders/{rider_id}/qr-token [post]
func (ctrl *TrackingController) RiderQRToken(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}
	riderID, ok := pathUUID(c, "rider_id")
	if !ok {
		return
	}

	rotate := c.Request.Method == http.MethodPost
	token, err := ctrl.trackingService.RiderQRToken(c.Request.Context(), tenantID, riderID, rotate, ctrl.scopeUserID(c))
	if err != nil {
		riderQRErrorResponse(c, err)
		return
	}
	c.JSON(http.StatusOK, models.RiderQRToken{QRToken: token})
}
