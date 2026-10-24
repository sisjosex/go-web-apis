package controllers

import (
	"context"
	"fmt"
	"net/http"

	"josex/web/modules/core/api"
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
func driverIDs(c *gin.Context) (uuid.UUID, uuid.UUID, error) {
	tenantID, err := uuid.Parse(c.GetString("tenant_id"))
	if err != nil {
		return uuid.Nil, uuid.Nil, api.Fail(http.StatusForbidden, coreErrors.BuildErrorSingle(c, trackingErrors.TenantRequired))
	}
	userID, err := uuid.Parse(c.GetString("user_id"))
	if err != nil {
		return uuid.Nil, uuid.Nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, trackingErrors.TenantRequired))
	}
	return tenantID, userID, nil
}

// RevalidateInput is the request of a read the client revalidates with the ETag it holds.
type RevalidateInput struct {
	IfNoneMatch string `header:"If-None-Match" doc:"The ETag the client holds"`
}

// Today is the driver's trips of each route's local today, any status, each with its stops, tasks and
// riders — what the phone keeps offline. ETag is the hash of the day; sending it back as
// If-None-Match answers 304 while nothing changed.
func (ctrl *DriverController) Today(ctx context.Context, in *RevalidateInput) (*api.Cached[*models.DriverToday], error) {
	c := api.Gin(ctx)
	tenantID, userID, err := driverIDs(c)
	if err != nil {
		return nil, err
	}

	day, etag, err := ctrl.driverService.Today(c.Request.Context(), tenantID, userID)
	if err != nil {
		return nil, tripErrorResponse(c, err)
	}
	return api.Revalidate(in.IfNoneMatch, etag, "private, no-cache", day)
}

// StartTripInput is StartTrip's request.
type StartTripInput struct {
	TripID string `path:"trip_id"`
}

// StartTrip is POST /trips/{trip_id}/start: Start one of the driver's trips.
func (ctrl *DriverController) StartTrip(ctx context.Context, _ *StartTripInput) (*api.Out[*models.TripDetail], error) {
	c := api.Gin(ctx)
	tenantID, userID, err := driverIDs(c)
	if err != nil {
		return nil, err
	}
	tripID, err := pathUUID(c, "trip_id")
	if err != nil {
		return nil, err
	}

	detail, err := ctrl.driverService.StartTrip(c.Request.Context(), tenantID, userID, tripID)
	if err != nil {
		return nil, tripErrorResponse(c, err)
	}
	return api.OK(detail)
}

// SyncInput is Sync's request.
type SyncInput struct {
	JSON api.Body[models.DriverSyncDto]
}

// Sync is POST /sync: Replay what the phone did offline.
func (ctrl *DriverController) Sync(ctx context.Context, in *SyncInput) (*api.Out[models.DriverSyncResponse], error) {
	c := api.Gin(ctx)
	tenantID, userID, err := driverIDs(c)
	if err != nil {
		return nil, err
	}

	var dto models.DriverSyncDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.DriverSyncInvalid, utils.ExtractValidationError(c, err)))
	}
	if detail := checkOps(dto.Ops); detail != "" {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.DriverSyncInvalid, detail))
	}

	results, err := ctrl.driverService.Sync(c.Request.Context(), tenantID, userID, dto.Ops)
	if err != nil {
		return nil, tripErrorResponse(c, err)
	}
	return api.OK(models.DriverSyncResponse{Results: results})
}

// checkOps is what binding cannot say about an op: that its args name the target its op needs. It
// answers the first problem, or "".
func checkOps(ops []models.DriverSyncOp) string {
	for i, op := range ops {
		var target *uuid.UUID
		switch op.Op {
		case models.DriverOpStopArrive, models.DriverOpStopSkip:
			target = op.Args.TripStopID
		case models.DriverOpTaskDone, models.DriverOpTaskNoShow, models.DriverOpTaskReset:
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

// ResolveRiderInput is ResolveRider's request.
type ResolveRiderInput struct {
	Code string `path:"code"`
}

// ResolveRider is GET /riders/{code}: Resolve a scanned rider card.
func (ctrl *DriverController) ResolveRider(ctx context.Context, in *ResolveRiderInput) (*api.Out[*models.DriverRider], error) {
	c := api.Gin(ctx)
	tenantID, userID, err := driverIDs(c)
	if err != nil {
		return nil, err
	}

	rider, err := ctrl.driverService.ResolveRider(c.Request.Context(), tenantID, userID, in.Code)
	if err != nil {
		return nil, tripErrorResponse(c, err)
	}
	return api.OK(rider)
}

// FindRidersInput is FindRiders's request.
type FindRidersInput struct {
	Query api.Query[models.DriverFindRidersQuery]
}

// FindRiders is GET /riders: Find a rider without their card.
func (ctrl *DriverController) FindRiders(ctx context.Context, in *FindRidersInput) (*api.Out[models.DriverFindRidersResponse], error) {
	c := api.Gin(ctx)
	tenantID, userID, err := driverIDs(c)
	if err != nil {
		return nil, err
	}
	var query models.DriverFindRidersQuery
	if err := in.Query.Into(ctx, &query); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.DriverRiderInvalid, utils.ExtractValidationError(c, err)))
	}

	riders, err := ctrl.driverService.FindRiders(c.Request.Context(), tenantID, userID, query.Q)
	if err != nil {
		return nil, tripErrorResponse(c, err)
	}
	return api.OK(models.DriverFindRidersResponse{Riders: riders})
}

// CreateProofInput is CreateProof's request.
type CreateProofInput struct {
	JSON api.Body[models.DriverProofDto]
}

// CreateProof is POST /proofs: Sign a drop-off photo's upload.
func (ctrl *DriverController) CreateProof(ctx context.Context, in *CreateProofInput) (*api.Out[*models.DriverProofTicket], error) {
	c := api.Gin(ctx)
	tenantID, _, err := driverIDs(c)
	if err != nil {
		return nil, err
	}
	var dto models.DriverProofDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.DriverProofInvalid, utils.ExtractValidationError(c, err)))
	}

	ticket, err := ctrl.driverService.ProofTicket(c.Request.Context(), tenantID, dto.Size)
	if err != nil {
		return nil, tripErrorResponse(c, err)
	}
	return api.OK(ticket)
}

// ReportIncidentInput is ReportIncident's request.
type ReportIncidentInput struct {
	JSON api.Body[models.DriverIncidentDto]
}

// ReportIncident is POST /incidents: Report an incident on one of the driver's trips.
func (ctrl *DriverController) ReportIncident(ctx context.Context, in *ReportIncidentInput) (*api.Out[models.DriverIncidentResponse], error) {
	c := api.Gin(ctx)
	tenantID, userID, err := driverIDs(c)
	if err != nil {
		return nil, err
	}
	var dto models.DriverIncidentDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.DriverIncidentInvalid, utils.ExtractValidationError(c, err)))
	}

	alert, err := ctrl.driverService.ReportIncident(c.Request.Context(), tenantID, userID, &dto)
	if err != nil {
		return nil, tripErrorResponse(c, err)
	}
	return api.OK(models.DriverIncidentResponse{Alert: *alert})
}
