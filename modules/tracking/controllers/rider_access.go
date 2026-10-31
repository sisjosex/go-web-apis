package controllers

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"josex/web/modules/core/api"
	coreErrors "josex/web/modules/core/errors"
	tenancyModels "josex/web/modules/tenancy/models"
	trackingErrors "josex/web/modules/tracking/errors"
	"josex/web/modules/tracking/models"
)

// riderAccessError answers a code that does not open: 404 unknown, 409 already used, 410 expired or revoked.
func riderAccessError(c *gin.Context, err error) error {
	var trackingErr *trackingErrors.TrackingError
	if errors.As(err, &trackingErr) {
		switch trackingErr.Code {
		case trackingErrors.RiderAccessNotFound:
			return api.Fail(http.StatusNotFound, coreErrors.BuildErrorSingle(c, trackingErr.Code))
		case trackingErrors.RiderAccessUsed:
			return api.Fail(http.StatusConflict, coreErrors.BuildErrorSingle(c, trackingErr.Code))
		case trackingErrors.RiderAccessExpired:
			return api.Fail(http.StatusGone, coreErrors.BuildErrorSingle(c, trackingErr.Code))
		}
	}
	return accessError(c, err)
}

// IssueRiderAccessCodeInput is IssueRiderAccessCode's request.
type IssueRiderAccessCodeInput struct {
	RiderID string `path:"rider_id"`
}

// IssueRiderAccessCode is POST /riders/{rider_id}/access-code: A passenger's QR to sign in.
func (ctrl *AccessController) IssueRiderAccessCode(ctx context.Context, _ *IssueRiderAccessCodeInput) (*api.Out[*models.RiderAccessCode], error) {
	c := api.Gin(ctx)
	tenantID, ids, err := accessIDs(c, "rider_id")
	if err != nil {
		return nil, err
	}
	var actingUser *uuid.UUID
	if id, err := uuid.Parse(c.GetString("user_id")); err == nil {
		actingUser = &id
	}
	code, err := ctrl.access.IssueRiderAccessCode(c.Request.Context(), tenantID, c.GetString("tenant_slug"), ids[0], actingUser,
		userIDAtLevel(c, tenancyModels.RoleOrganization))
	if err != nil {
		return nil, accessError(c, err)
	}
	return api.OK(code)
}

// RevokeRiderAccessCodeInput is RevokeRiderAccessCode's request.
type RevokeRiderAccessCodeInput struct {
	RiderID string `path:"rider_id"`
}

// RevokeRiderAccessCode is DELETE /riders/{rider_id}/access-code: End a passenger's QR.
func (ctrl *AccessController) RevokeRiderAccessCode(ctx context.Context, _ *RevokeRiderAccessCodeInput) (*api.Empty, error) {
	c := api.Gin(ctx)
	tenantID, ids, err := accessIDs(c, "rider_id")
	if err != nil {
		return nil, err
	}
	if err := ctrl.access.RevokeRiderAccessCode(c.Request.Context(), tenantID, ids[0], userIDAtLevel(c, tenancyModels.RoleOrganization)); err != nil {
		return nil, accessError(c, err)
	}
	return api.Done()
}

// CheckRiderAccessCodeInput is CheckRiderAccessCode's request.
type CheckRiderAccessCodeInput struct {
	Token string `path:"token"`
}

// CheckRiderAccessCode is POST /public/rider-access/{token}/check: Ready an account for a scanned QR.
func (ctrl *AccessController) CheckRiderAccessCode(ctx context.Context, in *CheckRiderAccessCodeInput) (*api.Empty, error) {
	c := api.Gin(ctx)
	tenantID, err := uuid.Parse(c.GetString("tenant_id"))
	if err != nil {
		return nil, api.Fail(http.StatusNotFound, coreErrors.BuildErrorSingle(c, trackingErrors.RiderAccessNotFound))
	}
	var dto models.CheckRiderAccessCodeDto
	if err := bindGrant(c, &dto); err != nil {
		return nil, err
	}
	if err := ctrl.access.CheckRiderAccessCode(c.Request.Context(), tenantID, in.Token, dto.Email); err != nil {
		return nil, riderAccessError(c, err)
	}
	return api.Done()
}

// RedeemRiderAccessCodeInput is RedeemRiderAccessCode's request.
type RedeemRiderAccessCodeInput struct {
	Token string `path:"token"`
}

// RedeemRiderAccessCode is POST /public/rider-access/{token}/redeem: Use a QR, signed in.
func (ctrl *AccessController) RedeemRiderAccessCode(ctx context.Context, in *RedeemRiderAccessCodeInput) (*api.Out[models.RiderAccessRedeemed], error) {
	c := api.Gin(ctx)
	tenantID, err := uuid.Parse(c.GetString("tenant_id"))
	if err != nil {
		return nil, api.Fail(http.StatusNotFound, coreErrors.BuildErrorSingle(c, trackingErrors.RiderAccessNotFound))
	}
	userID, err := uuid.Parse(c.GetString("user_id"))
	if err != nil {
		return nil, api.Fail(http.StatusForbidden, coreErrors.BuildErrorSingle(c, trackingErrors.RiderAccessNotFound))
	}
	riderID, err := ctrl.access.RedeemRiderAccessCode(c.Request.Context(), tenantID, in.Token, userID)
	if err != nil {
		return nil, riderAccessError(c, err)
	}
	return api.OK(models.RiderAccessRedeemed{RiderID: riderID, TenantSlug: c.GetString("tenant_slug")})
}
