package controllers

import (
	"fmt"
	"net/http"

	coreErrors "josex/web/modules/core/errors"
	"josex/web/modules/core/utils"
	trackingErrors "josex/web/modules/tracking/errors"
	"josex/web/modules/tracking/interfaces"
	"josex/web/modules/tracking/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// DriverController is /mobile/driver (TRACK-011): the driver's phone. Apart from TrackingController
// because it runs behind the driver's chain, which lifts the mobile-only refusal for that level only.
type DriverController struct {
	driverService interfaces.DriverService
}

func NewDriverController(driverService interfaces.DriverService) *DriverController {
	return &DriverController{driverService: driverService}
}

// driverIDs are the tenant and the account the chain resolved.
func driverIDs(c *gin.Context) (uuid.UUID, uuid.UUID, bool) {
	tenantID, err := uuid.Parse(c.GetString("tenant_id"))
	if err != nil {
		c.JSON(http.StatusForbidden, coreErrors.BuildErrorSingle(c, trackingErrors.TenantRequired))
		return uuid.Nil, uuid.Nil, false
	}
	userID, err := uuid.Parse(c.GetString("user_id"))
	if err != nil {
		c.JSON(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, trackingErrors.TenantRequired))
		return uuid.Nil, uuid.Nil, false
	}
	return tenantID, userID, true
}

// Today godoc
// @Summary The driver's day
// @Description The driver's trips of each route's local today, any status, each with its stops, tasks and riders — what the phone keeps offline. ETag is the hash of the day; sending it back as If-None-Match answers 304 while nothing changed
// @Tags Tracking - Driver
// @Produce json
// @Security BearerAuth
// @Param If-None-Match header string false "ETag of the day the phone holds"
// @Success 200 {object} models.DriverToday
// @Success 304
// @Failure 403 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /mobile/driver/today [get]
func (ctrl *DriverController) Today(c *gin.Context) {
	tenantID, userID, ok := driverIDs(c)
	if !ok {
		return
	}

	day, etag, err := ctrl.driverService.Today(c.Request.Context(), tenantID, userID)
	if err != nil {
		tripErrorResponse(c, err)
		return
	}
	c.Header("ETag", etag)
	c.Header("Cache-Control", "private, no-cache")
	if c.GetHeader("If-None-Match") == etag {
		c.Status(http.StatusNotModified)
		return
	}
	c.JSON(http.StatusOK, day)
}

// StartTrip godoc
// @Summary Start one of the driver's trips
// @Description planned → in_progress. Another driver's trip is 404
// @Tags Tracking - Driver
// @Produce json
// @Security BearerAuth
// @Param trip_id path string true "Trip ID (UUID)"
// @Success 200 {object} models.TripDetail
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 403 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 409 {object} coreErrors.ErrorResponse
// @Router /mobile/driver/trips/{trip_id}/start [post]
func (ctrl *DriverController) StartTrip(c *gin.Context) {
	tenantID, userID, ok := driverIDs(c)
	if !ok {
		return
	}
	tripID, ok := pathUUID(c, "trip_id")
	if !ok {
		return
	}

	detail, err := ctrl.driverService.StartTrip(c.Request.Context(), tenantID, userID, tripID)
	if err != nil {
		tripErrorResponse(c, err)
		return
	}
	c.JSON(http.StatusOK, detail)
}

// Sync godoc
// @Summary Replay what the phone did offline
// @Description Up to 100 ops, applied in the order they were done (client_recorded_at) and stamped with that time. Each answers applied, replayed (already applied by an earlier sync) or rejected with its code; a rejected op sent again is rejected again with the same code. One op's refusal never stops the next
// @Tags Tracking - Driver
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param ops body models.DriverSyncDto true "Queued ops"
// @Success 200 {object} models.DriverSyncResponse
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 403 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /mobile/driver/sync [post]
func (ctrl *DriverController) Sync(c *gin.Context) {
	tenantID, userID, ok := driverIDs(c)
	if !ok {
		return
	}

	var dto models.DriverSyncDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.DriverSyncInvalid, utils.ExtractValidationError(c, err)))
		return
	}
	if detail := checkOps(dto.Ops); detail != "" {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.DriverSyncInvalid, detail))
		return
	}

	results, err := ctrl.driverService.Sync(c.Request.Context(), tenantID, userID, dto.Ops)
	if err != nil {
		tripErrorResponse(c, err)
		return
	}
	c.JSON(http.StatusOK, models.DriverSyncResponse{Results: results})
}

// checkOps is what binding cannot say about an op: that its args name the target its op needs. It
// answers the first problem, or "".
func checkOps(ops []models.DriverSyncOp) string {
	for i, op := range ops {
		var target *uuid.UUID
		switch op.Op {
		case models.DriverOpStopArrive, models.DriverOpStopSkip:
			target = op.Args.TripStopID
		case models.DriverOpTaskDone, models.DriverOpTaskNoShow:
			target = op.Args.TripStopTaskID
		case models.DriverOpTripComplete:
			target = op.Args.TripID
		}
		if target == nil {
			return fmt.Sprintf("op %d: %s needs its target id in args", i, op.Op)
		}
	}
	return ""
}

// ResolveRider godoc
// @Summary Resolve a scanned rider card
// @Description The rider the card's code names and their pending pickup on the driver's trip in progress, null when there is none. The code may be typed by hand: case and blanks are ignored. An unknown, rotated or another tenant's code is 404
// @Tags Tracking - Driver
// @Produce json
// @Security BearerAuth
// @Param code path string true "Card code"
// @Success 200 {object} models.DriverRider
// @Failure 403 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Router /mobile/driver/riders/{code} [get]
func (ctrl *DriverController) ResolveRider(c *gin.Context) {
	tenantID, userID, ok := driverIDs(c)
	if !ok {
		return
	}

	rider, err := ctrl.driverService.ResolveRider(c.Request.Context(), tenantID, userID, c.Param("code"))
	if err != nil {
		tripErrorResponse(c, err)
		return
	}
	c.JSON(http.StatusOK, rider)
}

// FindRiders godoc
// @Summary Find a rider without their card
// @Description The riders on the driver's trips in progress whose name contains q, or whose card code is q, each with their pending pickup; at most 20, by name
// @Tags Tracking - Driver
// @Produce json
// @Security BearerAuth
// @Param q query string true "Part of a name, or a card code"
// @Success 200 {object} models.DriverFindRidersResponse
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 403 {object} coreErrors.ErrorResponse
// @Router /mobile/driver/riders [get]
func (ctrl *DriverController) FindRiders(c *gin.Context) {
	tenantID, userID, ok := driverIDs(c)
	if !ok {
		return
	}
	var query models.DriverFindRidersQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.DriverRiderInvalid, utils.ExtractValidationError(c, err)))
		return
	}

	riders, err := ctrl.driverService.FindRiders(c.Request.Context(), tenantID, userID, query.Q)
	if err != nil {
		tripErrorResponse(c, err)
		return
	}
	c.JSON(http.StatusOK, models.DriverFindRidersResponse{Riders: riders})
}

// ReportIncident godoc
// @Summary Report an incident on one of the driver's trips
// @Description A route alert on the trip's route and vehicle that names the trip and where the phone was; the board hears of it through the trip's change event. Severity follows the type: emergency critical, breakdown high, delay medium. Another driver's trip is 404
// @Tags Tracking - Driver
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param incident body models.DriverIncidentDto true "Incident"
// @Success 201 {object} models.DriverIncidentResponse
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 403 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Router /mobile/driver/incidents [post]
func (ctrl *DriverController) ReportIncident(c *gin.Context) {
	tenantID, userID, ok := driverIDs(c)
	if !ok {
		return
	}
	var dto models.DriverIncidentDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.DriverIncidentInvalid, utils.ExtractValidationError(c, err)))
		return
	}

	alert, err := ctrl.driverService.ReportIncident(c.Request.Context(), tenantID, userID, &dto)
	if err != nil {
		tripErrorResponse(c, err)
		return
	}
	c.JSON(http.StatusCreated, models.DriverIncidentResponse{Alert: *alert})
}
