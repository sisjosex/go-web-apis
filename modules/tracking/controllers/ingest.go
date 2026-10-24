package controllers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"josex/web/modules/core/api"
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

// IngestPositionsInput is IngestPositions's request.
type IngestPositionsInput struct {
	JSON api.Body[[]models.PositionPoint]
}

// IngestPositions is POST /ingest/positions: Post a batch of GPS points.
func (ctrl *IngestController) IngestPositions(ctx context.Context, in *IngestPositionsInput) (*api.Out[models.IngestResponse], error) {
	c := api.Gin(ctx)
	tenantID, err := uuid.Parse(c.GetString("tenant_id"))
	if err != nil {
		return nil, api.Fail(http.StatusForbidden, coreErrors.BuildErrorSingle(c, trackingErrors.TenantRequired))
	}

	var points []models.PositionPoint
	if err := in.JSON.Into(ctx, &points); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.IngestInvalid, utils.ExtractValidationError(c, err)))
	}
	if detail := checkBatch(points, time.Now()); detail != "" {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.IngestInvalid, detail))
	}

	vehicleID, err := ctrl.vehicle(c, tenantID)
	if err != nil {
		return nil, err
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
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildErrorDetail(c, trackingErrors.IngestFailed, err.Error()))
	}
	return api.OK(models.IngestResponse{Accepted: len(stored), Path: path})
}

// vehicle is whose points these are: the device's own, or the vehicle of the driver's trip in
// progress. It writes the refusal itself when there is none.
func (ctrl *IngestController) vehicle(c *gin.Context, tenantID uuid.UUID) (uuid.UUID, error) {
	if raw, ok := c.Get(tenancyMW.GPSVehicleKey); ok {
		return raw.(uuid.UUID), nil
	}
	userID, err := uuid.Parse(c.GetString("user_id"))
	if err != nil {
		return uuid.Nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, trackingErrors.TenantRequired))
	}
	vehicleID, err := ctrl.ingest.DriverVehicle(c.Request.Context(), tenantID, userID)
	if err != nil {
		return uuid.Nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildErrorDetail(c, trackingErrors.IngestFailed, err.Error()))
	}
	if vehicleID == nil {
		return uuid.Nil, api.Fail(http.StatusConflict, coreErrors.BuildErrorSingle(c, trackingErrors.IngestNoActiveTrip))
	}
	return *vehicleID, nil
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
