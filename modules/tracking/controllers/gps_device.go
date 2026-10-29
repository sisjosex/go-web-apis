package controllers

import (
	"context"
	"crypto/rand"
	"errors"
	"net/http"
	"time"

	"josex/web/modules/core/api"
	coreErrors "josex/web/modules/core/errors"
	tenancyInterfaces "josex/web/modules/tenancy/interfaces"
	tenancyRepos "josex/web/modules/tenancy/repositories"
	trackingErrors "josex/web/modules/tracking/errors"
	"josex/web/modules/tracking/models"
	"josex/web/modules/tracking/services"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// GPSDeviceController issues, reads and revokes a vehicle's GPS device token from the vehicle page
// (TRACK-068). The tokens live in the platform database (tenancy.gps_devices); the vehicle is checked in
// the tenant's first, so a vehicle of another tenant answers 404.
type GPSDeviceController struct {
	tracking *services.TrackingService
	devices  tenancyInterfaces.GPSDeviceRepository
}

func NewGPSDeviceController(tracking *services.TrackingService, devices tenancyInterfaces.GPSDeviceRepository) *GPSDeviceController {
	return &GPSDeviceController{tracking: tracking, devices: devices}
}

// GPSDeviceInput is the request of the three gps-device routes.
type GPSDeviceInput struct {
	VehicleID string `path:"vehicle_id"`
}

// GPSDeviceStatus is GET /vehicles/{vehicle_id}/gps-device: Whether the vehicle has a GPS device.
func (ctrl *GPSDeviceController) GPSDeviceStatus(ctx context.Context, _ *GPSDeviceInput) (*api.Out[models.GPSDeviceStatus], error) {
	c := api.Gin(ctx)
	tenantID, vehicleID, err := ctrl.vehicle(c)
	if err != nil {
		return nil, err
	}
	status, err := ctrl.devices.Status(c.Request.Context(), tenantID, vehicleID)
	if err != nil {
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	if status == nil {
		return api.OK(models.GPSDeviceStatus{})
	}
	return api.OK(models.GPSDeviceStatus{Linked: true, IssuedAt: &status.IssuedAt, LastSeenAt: status.LastSeenAt})
}

// IssueGPSDevice is POST /vehicles/{vehicle_id}/gps-device: Issue or rotate the vehicle's device token.
func (ctrl *GPSDeviceController) IssueGPSDevice(ctx context.Context, _ *GPSDeviceInput) (*api.Out[models.GPSDeviceIssued], error) {
	c := api.Gin(ctx)
	tenantID, vehicleID, err := ctrl.vehicle(c)
	if err != nil {
		return nil, err
	}
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	token := tenancyRepos.GPSDeviceToken(tenantID, secret)
	_, hash, _ := tenancyRepos.ParseGPSDeviceToken(token)
	if err := ctrl.devices.Issue(c.Request.Context(), tenantID, vehicleID, hash); err != nil {
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	issued := models.GPSDeviceIssued{Token: token, IssuedAt: time.Now().UTC()}
	if status, err := ctrl.devices.Status(c.Request.Context(), tenantID, vehicleID); err == nil && status != nil {
		issued.IssuedAt = status.IssuedAt
	}
	return api.OK(issued)
}

// RevokeGPSDevice is DELETE /vehicles/{vehicle_id}/gps-device: Unlink the vehicle's GPS device.
func (ctrl *GPSDeviceController) RevokeGPSDevice(ctx context.Context, _ *GPSDeviceInput) (*api.Empty, error) {
	c := api.Gin(ctx)
	tenantID, vehicleID, err := ctrl.vehicle(c)
	if err != nil {
		return nil, err
	}
	if err := ctrl.devices.Revoke(c.Request.Context(), tenantID, vehicleID); err != nil {
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.Done()
}

// vehicle is the request's tenant and its vehicle, checked to exist in that tenant. It writes the
// refusal itself.
func (ctrl *GPSDeviceController) vehicle(c *gin.Context) (uuid.UUID, uuid.UUID, error) {
	tenantID, err := uuid.Parse(c.GetString("tenant_id"))
	if err != nil {
		return uuid.Nil, uuid.Nil, api.Fail(http.StatusForbidden, coreErrors.BuildErrorSingle(c, trackingErrors.TenantRequired))
	}
	vehicleID, err := pathUUID(c, "vehicle_id")
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	if _, err := ctrl.tracking.GetVehicle(c.Request.Context(), tenantID, vehicleID); err != nil {
		var trackingErr *trackingErrors.TrackingError
		if errors.As(err, &trackingErr) && trackingErr.Code == trackingErrors.VehicleNotFound {
			return uuid.Nil, uuid.Nil, api.Fail(http.StatusNotFound, coreErrors.BuildErrorSingle(c, trackingErr.Code))
		}
		return uuid.Nil, uuid.Nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return tenantID, vehicleID, nil
}
