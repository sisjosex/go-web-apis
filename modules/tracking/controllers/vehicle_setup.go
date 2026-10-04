package controllers

import (
	"errors"
	"net/http"

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
}

func vehicleSetupError(c *gin.Context, err error) {
	var trackingErr *trackingErrors.TrackingError
	if errors.As(err, &trackingErr) {
		if status, ok := vehicleSetupStatus[trackingErr.Code]; ok {
			c.JSON(status, coreErrors.BuildErrorSingle(c, trackingErr.Code))
			return
		}
	}
	c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
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

// SetVehicleDriver godoc
// @Summary Set a vehicle's usual driver
// @Description Who usually drives the vehicle; its routes without their own driver run with this one
// @Description from today (TRACK-034 D1). A null driver_id clears it.
// @Tags Tracking - Vehicles
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param vehicle_id path string true "Vehicle ID (UUID)"
// @Param body body models.SetVehicleDriverDto true "The driver, or null"
// @Success 200 {object} models.Vehicle
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 422 {object} coreErrors.ErrorResponse
// @Router /tracking/vehicles/{vehicle_id}/driver [put]
func (ctrl *TrackingController) SetVehicleDriver(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}
	vehicleID, err := uuid.Parse(c.Param("vehicle_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
		return
	}
	var dto models.SetVehicleDriverDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.VehicleDriverFailed, utils.ExtractValidationError(c, err)))
		return
	}
	vehicle, err := ctrl.trackingService.SetVehicleDriver(c.Request.Context(), tenantID, vehicleID, dto.DriverID)
	if err != nil {
		vehicleSetupError(c, err)
		return
	}
	c.JSON(http.StatusOK, vehicle)
}

// VehicleConflicts godoc
// @Summary Schedule clashes of an assignment
// @Description The runs that would overlap if route_id ran on the vehicle — or, without it, the
// @Description vehicle's own routes with driver_id driving them (TRACK-034 D2). Nothing is refused:
// @Description the screen warns and lets the operator assign anyway.
// @Tags Tracking - Vehicles
// @Produce json
// @Security BearerAuth
// @Param vehicle_id path string true "Vehicle ID (UUID)"
// @Param route_id query string false "The route being assigned (UUID)"
// @Param driver_id query string false "The driver being set (UUID)"
// @Success 200 {object} models.VehicleConflictsResponse
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Router /tracking/vehicles/{vehicle_id}/conflicts [get]
func (ctrl *TrackingController) VehicleConflicts(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}
	vehicleID, err := uuid.Parse(c.Param("vehicle_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
		return
	}
	var query models.VehicleConflictsQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.VehicleConflictsFailed, utils.ExtractValidationError(c, err)))
		return
	}
	routeID, okRoute := optionalUUID(query.RouteID)
	driverID, okDriver := optionalUUID(query.DriverID)
	if !okRoute || !okDriver {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
		return
	}
	response, err := ctrl.trackingService.VehicleScheduleConflicts(c.Request.Context(), tenantID, vehicleID, routeID, driverID)
	if err != nil {
		vehicleSetupError(c, err)
		return
	}
	c.JSON(http.StatusOK, response)
}
