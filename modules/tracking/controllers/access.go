package controllers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/leebenson/conform"

	coreErrors "josex/web/modules/core/errors"
	"josex/web/modules/core/utils"
	tenancyModels "josex/web/modules/tenancy/models"
	trackingErrors "josex/web/modules/tracking/errors"
	"josex/web/modules/tracking/models"
	"josex/web/modules/tracking/services"
)

// The app-access handlers of TRACK-032: a rider's family, a driver and an organization's members get
// the app from the screen they are managed on. One slice, one file.

// AccessController serves the grant and revoke endpoints. The level is the route's, never the body's.
type AccessController struct {
	access *services.AccessService
}

func NewAccessController(access *services.AccessService) *AccessController {
	return &AccessController{access: access}
}

// accessStatus is the status each access code answers; a code not listed is a 500.
var accessStatus = map[string]int{
	trackingErrors.RiderNotFound:                  http.StatusNotFound,
	trackingErrors.DriverNotFound:                 http.StatusNotFound,
	trackingErrors.OrganizationNotFound:           http.StatusNotFound,
	trackingErrors.RiderGuardianNotFound:          http.StatusNotFound,
	trackingErrors.OrganizationMemberNotFound:     http.StatusNotFound,
	trackingErrors.OrganizationMemberUserNotFound: http.StatusNotFound,
	trackingErrors.AccessUserNotFound:             http.StatusNotFound,
	trackingErrors.AccessWebAccount:               http.StatusConflict,
	trackingErrors.AccessLevelMismatch:            http.StatusConflict,
	trackingErrors.AccessUserDeleted:              http.StatusConflict,
	trackingErrors.DriverUserAlreadyLinked:        http.StatusConflict,
	trackingErrors.OrganizationScopeDenied:        http.StatusForbidden,
	trackingErrors.RiderGuardianInvalid:           http.StatusBadRequest,
}

func accessError(c *gin.Context, err error) {
	var trackingErr *trackingErrors.TrackingError
	if errors.As(err, &trackingErr) {
		if status, ok := accessStatus[trackingErr.Code]; ok {
			if trackingErr.Detail != nil {
				c.JSON(status, coreErrors.BuildErrorDetail(c, trackingErr.Code, trackingErr.Detail))
				return
			}
			c.JSON(status, coreErrors.BuildErrorSingle(c, trackingErr.Code))
			return
		}
	}
	c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
}

// accessIDs reads the tenant and the path ids a handler names, answering 403/400 itself.
func accessIDs(c *gin.Context, params ...string) (uuid.UUID, []uuid.UUID, bool) {
	tenantID, err := uuid.Parse(c.GetString("tenant_id"))
	if err != nil {
		c.JSON(http.StatusForbidden, coreErrors.BuildErrorSingle(c, trackingErrors.TenantRequired))
		return uuid.Nil, nil, false
	}
	ids := make([]uuid.UUID, len(params))
	for i, p := range params {
		if ids[i], err = uuid.Parse(c.Param(p)); err != nil {
			c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
			return uuid.Nil, nil, false
		}
	}
	return tenantID, ids, true
}

func bindGrant(c *gin.Context, dto any) bool {
	if err := c.ShouldBindJSON(dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.AccessGrantFailed, utils.ExtractValidationError(c, err)))
		return false
	}
	_ = conform.Strings(dto)
	return true
}

// CreateRider godoc
// @Summary Create rider
// @Description A rider (student/employee) with up to 5 guardians in one request (TRACK-037 D2): an account by user_id, a person to invite by email and name, or a contact by name and phone. A refused guardian names its index in detail and nothing is created.
// @Tags Tracking - Riders
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param rider body models.CreateRiderDto true "Rider data"
// @Success 201 {object} models.CreatedRider
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 409 {object} coreErrors.ErrorResponse
// @Router /tracking/riders [post]
func (ctrl *AccessController) CreateRider(c *gin.Context) {
	tenantID, _, ok := accessIDs(c)
	if !ok {
		return
	}
	var dto models.CreateRiderDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.RiderCreateFailed, utils.ExtractValidationError(c, err)))
		return
	}
	_ = conform.Strings(&dto)
	created, err := ctrl.access.CreateRider(c.Request.Context(), tenantID, &dto,
		userIDAtLevel(c, tenancyModels.RoleOrganization), c.GetString("lang"))
	if err != nil {
		accessError(c, err)
		return
	}
	c.JSON(http.StatusCreated, created)
}

// ListRiderGuardians godoc
// @Summary A rider's family accounts
// @Description The guardian accounts that see the rider in the app (D3), each with the access notice to copy
// @Tags Tracking - Access
// @Produce json
// @Security BearerAuth
// @Param rider_id path string true "Rider ID (UUID)"
// @Success 200 {array} models.RiderGuardian
// @Failure 404 {object} coreErrors.ErrorResponse
// @Router /tracking/riders/{rider_id}/guardians [get]
func (ctrl *AccessController) ListRiderGuardians(c *gin.Context) {
	tenantID, ids, ok := accessIDs(c, "rider_id")
	if !ok {
		return
	}
	guardians, err := ctrl.access.ListRiderGuardians(c.Request.Context(), tenantID, ids[0],
		userIDAtLevel(c, tenancyModels.RoleOrganization), c.GetString("lang"))
	if err != nil {
		accessError(c, err)
		return
	}
	c.JSON(http.StatusOK, guardians)
}

// AddRiderGuardian godoc
// @Summary Give a rider's family the app
// @Description An existing account by user_id, or a person by email and name, found or created at the portal level; 409 when the email has web access or another app level (D4)
// @Tags Tracking - Access
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param rider_id path string true "Rider ID (UUID)"
// @Param body body models.GrantAccessDto true "Account or person"
// @Success 201 {object} models.AccessGranted
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 409 {object} coreErrors.ErrorResponse
// @Router /tracking/riders/{rider_id}/guardians [post]
func (ctrl *AccessController) AddRiderGuardian(c *gin.Context) {
	tenantID, ids, ok := accessIDs(c, "rider_id")
	if !ok {
		return
	}
	var dto models.GrantAccessDto
	if !bindGrant(c, &dto) {
		return
	}
	granted, err := ctrl.access.AddRiderGuardian(c.Request.Context(), tenantID, ids[0], &dto,
		userIDAtLevel(c, tenancyModels.RoleOrganization), c.GetString("lang"))
	if err != nil {
		accessError(c, err)
		return
	}
	c.JSON(http.StatusCreated, granted)
}

// RemoveRiderGuardian godoc
// @Summary Take a family account off a rider
// @Description The account's portal access ends when no other rider names it
// @Tags Tracking - Access
// @Security BearerAuth
// @Param rider_id path string true "Rider ID (UUID)"
// @Param user_id path string true "User ID (UUID)"
// @Success 204 "No Content"
// @Failure 404 {object} coreErrors.ErrorResponse
// @Router /tracking/riders/{rider_id}/guardians/{user_id} [delete]
func (ctrl *AccessController) RemoveRiderGuardian(c *gin.Context) {
	tenantID, ids, ok := accessIDs(c, "rider_id", "user_id")
	if !ok {
		return
	}
	if err := ctrl.access.RemoveRiderGuardian(c.Request.Context(), tenantID, ids[0], ids[1],
		userIDAtLevel(c, tenancyModels.RoleOrganization)); err != nil {
		accessError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// SetDriverAccount godoc
// @Summary Give a driver the app
// @Description Links an account at the driver level, found or created; the account it replaces loses the level when nothing else names it
// @Tags Tracking - Access
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param driver_id path string true "Driver ID (UUID)"
// @Param body body models.GrantAccessDto true "Account or person"
// @Success 201 {object} models.AccessGranted
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 409 {object} coreErrors.ErrorResponse
// @Router /tracking/drivers/{driver_id}/account [put]
func (ctrl *AccessController) SetDriverAccount(c *gin.Context) {
	tenantID, ids, ok := accessIDs(c, "driver_id")
	if !ok {
		return
	}
	var dto models.GrantAccessDto
	if !bindGrant(c, &dto) {
		return
	}
	granted, err := ctrl.access.SetDriverAccount(c.Request.Context(), tenantID, ids[0], &dto, c.GetString("lang"))
	if err != nil {
		accessError(c, err)
		return
	}
	c.JSON(http.StatusCreated, granted)
}

// ClearDriverAccount godoc
// @Summary Take the app away from a driver
// @Tags Tracking - Access
// @Security BearerAuth
// @Param driver_id path string true "Driver ID (UUID)"
// @Success 204 "No Content"
// @Failure 404 {object} coreErrors.ErrorResponse
// @Router /tracking/drivers/{driver_id}/account [delete]
func (ctrl *AccessController) ClearDriverAccount(c *gin.Context) {
	tenantID, ids, ok := accessIDs(c, "driver_id")
	if !ok {
		return
	}
	if err := ctrl.access.ClearDriverAccount(c.Request.Context(), tenantID, ids[0]); err != nil {
		accessError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// InviteOrganizationMember godoc
// @Summary Add an organization member, found or created
// @Description The account gets the organization level; 409 when the email has web access or another app level (D4)
// @Tags Tracking - Access
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param organization_id path string true "Organization ID (UUID)"
// @Param body body models.GrantMemberDto true "Account or person, and the role"
// @Success 201 {object} models.AccessGranted
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 409 {object} coreErrors.ErrorResponse
// @Router /tracking/organizations/{organization_id}/members [post]
func (ctrl *AccessController) InviteOrganizationMember(c *gin.Context) {
	tenantID, ids, ok := accessIDs(c, "organization_id")
	if !ok {
		return
	}
	var dto models.GrantMemberDto
	if !bindGrant(c, &dto) {
		return
	}
	granted, err := ctrl.access.InviteOrganizationMember(c.Request.Context(), tenantID, ids[0], &dto, c.GetString("lang"))
	if err != nil {
		accessError(c, err)
		return
	}
	c.JSON(http.StatusCreated, granted)
}

// UpsertOrganizationMember godoc
// @Summary Change an organization member's role
// @Description The same grant as POST with the account named by the path, so it holds to the organization level as well
// @Tags Tracking - Access
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param organization_id path string true "Organization ID (UUID)"
// @Param user_id path string true "User ID (UUID)"
// @Param member body models.UpsertOrganizationMemberDto true "Member role"
// @Success 200 {object} models.AccessGranted
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 409 {object} coreErrors.ErrorResponse
// @Router /tracking/organizations/{organization_id}/members/{user_id} [put]
func (ctrl *AccessController) UpsertOrganizationMember(c *gin.Context) {
	tenantID, ids, ok := accessIDs(c, "organization_id", "user_id")
	if !ok {
		return
	}
	var body models.UpsertOrganizationMemberDto
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.OrganizationMemberSaveFailed, utils.ExtractValidationError(c, err)))
		return
	}
	_ = conform.Strings(&body)
	dto := models.GrantMemberDto{GrantAccessDto: models.GrantAccessDto{UserID: &ids[1]}, Role: body.Role}
	granted, err := ctrl.access.InviteOrganizationMember(c.Request.Context(), tenantID, ids[0], &dto, c.GetString("lang"))
	if err != nil {
		accessError(c, err)
		return
	}
	c.JSON(http.StatusOK, granted)
}

// RemoveOrganizationMember godoc
// @Summary Remove an organization member
// @Description The account's organization level ends when nothing else names it
// @Tags Tracking - Access
// @Security BearerAuth
// @Param organization_id path string true "Organization ID (UUID)"
// @Param user_id path string true "User ID (UUID)"
// @Success 204 "No Content"
// @Failure 404 {object} coreErrors.ErrorResponse
// @Router /tracking/organizations/{organization_id}/members/{user_id} [delete]
func (ctrl *AccessController) RemoveOrganizationMember(c *gin.Context) {
	tenantID, ids, ok := accessIDs(c, "organization_id", "user_id")
	if !ok {
		return
	}
	if err := ctrl.access.RemoveOrganizationMember(c.Request.Context(), tenantID, ids[0], ids[1]); err != nil {
		accessError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
