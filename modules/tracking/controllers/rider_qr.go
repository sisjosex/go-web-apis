package controllers

import (
	"context"
	"errors"
	"net/http"

	"josex/web/modules/core/api"
	coreErrors "josex/web/modules/core/errors"
	trackingErrors "josex/web/modules/tracking/errors"
	"josex/web/modules/tracking/models"

	"github.com/gin-gonic/gin"
)

// The rider card of TRACK-027, as the web reads it: the code a driver scans.

func riderQRErrorResponse(c *gin.Context, err error) error {
	if err := scopeRefused(c, err); err != nil {
		return err
	}
	var trackingErr *trackingErrors.TrackingError
	if errors.As(err, &trackingErr) && trackingErr.Code == trackingErrors.RiderNotFound {
		return api.Fail(http.StatusNotFound, coreErrors.BuildErrorSingle(c, trackingErr.Code))
	}
	return api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
}

// RiderQRTokenInput is RiderQRToken's request.
type RiderQRTokenInput struct {
	RiderID string `path:"rider_id"`
}

// RiderQRToken is GET /riders/{rider_id}/qr-token: Read or rotate a rider's card code.
func (ctrl *TrackingController) RiderQRToken(ctx context.Context, _ *RiderQRTokenInput) (*api.Out[models.RiderQRToken], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}
	riderID, err := pathUUID(c, "rider_id")
	if err != nil {
		return nil, err
	}

	rotate := c.Request.Method == http.MethodPost
	token, err := ctrl.trackingService.RiderQRToken(c.Request.Context(), tenantID, riderID, rotate, ctrl.scopeUserID(c))
	if err != nil {
		return nil, riderQRErrorResponse(c, err)
	}
	return api.OK(models.RiderQRToken{QRToken: token})
}

// ListRiderCardsInput is ListRiderCards's request.
type ListRiderCardsInput struct {
	Query api.Query[models.RiderCardsQuery]
}

// ListRiderCards is GET /riders/cards: Printable rider cards.
func (ctrl *TrackingController) ListRiderCards(ctx context.Context, in *ListRiderCardsInput) (*api.Out[models.RiderCards], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}
	var query models.RiderCardsQuery
	if err := in.Query.Into(ctx, &query); err != nil || countSet(query.RouteID, query.OrganizationID, query.RiderID) != 1 {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, trackingErrors.RiderCardsInvalid))
	}

	cards, err := ctrl.trackingService.ListRiderCards(c.Request.Context(), tenantID, query, ctrl.scopeUserID(c))
	if err != nil {
		if err := scopeRefused(c, err); err != nil {
			return nil, err
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.OK(models.RiderCards{Cards: cards})
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
