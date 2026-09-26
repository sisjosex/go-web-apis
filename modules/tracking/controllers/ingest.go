package controllers

import (
	"fmt"
	"net/http"
	"time"

	coreErrors "josex/web/modules/core/errors"
	"josex/web/modules/core/utils"
	tenancyMW "josex/web/modules/tenancy/middleware"
	trackingErrors "josex/web/modules/tracking/errors"
	"josex/web/modules/tracking/models"
	"josex/web/modules/tracking/services"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/google/uuid"
)

// IngestController is POST /tracking/ingest/positions (TRACK-010): a GPS device or a driver's phone
// posting a batch of points. Apart from TrackingController because it answers to two auth chains and
// needs Valkey, which nothing else in the module does.
type IngestController struct {
	ingest *services.PositionIngest
}

func NewIngestController(ingest *services.PositionIngest) *IngestController {
	return &IngestController{ingest: ingest}
}

// IngestPositions godoc
// @Summary Post a batch of GPS points
// @Description Up to 500 points, any order, late ones included. Authenticated either by X-Device-Token (a GPS device: its vehicle) or by a driver's Bearer + X-Tenant-Slug (the vehicle of their trip in progress). Replaying a point is a no-op. 202 path=queued (the worker stores it) or stored (Valkey absent: stored inline)
// @Tags Tracking - Positions
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param points body []models.PositionPoint true "GPS points"
// @Success 202 {object} models.IngestResponse
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 401 {object} coreErrors.ErrorResponse
// @Failure 409 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/ingest/positions [post]
func (ctrl *IngestController) IngestPositions(c *gin.Context) {
	tenantID, err := uuid.Parse(c.GetString("tenant_id"))
	if err != nil {
		c.JSON(http.StatusForbidden, coreErrors.BuildErrorSingle(c, trackingErrors.TenantRequired))
		return
	}

	var points []models.PositionPoint
	if err := c.ShouldBindJSON(&points); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.IngestInvalid, utils.ExtractValidationError(c, err)))
		return
	}
	if detail := checkBatch(points, time.Now()); detail != "" {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.IngestInvalid, detail))
		return
	}

	vehicleID, ok := ctrl.vehicle(c, tenantID)
	if !ok {
		return
	}

	stored := make([]models.StoredPoint, len(points))
	for i, p := range points {
		stored[i] = models.StoredPoint{
			VehicleID: vehicleID, RecordedAt: p.RecordedAt.UTC(), Lat: *p.Lat, Lng: *p.Lng,
			Speed: p.Speed, Heading: p.Heading, Accuracy: p.Accuracy,
		}
	}
	path, err := ctrl.ingest.Ingest(c.Request.Context(), tenantID, stored)
	if err != nil {
		c.JSON(http.StatusInternalServerError, coreErrors.BuildErrorDetail(c, trackingErrors.IngestFailed, err.Error()))
		return
	}
	c.JSON(http.StatusAccepted, models.IngestResponse{Accepted: len(stored), Path: path})
}

// vehicle is whose points these are: the device's own, or the vehicle of the driver's trip in
// progress. It writes the refusal itself when there is none.
func (ctrl *IngestController) vehicle(c *gin.Context, tenantID uuid.UUID) (uuid.UUID, bool) {
	if raw, ok := c.Get(tenancyMW.GPSVehicleKey); ok {
		return raw.(uuid.UUID), true
	}
	userID, err := uuid.Parse(c.GetString("user_id"))
	if err != nil {
		c.JSON(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, trackingErrors.TenantRequired))
		return uuid.Nil, false
	}
	vehicleID, err := ctrl.ingest.DriverVehicle(c.Request.Context(), tenantID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, coreErrors.BuildErrorDetail(c, trackingErrors.IngestFailed, err.Error()))
		return uuid.Nil, false
	}
	if vehicleID == nil {
		c.JSON(http.StatusConflict, coreErrors.BuildErrorSingle(c, trackingErrors.IngestNoActiveTrip))
		return uuid.Nil, false
	}
	return *vehicleID, true
}

// checkBatch is what binding cannot say about a top-level array: its size, each point's fields and
// that no point is stamped in the future. It answers the first problem, or "".
func checkBatch(points []models.PositionPoint, now time.Time) string {
	if len(points) == 0 || len(points) > models.MaxIngestPoints {
		return fmt.Sprintf("between 1 and %d points", models.MaxIngestPoints)
	}
	latest := now.Add(models.IngestFutureSkew)
	for i, p := range points {
		if err := binding.Validator.ValidateStruct(&p); err != nil {
			return fmt.Sprintf("point %d: %v", i, err)
		}
		if p.RecordedAt.After(latest) {
			return fmt.Sprintf("point %d: recorded_at is in the future", i)
		}
	}
	return ""
}
