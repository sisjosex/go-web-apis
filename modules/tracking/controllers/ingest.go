package controllers

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
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

	path, err := ctrl.store(c, tenantID, points)
	if err != nil {
		return nil, err
	}
	return api.OK(models.IngestResponse{Accepted: len(points), Path: path})
}

// IngestOsmandInput is IngestOsmand's request.
type IngestOsmandInput struct {
	Query api.Query[models.OsmandQuery]
}

// IngestOsmand is GET|POST /gps/osmand: one point of a hardware tracker or Traccar Client, in the
// OsmAnd query protocol (TRACK-068 D1). The device chain already resolved the token in id; the point
// takes the ingest's path and the answer is the empty 200 those clients expect.
func (ctrl *IngestController) IngestOsmand(ctx context.Context, in *IngestOsmandInput) (*api.Empty, error) {
	c := api.Gin(ctx)
	tenantID, err := uuid.Parse(c.GetString("tenant_id"))
	if err != nil {
		return nil, api.Fail(http.StatusForbidden, coreErrors.BuildErrorSingle(c, trackingErrors.TenantRequired))
	}
	var q models.OsmandQuery
	if err := in.Query.Into(ctx, &q); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.IngestInvalid, utils.ExtractValidationError(c, err)))
	}
	now := time.Now()
	point, detail := osmandPoint(q, now)
	if detail == "" {
		detail = checkBatch([]models.PositionPoint{point}, now)
	}
	if detail != "" {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.IngestInvalid, detail))
	}
	if _, err := ctrl.store(c, tenantID, []models.PositionPoint{point}); err != nil {
		return nil, err
	}
	return api.Done()
}

// osmandPoint maps the protocol's fields onto a batch point: lon → lng, knots → m/s, bearing →
// heading, a missing timestamp → now. It answers the problem with timestamp, or "".
func osmandPoint(q models.OsmandQuery, now time.Time) (models.PositionPoint, string) {
	at := now
	if q.Timestamp != "" {
		if secs, err := strconv.ParseFloat(q.Timestamp, 64); err == nil {
			at = time.Unix(0, int64(secs*float64(time.Second)))
		} else if t, err := time.Parse(time.RFC3339, q.Timestamp); err == nil {
			at = t
		} else {
			return models.PositionPoint{}, "timestamp: unix seconds or RFC3339"
		}
	}
	point := models.PositionPoint{RecordedAt: at, Lat: q.Lat, Lng: q.Lon, Heading: q.Bearing, Accuracy: q.Accuracy}
	if q.Speed != nil {
		ms := *q.Speed * models.KnotsToMS
		point.Speed = &ms
	}
	return point, ""
}

// store hands checked points to the pipeline under their vehicle, stamped with the receiver that
// sent them: a device token is a tracker, a user session the driver's phone (TRACK-068 D2).
func (ctrl *IngestController) store(c *gin.Context, tenantID uuid.UUID, points []models.PositionPoint) (string, error) {
	vehicleID, err := ctrl.vehicle(c, tenantID)
	if err != nil {
		return "", err
	}
	source := models.PointSourcePhone
	if _, ok := c.Get(tenancyMW.GPSVehicleKey); ok {
		source = models.PointSourceDevice
	}

	stored := make([]models.StoredPoint, len(points))
	for i, p := range points {
		stored[i] = models.StoredPoint{
			VehicleID: vehicleID, RecordedAt: p.RecordedAt.UTC(), Lat: *p.Lat, Lng: *p.Lng,
			Speed: p.Speed, Heading: p.Heading, Accuracy: p.Accuracy, Source: source,
		}
	}
	path, err := ctrl.ingest.Ingest(c.Request.Context(), tenantID, stored)
	if err != nil {
		return "", api.Fail(http.StatusInternalServerError, coreErrors.BuildErrorDetail(c, trackingErrors.IngestFailed, err.Error()))
	}
	return path, nil
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
