package controllers

import (
	"context"
	"net/http"

	authErrors "josex/web/modules/auth/errors"
	authInterfaces "josex/web/modules/auth/interfaces"
	authModels "josex/web/modules/auth/models"
	"josex/web/modules/core/api"
	coreErrors "josex/web/modules/core/errors"
	"josex/web/modules/core/utils"

	"github.com/google/uuid"
)

// PushDeviceController is /mobile/devices (TRACK-012): where the signed-in account's phone receives
// pushes. Auth only — a device belongs to the account, not to a tenant, so no slug.
type PushDeviceController struct {
	devices authInterfaces.PushDeviceService
}

func NewPushDeviceController(devices authInterfaces.PushDeviceService) *PushDeviceController {
	return &PushDeviceController{devices: devices}
}

// RegisterDeviceInput is RegisterDevice's request.
type RegisterDeviceInput struct {
	JSON api.Body[authModels.RegisterPushDeviceDto]
}

// RegisterDevice is POST : Register or refresh this phone's push token.
func (ctrl *PushDeviceController) RegisterDevice(ctx context.Context, in *RegisterDeviceInput) (*api.Empty, error) {
	c := api.Gin(ctx)
	userID, err := uuid.Parse(c.GetString("user_id"))
	if err != nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, authErrors.SessionUnauthorized))
	}
	var dto authModels.RegisterPushDeviceDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, authErrors.DeviceInvalid, utils.ExtractValidationError(c, err)))
	}

	if err := ctrl.devices.Register(c.Request.Context(), userID, dto); err != nil {
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildErrorSingle(c, authErrors.DeviceSaveFailed))
	}
	return api.Done()
}

// ForgetDeviceInput is ForgetDevice's request.
type ForgetDeviceInput struct {
	DeviceID string `path:"device_id"`
}

// ForgetDevice is DELETE /{device_id}: Stop pushing to this phone.
func (ctrl *PushDeviceController) ForgetDevice(ctx context.Context, in *ForgetDeviceInput) (*api.Empty, error) {
	c := api.Gin(ctx)
	userID, err := uuid.Parse(c.GetString("user_id"))
	if err != nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, authErrors.SessionUnauthorized))
	}

	if err := ctrl.devices.Forget(c.Request.Context(), userID, in.DeviceID); err != nil {
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildErrorSingle(c, authErrors.DeviceSaveFailed))
	}
	return api.Done()
}
