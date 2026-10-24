package controllers

import (
	"context"
	"errors"
	"net/http"

	"josex/web/modules/core/api"
	coreErrors "josex/web/modules/core/errors"
	"josex/web/modules/core/utils"
	trackingErrors "josex/web/modules/tracking/errors"
	"josex/web/modules/tracking/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// The vehicle-centred assignment of TRACK-034, beside tracking_controller.go: the usual driver lives
// on the vehicle (D1) and an assignment is checked for schedule clashes before it is saved (D2).

// vehicleSetupStatus is the status each code answers; a code not listed is a 500.
var vehicleSetupStatus = map[string]int{
	trackingErrors.VehicleNotFound:              http.StatusNotFound,
	trackingErrors.VehicleDriverCompanyMismatch: http.StatusUnprocessableEntity,
	trackingErrors.CompanyNotFound:              http.StatusNotFound,
	trackingErrors.VehiclePlateAlreadyExists:    http.StatusConflict,
	trackingErrors.RouteCodeAlreadyExists:       http.StatusConflict,
	trackingErrors.ServiceBlocked:               http.StatusConflict,
}

func vehicleSetupError(c *gin.Context, err error) error {
	var trackingErr *trackingErrors.TrackingError
	if errors.As(err, &trackingErr) {
		if status, ok := vehicleSetupStatus[trackingErr.Code]; ok {
			if trackingErr.Detail != nil {
				return api.Fail(status, coreErrors.BuildErrorDetail(c, trackingErr.Code, trackingErr.Detail))
			} else {
				return api.Fail(status, coreErrors.BuildErrorSingle(c, trackingErr.Code))
			}

		}
	}
	return api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
}

// optionalUUID parses a query value that may be absent; ok is false when it is present and malformed.
func optionalUUID(raw *string) (*uuid.UUID, bool) {
	if raw == nil || *raw == "" {
		return nil, true
	}
	id, err := uuid.Parse(*raw)
	if err != nil {
		return nil, false
	}
	return &id, true
}

// SetVehicleDriverInput is SetVehicleDriver's request.
type SetVehicleDriverInput struct {
	VehicleID string `path:"vehicle_id"`
	JSON      api.Body[models.SetVehicleDriverDto]
}

// SetVehicleDriver is PUT /vehicles/{vehicle_id}/driver: Set a vehicle's usual driver.
func (ctrl *TrackingController) SetVehicleDriver(ctx context.Context, in *SetVehicleDriverInput) (*api.Out[*models.Vehicle], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}
	vehicleID, err := uuid.Parse(in.VehicleID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
	}
	var dto models.SetVehicleDriverDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.VehicleDriverFailed, utils.ExtractValidationError(c, err)))
	}
	vehicle, err := ctrl.trackingService.SetVehicleDriver(c.Request.Context(), tenantID, vehicleID, dto.DriverID)
	if err != nil {
		return nil, vehicleSetupError(c, err)
	}
	return api.OK(vehicle)
}

// MoveVehicleCompanyInput is MoveVehicleCompany's request.
type MoveVehicleCompanyInput struct {
	VehicleID string `path:"vehicle_id"`
	JSON      api.Body[models.MoveVehicleCompanyDto]
}

// MoveVehicleCompany is POST /vehicles/{vehicle_id}/company: Move a vehicle to another company.
func (ctrl *TrackingController) MoveVehicleCompany(ctx context.Context, in *MoveVehicleCompanyInput) (*api.Out[*models.MovedVehicle], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}
	vehicleID, err := uuid.Parse(in.VehicleID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
	}
	var dto models.MoveVehicleCompanyDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.VehicleMoveFailed, utils.ExtractValidationError(c, err)))
	}
	moved, err := ctrl.trackingService.MoveVehicleCompany(c.Request.Context(), tenantID, vehicleID, &dto)
	if err != nil {
		return nil, vehicleSetupError(c, err)
	}
	return api.OK(moved)
}

// VehicleConflictsInput is VehicleConflicts's request.
type VehicleConflictsInput struct {
	VehicleID string `path:"vehicle_id"`
	Query     api.Query[models.VehicleConflictsQuery]
}

// VehicleConflicts is GET /vehicles/{vehicle_id}/conflicts: Schedule clashes of an assignment.
func (ctrl *TrackingController) VehicleConflicts(ctx context.Context, in *VehicleConflictsInput) (*api.Out[*models.VehicleConflictsResponse], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}
	vehicleID, err := uuid.Parse(in.VehicleID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
	}
	var query models.VehicleConflictsQuery
	if err := in.Query.Into(ctx, &query); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.VehicleConflictsFailed, utils.ExtractValidationError(c, err)))
	}
	routeID, okRoute := optionalUUID(query.RouteID)
	driverID, okDriver := optionalUUID(query.DriverID)
	if !okRoute || !okDriver {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
	}
	response, err := ctrl.trackingService.VehicleScheduleConflicts(c.Request.Context(), tenantID, vehicleID, routeID, driverID)
	if err != nil {
		return nil, vehicleSetupError(c, err)
	}
	return api.OK(response)
}
