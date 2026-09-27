package controllers

import (
	"net/http"

	authErrors "josex/web/modules/auth/errors"
	authInterfaces "josex/web/modules/auth/interfaces"
	authModels "josex/web/modules/auth/models"
	coreErrors "josex/web/modules/core/errors"
	"josex/web/modules/core/utils"

	"github.com/gin-gonic/gin"
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

// RegisterDevice godoc
// @Summary Register or refresh this phone's push token
// @Description Upsert on (account, device_id): send it on every app start, the token rotates. A token registered under another account moves to this one
// @Tags Mobile
// @Accept json
// @Security BearerAuth
// @Param body body authModels.RegisterPushDeviceDto true "The device"
// @Success 204
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 401 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /mobile/devices [post]
func (ctrl *PushDeviceController) RegisterDevice(c *gin.Context) {
	userID, err := uuid.Parse(c.GetString("user_id"))
	if err != nil {
		c.JSON(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, authErrors.SessionUnauthorized))
		return
	}
	var dto authModels.RegisterPushDeviceDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, authErrors.DeviceInvalid, utils.ExtractValidationError(c, err)))
		return
	}

	if err := ctrl.devices.Register(c.Request.Context(), userID, dto); err != nil {
		c.JSON(http.StatusInternalServerError, coreErrors.BuildErrorSingle(c, authErrors.DeviceSaveFailed))
		return
	}
	c.Status(http.StatusNoContent)
}

// ForgetDevice godoc
// @Summary Stop pushing to this phone
// @Description Call on logout. An unknown device_id is not an error
// @Tags Mobile
// @Security BearerAuth
// @Param device_id path string true "The app's install id"
// @Success 204
// @Failure 401 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /mobile/devices/{device_id} [delete]
func (ctrl *PushDeviceController) ForgetDevice(c *gin.Context) {
	userID, err := uuid.Parse(c.GetString("user_id"))
	if err != nil {
		c.JSON(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, authErrors.SessionUnauthorized))
		return
	}

	if err := ctrl.devices.Forget(c.Request.Context(), userID, c.Param("device_id")); err != nil {
		c.JSON(http.StatusInternalServerError, coreErrors.BuildErrorSingle(c, authErrors.DeviceSaveFailed))
		return
	}
	c.Status(http.StatusNoContent)
}
