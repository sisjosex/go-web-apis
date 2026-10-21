package controllers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	coreErrors "josex/web/modules/core/errors"
	"josex/web/modules/core/utils"
	tenancyModels "josex/web/modules/tenancy/models"
	trackingErrors "josex/web/modules/tracking/errors"
	"josex/web/modules/tracking/models"
)

// The invitation handlers of TRACK-062. An organization user acts in its own scope; the rest of the
// access slice's codes and statuses apply (accessError).

// Invite godoc
// @Summary Invite the families of an organization or of the selected riders
// @Description Every guardian contact without an account gets one by its email, the portal level and the access email; a contact with only a phone, or nothing, is skipped and named (D1)
// @Tags Tracking - Access
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body models.InviteDto true "An organization or riders"
// @Success 200 {object} models.InviteResult
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 403 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Router /tracking/invites [post]
func (ctrl *AccessController) Invite(c *gin.Context) {
	tenantID, _, ok := accessIDs(c)
	if !ok {
		return
	}
	var dto models.InviteDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.InviteTargetRequired, utils.ExtractValidationError(c, err)))
		return
	}
	result, err := ctrl.access.Invite(c.Request.Context(), tenantID, &dto,
		userIDAtLevel(c, tenancyModels.RoleOrganization), actingUser(c), c.GetString("lang"))
	if err != nil {
		accessError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// ListOrganizationInvites godoc
// @Summary An organization's invitations
// @Description The guardian accounts of the organization's riders with their invitation's channel and status (pending, sent, accepted, expired) and the message to send
// @Tags Tracking - Access
// @Produce json
// @Security BearerAuth
// @Param organization_id path string true "Organization ID (UUID)"
// @Param status query string false "Invitation status" Enums(pending, sent, accepted, expired)
// @Param page query int false "Page number, 1-based" default(1)
// @Param page_size query int false "Rows per page, max 100" default(20)
// @Success 200 {object} models.ListOrganizationInvitesResponse
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Router /tracking/organizations/{organization_id}/invites [get]
func (ctrl *AccessController) ListOrganizationInvites(c *gin.Context) {
	tenantID, ids, ok := accessIDs(c, "organization_id")
	if !ok {
		return
	}
	var query models.ListOrganizationInvitesQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.invalid", utils.ExtractValidationError(c, err)))
		return
	}
	result, err := ctrl.access.ListOrganizationInvites(c.Request.Context(), tenantID, ids[0], query,
		userIDAtLevel(c, tenancyModels.RoleOrganization), c.GetString("lang"))
	if err != nil {
		accessError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// MarkInviteSent godoc
// @Summary Record that an invitation was sent
// @Description The operator sent the message by WhatsApp (D2); the invitation turns sent
// @Tags Tracking - Access
// @Security BearerAuth
// @Param user_id path string true "Guardian account ID (UUID)"
// @Success 204 "No Content"
// @Failure 404 {object} coreErrors.ErrorResponse
// @Router /tracking/invites/{user_id}/sent [post]
func (ctrl *AccessController) MarkInviteSent(c *gin.Context) {
	tenantID, ids, ok := accessIDs(c, "user_id")
	if !ok {
		return
	}
	if err := ctrl.access.MarkInviteSent(c.Request.Context(), tenantID, ids[0], userIDAtLevel(c, tenancyModels.RoleOrganization)); err != nil {
		accessError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ResendInvite godoc
// @Summary Send an invitation again
// @Description Answers the message; an email invitation is mailed again and counts as sent now
// @Tags Tracking - Access
// @Produce json
// @Security BearerAuth
// @Param user_id path string true "Guardian account ID (UUID)"
// @Success 200 {object} models.InviteMessage
// @Failure 404 {object} coreErrors.ErrorResponse
// @Router /tracking/invites/{user_id}/resend [post]
func (ctrl *AccessController) ResendInvite(c *gin.Context) {
	tenantID, ids, ok := accessIDs(c, "user_id")
	if !ok {
		return
	}
	message, err := ctrl.access.ResendInvite(c.Request.Context(), tenantID, ids[0],
		userIDAtLevel(c, tenancyModels.RoleOrganization), c.GetString("lang"))
	if err != nil {
		accessError(c, err)
		return
	}
	c.JSON(http.StatusOK, message)
}
