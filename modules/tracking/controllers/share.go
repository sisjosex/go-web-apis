package controllers

import (
	"errors"
	"net/http"

	"josex/web/config"
	coreErrors "josex/web/modules/core/errors"
	"josex/web/modules/core/utils"
	trackingErrors "josex/web/modules/tracking/errors"
	"josex/web/modules/tracking/models"
	"josex/web/modules/tracking/services"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// The share link handlers of TRACK-060: one slice, one file.

// shareStatus is the status each share code answers; a code not listed is a 500.
var shareStatus = map[string]int{
	trackingErrors.ShareNotFound: http.StatusNotFound,
	trackingErrors.TripNotFound:  http.StatusNotFound,
	trackingErrors.RiderNotFound: http.StatusNotFound,
	trackingErrors.ShareNoTrip:   http.StatusConflict,
}

func shareErrorResponse(c *gin.Context, err error) {
	var trackingErr *trackingErrors.TrackingError
	if errors.As(err, &trackingErr) {
		if status, ok := shareStatus[trackingErr.Code]; ok {
			c.JSON(status, coreErrors.BuildErrorSingle(c, trackingErr.Code))
			return
		}
	}
	c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
}

// ShareURL is the public page that opens a token (D1).
func ShareURL(token string) string {
	return config.ModularAppConfig.Core.FrontendURL + "/s/" + token
}

// CreateTripShareLink godoc
// @Summary Share a trip
// @Description A public link to a planned or running trip: route, plate, stops and the bus, never a name. ttl_minutes defaults to the trip's planned end + 60 minutes, at most 1440.
// @Tags Tracking - Share links
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param trip_id path string true "Trip ID (UUID)"
// @Param link body models.CreateShareLinkDto false "Duration"
// @Success 201 {object} models.CreateShareLinkResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 409 {object} coreErrors.ErrorResponse
// @Router /tracking/trips/{trip_id}/share-links [post]
func (ctrl *TrackingController) CreateTripShareLink(c *gin.Context) {
	tripID, ok := pathUUID(c, "trip_id")
	if !ok {
		return
	}
	ctrl.createShareLink(c, &tripID, nil, trackingErrors.ShareCreateFailed)
}

// CreateRiderShareLink godoc
// @Summary Share a rider's trip of today
// @Description A guardian's public link to their rider's trip of today still to run: that rider's stops and first name only. 409 when there is no such trip.
// @Tags Tracking - Share links
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param rider_id path string true "Rider ID (UUID)"
// @Param link body models.CreateShareLinkDto false "Duration"
// @Success 201 {object} models.CreateShareLinkResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 409 {object} coreErrors.ErrorResponse
// @Router /tracking/riders/{rider_id}/share-links [post]
func (ctrl *TrackingController) CreateRiderShareLink(c *gin.Context) {
	riderID, ok := pathUUID(c, "rider_id")
	if !ok {
		return
	}
	ctrl.createShareLink(c, nil, &riderID, trackingErrors.ShareCreateFailed)
}

func (ctrl *TrackingController) createShareLink(c *gin.Context, tripID, riderID *uuid.UUID, failedCode string) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}
	slug := c.GetString("tenant_slug")
	if slug == "" {
		c.JSON(http.StatusForbidden, coreErrors.BuildErrorSingle(c, trackingErrors.TenantRequired))
		return
	}
	var dto models.CreateShareLinkDto
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&dto); err != nil {
			c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, failedCode, utils.ExtractValidationError(c, err)))
			return
		}
	}

	link, err := ctrl.trackingService.CreateShareLink(c.Request.Context(), tenantID, slug, tripID, riderID,
		ctrl.actingUserID(c), ctrl.guardianUserID(c) != nil, dto.TTLMinutes)
	if err != nil {
		shareErrorResponse(c, err)
		return
	}
	c.JSON(http.StatusCreated, models.CreateShareLinkResponse{ID: link.ID, URL: ShareURL(link.Token), ExpiresAt: link.ExpiresAt})
}

// ListTripShareLinks godoc
// @Summary List a trip's open share links
// @Description Newest first, those neither expired nor revoked, with who made them.
// @Tags Tracking - Share links
// @Produce json
// @Security BearerAuth
// @Param trip_id path string true "Trip ID (UUID)"
// @Success 200 {object} models.ListShareLinksResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Router /tracking/trips/{trip_id}/share-links [get]
func (ctrl *TrackingController) ListTripShareLinks(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}
	tripID, ok := pathUUID(c, "trip_id")
	if !ok {
		return
	}
	links, err := ctrl.trackingService.ListShareLinks(c.Request.Context(), tenantID, tripID)
	if err != nil {
		shareErrorResponse(c, err)
		return
	}
	c.JSON(http.StatusOK, models.ListShareLinksResponse{ShareLinks: links})
}

// RevokeShareLink godoc
// @Summary Revoke a share link
// @Description The link answers 404 from now on. A guardian revokes only the links they made.
// @Tags Tracking - Share links
// @Security BearerAuth
// @Param link_id path string true "Share link ID (UUID)"
// @Success 204
// @Failure 404 {object} coreErrors.ErrorResponse
// @Router /tracking/share-links/{link_id} [delete]
func (ctrl *TrackingController) RevokeShareLink(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}
	linkID, ok := pathUUID(c, "link_id")
	if !ok {
		return
	}
	guardian := ctrl.guardianUserID(c)
	if err := ctrl.trackingService.RevokeShareLink(c.Request.Context(), tenantID, linkID, ctrl.actingUserID(c), guardian != nil); err != nil {
		shareErrorResponse(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// GetSharedTrip godoc
// @Summary A shared trip, for anyone holding the link
// @Description No session: the token's slug names the operator. Route, plate, status, stops with planned and actual times, the bus while it runs and the cached ETA; never a driver's or passenger's name (a guardian's link adds the rider's first name only). An unknown, expired or revoked link is one 404.
// @Tags Tracking - Share links
// @Produce json
// @Param token path string true "Share token"
// @Success 200 {object} models.SharedTrip
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 429 {object} coreErrors.ErrorResponse
// @Router /public/share/{token} [get]
func (ctrl *LiveController) GetSharedTrip(c *gin.Context) {
	tenantID, err := uuid.Parse(c.GetString("tenant_id"))
	_, hash, ok := services.ParseShareToken(c.Param("token"))
	if err != nil || !ok {
		c.JSON(http.StatusNotFound, coreErrors.BuildErrorSingle(c, trackingErrors.ShareNotFound))
		return
	}
	trip, err := ctrl.live.Shared(c.Request.Context(), tenantID, hash)
	if err != nil {
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	if trip == nil {
		c.JSON(http.StatusNotFound, coreErrors.BuildErrorSingle(c, trackingErrors.ShareNotFound))
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, trip)
}
