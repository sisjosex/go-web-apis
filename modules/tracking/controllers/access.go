package controllers

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/leebenson/conform"

	"josex/web/modules/core/api"
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
	trackingErrors.InviteNotFound:                 http.StatusNotFound,
	trackingErrors.InviteTargetRequired:           http.StatusBadRequest,
}

func accessError(c *gin.Context, err error) error {
	var trackingErr *trackingErrors.TrackingError
	if errors.As(err, &trackingErr) {
		if status, ok := accessStatus[trackingErr.Code]; ok {
			if trackingErr.Detail != nil {
				return api.Fail(status, coreErrors.BuildErrorDetail(c, trackingErr.Code, trackingErr.Detail))
			}
			return api.Fail(status, coreErrors.BuildErrorSingle(c, trackingErr.Code))
		}
	}
	return api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
}

// accessIDs reads the tenant and the path ids a handler names, answering 403/400 itself.
func accessIDs(c *gin.Context, params ...string) (uuid.UUID, []uuid.UUID, error) {
	tenantID, err := uuid.Parse(c.GetString("tenant_id"))
	if err != nil {
		return uuid.Nil, nil, api.Fail(http.StatusForbidden, coreErrors.BuildErrorSingle(c, trackingErrors.TenantRequired))
	}
	ids := make([]uuid.UUID, len(params))
	for i, p := range params {
		if ids[i], err = uuid.Parse(c.Param(p)); err != nil {
			return uuid.Nil, nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
		}
	}
	return tenantID, ids, nil
}

func bindGrant(c *gin.Context, dto any) error {
	if err := c.ShouldBindJSON(dto); err != nil {
		return api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.AccessGrantFailed, utils.ExtractValidationError(c, err)))
	}
	_ = conform.Strings(dto)
	return nil
}

// AccessControllerCreateRiderInput is CreateRider's request.
type AccessControllerCreateRiderInput struct {
	JSON api.Body[models.CreateRiderDto]
}

// CreateRider is POST /riders: Create rider.
func (ctrl *AccessController) CreateRider(ctx context.Context, in *AccessControllerCreateRiderInput) (*api.Out[*models.CreatedRider], error) {
	c := api.Gin(ctx)
	tenantID, _, err := accessIDs(c)
	if err != nil {
		return nil, err
	}
	var dto models.CreateRiderDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.RiderCreateFailed, utils.ExtractValidationError(c, err)))
	}
	_ = conform.Strings(&dto)
	created, err := ctrl.access.CreateRider(c.Request.Context(), tenantID, &dto,
		userIDAtLevel(c, tenancyModels.RoleOrganization), c.GetString("lang"))
	if err != nil {
		return nil, accessError(c, err)
	}
	return api.OK(created)
}

// AccessControllerCreateDriverInput is CreateDriver's request.
type AccessControllerCreateDriverInput struct {
	JSON api.Body[models.CreateDriverDto]
}

// CreateDriver is POST /drivers: Create driver.
func (ctrl *AccessController) CreateDriver(ctx context.Context, in *AccessControllerCreateDriverInput) (*api.Out[*models.CreatedDriver], error) {
	c := api.Gin(ctx)
	tenantID, _, err := accessIDs(c)
	if err != nil {
		return nil, err
	}
	var dto models.CreateDriverDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.DriverCreateFailed, utils.ExtractValidationError(c, err)))
	}
	_ = conform.Strings(&dto)
	created, err := ctrl.access.CreateDriver(c.Request.Context(), tenantID, &dto, c.GetString("lang"))
	if err != nil {
		if err := driverErrorResponse(c, err); err != nil {
			return nil, err
		}
		return nil, accessError(c, err)
	}
	return api.OK(created)
}

// ListRiderGuardiansInput is ListRiderGuardians's request.
type ListRiderGuardiansInput struct {
	RiderID string `path:"rider_id"`
}

// ListRiderGuardians is GET /riders/{rider_id}/guardians: A rider's family accounts.
func (ctrl *AccessController) ListRiderGuardians(ctx context.Context, _ *ListRiderGuardiansInput) (*api.Out[[]*models.RiderGuardian], error) {
	c := api.Gin(ctx)
	tenantID, ids, err := accessIDs(c, "rider_id")
	if err != nil {
		return nil, err
	}
	guardians, err := ctrl.access.ListRiderGuardians(c.Request.Context(), tenantID, ids[0],
		userIDAtLevel(c, tenancyModels.RoleOrganization), c.GetString("lang"))
	if err != nil {
		return nil, accessError(c, err)
	}
	return api.OK(guardians)
}

// AddRiderGuardianInput is AddRiderGuardian's request.
type AddRiderGuardianInput struct {
	RiderID string `path:"rider_id"`
}

// AddRiderGuardian is POST /riders/{rider_id}/guardians: Add a rider's guardian.
func (ctrl *AccessController) AddRiderGuardian(ctx context.Context, _ *AddRiderGuardianInput) (*api.Out[*models.AccessGranted], error) {
	c := api.Gin(ctx)
	tenantID, ids, err := accessIDs(c, "rider_id")
	if err != nil {
		return nil, err
	}
	var dto models.NewRiderGuardianDto
	if err := bindGrant(c, &dto); err != nil {
		return nil, err
	}
	granted, err := ctrl.access.AddRiderGuardian(c.Request.Context(), tenantID, ids[0], &dto,
		userIDAtLevel(c, tenancyModels.RoleOrganization), c.GetString("lang"))
	if err != nil {
		return nil, accessError(c, err)
	}
	return api.OK(granted)
}

// RemoveRiderGuardianInput is RemoveRiderGuardian's request.
type RemoveRiderGuardianInput struct {
	RiderID string `path:"rider_id"`
	UserID  string `path:"user_id"`
}

// RemoveRiderGuardian is DELETE /riders/{rider_id}/guardians/{user_id}: Take a family account off a rider.
func (ctrl *AccessController) RemoveRiderGuardian(ctx context.Context, _ *RemoveRiderGuardianInput) (*api.Empty, error) {
	c := api.Gin(ctx)
	tenantID, ids, err := accessIDs(c, "rider_id", "user_id")
	if err != nil {
		return nil, err
	}
	if err := ctrl.access.RemoveRiderGuardian(c.Request.Context(), tenantID, ids[0], ids[1],
		userIDAtLevel(c, tenancyModels.RoleOrganization)); err != nil {
		return nil, accessError(c, err)
	}
	return api.Done()
}

// SetDriverAccountInput is SetDriverAccount's request.
type SetDriverAccountInput struct {
	DriverID string `path:"driver_id"`
}

// SetDriverAccount is PUT /drivers/{driver_id}/account: Give a driver the app.
func (ctrl *AccessController) SetDriverAccount(ctx context.Context, _ *SetDriverAccountInput) (*api.Out[*models.AccessGranted], error) {
	c := api.Gin(ctx)
	tenantID, ids, err := accessIDs(c, "driver_id")
	if err != nil {
		return nil, err
	}
	var dto models.GrantAccessDto
	if err := bindGrant(c, &dto); err != nil {
		return nil, err
	}
	granted, err := ctrl.access.SetDriverAccount(c.Request.Context(), tenantID, ids[0], &dto, c.GetString("lang"))
	if err != nil {
		return nil, accessError(c, err)
	}
	return api.OK(granted)
}

// ClearDriverAccountInput is ClearDriverAccount's request.
type ClearDriverAccountInput struct {
	DriverID string `path:"driver_id"`
}

// ClearDriverAccount is DELETE /drivers/{driver_id}/account: Take the app away from a driver.
func (ctrl *AccessController) ClearDriverAccount(ctx context.Context, _ *ClearDriverAccountInput) (*api.Empty, error) {
	c := api.Gin(ctx)
	tenantID, ids, err := accessIDs(c, "driver_id")
	if err != nil {
		return nil, err
	}
	if err := ctrl.access.ClearDriverAccount(c.Request.Context(), tenantID, ids[0]); err != nil {
		return nil, accessError(c, err)
	}
	return api.Done()
}

// InviteOrganizationMemberInput is InviteOrganizationMember's request.
type InviteOrganizationMemberInput struct {
	OrganizationID string `path:"organization_id"`
}

// InviteOrganizationMember is POST /organizations/{organization_id}/members: Add an organization member, found or created.
func (ctrl *AccessController) InviteOrganizationMember(ctx context.Context, _ *InviteOrganizationMemberInput) (*api.Out[*models.AccessGranted], error) {
	c := api.Gin(ctx)
	tenantID, ids, err := accessIDs(c, "organization_id")
	if err != nil {
		return nil, err
	}
	var dto models.GrantMemberDto
	if err := bindGrant(c, &dto); err != nil {
		return nil, err
	}
	granted, err := ctrl.access.InviteOrganizationMember(c.Request.Context(), tenantID, ids[0], &dto,
		userIDAtLevel(c, tenancyModels.RoleOrganization), c.GetString("lang"))
	if err != nil {
		return nil, accessError(c, err)
	}
	return api.OK(granted)
}

// AccessControllerUpsertOrganizationMemberInput is UpsertOrganizationMember's request.
type AccessControllerUpsertOrganizationMemberInput struct {
	OrganizationID string `path:"organization_id"`
	UserID         string `path:"user_id"`
	JSON           api.Body[models.UpsertOrganizationMemberDto]
}

// UpsertOrganizationMember is PUT /organizations/{organization_id}/members/{user_id}: Change an organization member's role.
func (ctrl *AccessController) UpsertOrganizationMember(ctx context.Context, in *AccessControllerUpsertOrganizationMemberInput) (*api.Out[*models.AccessGranted], error) {
	c := api.Gin(ctx)
	tenantID, ids, err := accessIDs(c, "organization_id", "user_id")
	if err != nil {
		return nil, err
	}
	var body models.UpsertOrganizationMemberDto
	if err := in.JSON.Into(ctx, &body); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.OrganizationMemberSaveFailed, utils.ExtractValidationError(c, err)))
	}
	_ = conform.Strings(&body)
	dto := models.GrantMemberDto{GrantAccessDto: models.GrantAccessDto{UserID: &ids[1]}, Role: body.Role, Groups: body.Groups}
	granted, err := ctrl.access.InviteOrganizationMember(c.Request.Context(), tenantID, ids[0], &dto,
		userIDAtLevel(c, tenancyModels.RoleOrganization), c.GetString("lang"))
	if err != nil {
		return nil, accessError(c, err)
	}
	return api.OK(granted)
}

// RemoveOrganizationMemberInput is RemoveOrganizationMember's request.
type RemoveOrganizationMemberInput struct {
	OrganizationID string `path:"organization_id"`
	UserID         string `path:"user_id"`
}

// RemoveOrganizationMember is DELETE /organizations/{organization_id}/members/{user_id}: Remove an organization member.
func (ctrl *AccessController) RemoveOrganizationMember(ctx context.Context, _ *RemoveOrganizationMemberInput) (*api.Empty, error) {
	c := api.Gin(ctx)
	tenantID, ids, err := accessIDs(c, "organization_id", "user_id")
	if err != nil {
		return nil, err
	}
	if err := ctrl.access.RemoveOrganizationMember(c.Request.Context(), tenantID, ids[0], ids[1],
		userIDAtLevel(c, tenancyModels.RoleOrganization)); err != nil {
		return nil, accessError(c, err)
	}
	return api.Done()
}
