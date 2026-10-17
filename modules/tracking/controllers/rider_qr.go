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

// ListRiderCards godoc
// @Summary Printable rider cards
// @Description The cards of one route's riders (its pair's too, in boarding order), of one client, or of one rider: name, client, the outbound route and pickup stop, and the code a driver scans. Exactly one filter. An organization user reaches only their organizations' riders
// @Tags Tracking - Riders
// @Produce json
// @Security BearerAuth
// @Param route_id query string false "Route ID (UUID)"
// @Param organization_id query string false "Organization ID (UUID)"
// @Param rider_id query string false "Rider ID (UUID)"
// @Success 200 {object} models.RiderCards
// @Failure 400 {object} coreErrors.ErrorResponse
// @Router /tracking/riders/cards [get]
func (ctrl *TrackingController) ListRiderCards(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}
	var query models.RiderCardsQuery
	if err := c.ShouldBindQuery(&query); err != nil || countSet(query.RouteID, query.OrganizationID, query.RiderID) != 1 {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, trackingErrors.RiderCardsInvalid))
		return
	}

	cards, err := ctrl.trackingService.ListRiderCards(c.Request.Context(), tenantID, query, ctrl.scopeUserID(c))
	if err != nil {
		if scopeRefused(c, err) {
			return
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.JSON(http.StatusOK, models.RiderCards{Cards: cards})
}

// countSet is how many of the optional filters were sent.
func countSet(values ...*string) int {
	n := 0
	for _, v := range values {
		if v != nil && *v != "" {
			n++
		}
	}
	return n
}
