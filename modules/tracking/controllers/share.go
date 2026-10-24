package controllers

import (
	"context"
	"errors"
	"net/http"

	"josex/web/config"
	"josex/web/modules/core/api"
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

func shareErrorResponse(c *gin.Context, err error) error {
	var trackingErr *trackingErrors.TrackingError
	if errors.As(err, &trackingErr) {
		if status, ok := shareStatus[trackingErr.Code]; ok {
			return api.Fail(status, coreErrors.BuildErrorSingle(c, trackingErr.Code))
		}
	}
	return api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
}

// ShareURL is the public page that opens a token (D1).
func ShareURL(token string) string {
	return config.ModularAppConfig.Core.FrontendURL + "/s/" + token
}

// CreateTripShareLinkInput is CreateTripShareLink's request; the body is optional.
type CreateTripShareLinkInput struct {
	TripID string `path:"trip_id"`
	JSON   api.Body[models.CreateShareLinkDto]
}

// CreateTripShareLink is a public link to a planned or running trip: route, plate, stops and the
// bus, never a name. ttl_minutes defaults to the trip's planned end + 60 minutes, at most 1440.
func (ctrl *TrackingController) CreateTripShareLink(ctx context.Context, _ *CreateTripShareLinkInput) (*api.Out[models.CreateShareLinkResponse], error) {
	tripID, err := pathUUID(api.Gin(ctx), "trip_id")
	if err != nil {
		return nil, err
	}
	return ctrl.createShareLink(ctx, &tripID, nil, trackingErrors.ShareCreateFailed)
}

// CreateRiderShareLinkInput is CreateRiderShareLink's request; the body is optional.
type CreateRiderShareLinkInput struct {
	RiderID string `path:"rider_id"`
	JSON    api.Body[models.CreateShareLinkDto]
}

// CreateRiderShareLink is a guardian's public link to their rider's trip of today still to run: that
// rider's stops and first name only. 409 when there is no such trip.
func (ctrl *TrackingController) CreateRiderShareLink(ctx context.Context, _ *CreateRiderShareLinkInput) (*api.Out[models.CreateShareLinkResponse], error) {
	riderID, err := pathUUID(api.Gin(ctx), "rider_id")
	if err != nil {
		return nil, err
	}
	return ctrl.createShareLink(ctx, nil, &riderID, trackingErrors.ShareCreateFailed)
}

func (ctrl *TrackingController) createShareLink(ctx context.Context, tripID, riderID *uuid.UUID, failedCode string) (*api.Out[models.CreateShareLinkResponse], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}
	slug := c.GetString("tenant_slug")
	if slug == "" {
		return nil, api.Fail(http.StatusForbidden, coreErrors.BuildErrorSingle(c, trackingErrors.TenantRequired))
	}
	var dto models.CreateShareLinkDto
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&dto); err != nil {
			return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, failedCode, utils.ExtractValidationError(c, err)))
		}
	}

	link, err := ctrl.trackingService.CreateShareLink(c.Request.Context(), tenantID, slug, tripID, riderID,
		ctrl.actingUserID(c), ctrl.guardianUserID(c) != nil, dto.TTLMinutes)
	if err != nil {
		return nil, shareErrorResponse(c, err)
	}
	return api.OK(models.CreateShareLinkResponse{ID: link.ID, URL: ShareURL(link.Token), ExpiresAt: link.ExpiresAt})
}

// ListTripShareLinksInput is ListTripShareLinks's request.
type ListTripShareLinksInput struct {
	TripID string `path:"trip_id"`
}

// ListTripShareLinks is GET /trips/{trip_id}/share-links: List a trip's open share links.
func (ctrl *TrackingController) ListTripShareLinks(ctx context.Context, _ *ListTripShareLinksInput) (*api.Out[models.ListShareLinksResponse], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}
	tripID, err := pathUUID(c, "trip_id")
	if err != nil {
		return nil, err
	}
	links, err := ctrl.trackingService.ListShareLinks(c.Request.Context(), tenantID, tripID)
	if err != nil {
		return nil, shareErrorResponse(c, err)
	}
	return api.OK(models.ListShareLinksResponse{ShareLinks: links})
}

// RevokeShareLinkInput is RevokeShareLink's request.
type RevokeShareLinkInput struct {
	LinkID string `path:"link_id"`
}

// RevokeShareLink is DELETE /share-links/{link_id}: Revoke a share link.
func (ctrl *TrackingController) RevokeShareLink(ctx context.Context, _ *RevokeShareLinkInput) (*api.Empty, error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}
	linkID, err := pathUUID(c, "link_id")
	if err != nil {
		return nil, err
	}
	guardian := ctrl.guardianUserID(c)
	if err := ctrl.trackingService.RevokeShareLink(c.Request.Context(), tenantID, linkID, ctrl.actingUserID(c), guardian != nil); err != nil {
		return nil, shareErrorResponse(c, err)
	}
	return api.Done()
}

// GetSharedTripInput is GetSharedTrip's request.
type GetSharedTripInput struct {
	Token string `path:"token" doc:"Share token"`
}

// GetSharedTrip is a shared trip, for anyone holding the link. No session: the token's slug names the
// operator. Route, plate, status, stops with planned and actual times, the bus while it runs and the
// cached ETA; never a driver's or passenger's name (a guardian's link adds the rider's first name
// only). An unknown, expired or revoked link is one 404.
func (ctrl *LiveController) GetSharedTrip(ctx context.Context, in *GetSharedTripInput) (*api.NoStore[*models.SharedTrip], error) {
	c := api.Gin(ctx)
	tenantID, err := uuid.Parse(c.GetString("tenant_id"))
	_, hash, ok := services.ParseShareToken(in.Token)
	if err != nil || !ok {
		return nil, api.Fail(http.StatusNotFound, coreErrors.BuildErrorSingle(c, trackingErrors.ShareNotFound))
	}
	trip, err := ctrl.live.Shared(c.Request.Context(), tenantID, hash)
	if err != nil {
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	if trip == nil {
		return nil, api.Fail(http.StatusNotFound, coreErrors.BuildErrorSingle(c, trackingErrors.ShareNotFound))
	}
	return api.Uncached(trip)
}
