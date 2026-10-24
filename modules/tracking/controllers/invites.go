package controllers

import (
	"context"
	"net/http"

	"josex/web/modules/core/api"
	coreErrors "josex/web/modules/core/errors"
	"josex/web/modules/core/utils"
	tenancyModels "josex/web/modules/tenancy/models"
	trackingErrors "josex/web/modules/tracking/errors"
	"josex/web/modules/tracking/models"
)

// The invitation handlers of TRACK-062. An organization user acts in its own scope; the rest of the
// access slice's codes and statuses apply (accessError).

// InviteInput is Invite's request.
type InviteInput struct {
	JSON api.Body[models.InviteDto]
}

// Invite is POST /invites: Invite the families of an organization or of the selected riders.
func (ctrl *AccessController) Invite(ctx context.Context, in *InviteInput) (*api.Out[*models.InviteResult], error) {
	c := api.Gin(ctx)
	tenantID, _, err := accessIDs(c)
	if err != nil {
		return nil, err
	}
	var dto models.InviteDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.InviteTargetRequired, utils.ExtractValidationError(c, err)))
	}
	result, err := ctrl.access.Invite(c.Request.Context(), tenantID, &dto,
		userIDAtLevel(c, tenancyModels.RoleOrganization), actingUser(c), c.GetString("lang"))
	if err != nil {
		return nil, accessError(c, err)
	}
	return api.OK(result)
}

// ListOrganizationInvitesInput is ListOrganizationInvites's request.
type ListOrganizationInvitesInput struct {
	OrganizationID string `path:"organization_id"`
	Query          api.Query[models.ListOrganizationInvitesQuery]
}

// ListOrganizationInvites is GET /organizations/{organization_id}/invites: An organization's invitations.
func (ctrl *AccessController) ListOrganizationInvites(ctx context.Context, in *ListOrganizationInvitesInput) (*api.Out[*models.ListOrganizationInvitesResponse], error) {
	c := api.Gin(ctx)
	tenantID, ids, err := accessIDs(c, "organization_id")
	if err != nil {
		return nil, err
	}
	var query models.ListOrganizationInvitesQuery
	if err := in.Query.Into(ctx, &query); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.invalid", utils.ExtractValidationError(c, err)))
	}
	result, err := ctrl.access.ListOrganizationInvites(c.Request.Context(), tenantID, ids[0], query,
		userIDAtLevel(c, tenancyModels.RoleOrganization), c.GetString("lang"))
	if err != nil {
		return nil, accessError(c, err)
	}
	return api.OK(result)
}

// MarkInviteSentInput is MarkInviteSent's request.
type MarkInviteSentInput struct {
	UserID string `path:"user_id"`
}

// MarkInviteSent is POST /invites/{user_id}/sent: Record that an invitation was sent.
func (ctrl *AccessController) MarkInviteSent(ctx context.Context, _ *MarkInviteSentInput) (*api.Empty, error) {
	c := api.Gin(ctx)
	tenantID, ids, err := accessIDs(c, "user_id")
	if err != nil {
		return nil, err
	}
	if err := ctrl.access.MarkInviteSent(c.Request.Context(), tenantID, ids[0], userIDAtLevel(c, tenancyModels.RoleOrganization)); err != nil {
		return nil, accessError(c, err)
	}
	return api.Done()
}

// ResendInviteInput is ResendInvite's request.
type ResendInviteInput struct {
	UserID string `path:"user_id"`
}

// ResendInvite is POST /invites/{user_id}/resend: Send an invitation again.
func (ctrl *AccessController) ResendInvite(ctx context.Context, _ *ResendInviteInput) (*api.Out[*models.InviteMessage], error) {
	c := api.Gin(ctx)
	tenantID, ids, err := accessIDs(c, "user_id")
	if err != nil {
		return nil, err
	}
	message, err := ctrl.access.ResendInvite(c.Request.Context(), tenantID, ids[0],
		userIDAtLevel(c, tenancyModels.RoleOrganization), c.GetString("lang"))
	if err != nil {
		return nil, accessError(c, err)
	}
	return api.OK(message)
}
