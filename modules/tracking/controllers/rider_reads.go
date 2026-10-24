package controllers

import (
	"context"
	"errors"
	"net/http"

	"josex/web/modules/core/api"
	coreErrors "josex/web/modules/core/errors"
	"josex/web/modules/core/utils"
	tenancyModels "josex/web/modules/tenancy/models"
	trackingErrors "josex/web/modules/tracking/errors"
	"josex/web/modules/tracking/models"
	"josex/web/modules/tracking/services"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RiderReadsController is the guardian's rider reads (TRACK-061) on the portal-permissive chain:
// staff, an organization for its riders and a guardian for their own — the SPs resolve the scope.
type RiderReadsController struct {
	reads *services.RiderReadsService
}

func NewRiderReadsController(reads *services.RiderReadsService) *RiderReadsController {
	return &RiderReadsController{reads: reads}
}

var riderReadStatus = map[string]int{
	trackingErrors.RiderNotFound:     http.StatusNotFound,
	trackingErrors.RiderHistoryRange: http.StatusBadRequest,
	trackingErrors.RiderCutoffsRange: http.StatusBadRequest,
}

func riderReadErrorResponse(c *gin.Context, err error) error {
	if err := scopeRefused(c, err); err != nil {
		return err
	}
	var trackingErr *trackingErrors.TrackingError
	if errors.As(err, &trackingErr) {
		if status, ok := riderReadStatus[trackingErr.Code]; ok {
			return api.Fail(status, coreErrors.BuildErrorSingle(c, trackingErr.Code))
		}
	}
	return api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
}

// riderReadIDs are the tenant and the :rider_id of a read, or false once the refusal is written.
func riderReadIDs(c *gin.Context) (uuid.UUID, uuid.UUID, error) {
	tenantID, err := uuid.Parse(c.GetString("tenant_id"))
	if err != nil {
		return uuid.Nil, uuid.Nil, api.Fail(http.StatusForbidden, coreErrors.BuildErrorSingle(c, trackingErrors.TenantRequired))
	}
	riderID, err := pathUUID(c, "rider_id")
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	return tenantID, riderID, nil
}

func riderDaysQuery(c *gin.Context) (models.RiderDaysQuery, error) {
	var query models.RiderDaysQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		return query, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.RiderReadFailed, utils.ExtractValidationError(c, err)))
	}
	return query, nil
}

// RiderTodayInput is RiderToday's request.
type RiderTodayInput struct {
	RiderID     string `path:"rider_id"`
	IfNoneMatch string `header:"If-None-Match" doc:"The ETag the client holds"`
}

// RiderToday is every trip of each route's local today carrying the rider, in order, each with the
// pickup and dropoff — planned, arrived, task status — and the ETA on the trip in progress while the
// rider rides it; today's absence and the carrier's phone. ETag is the hash of the body; sent back as
// If-None-Match it answers 304.
func (ctrl *RiderReadsController) RiderToday(ctx context.Context, in *RiderTodayInput) (*api.Cached[api.RawJSON[models.RiderToday]], error) {
	c := api.Gin(ctx)
	tenantID, riderID, err := riderReadIDs(c)
	if err != nil {
		return nil, err
	}
	body, etag, err := ctrl.reads.Today(c.Request.Context(), tenantID, riderID,
		userIDAtLevel(c, tenancyModels.RoleOrganization), userIDAtLevel(c, tenancyModels.RolePortal))
	if err != nil {
		return nil, riderReadErrorResponse(c, err)
	}
	return api.Revalidate(in.IfNoneMatch, etag, "private, no-cache", api.RawJSON[models.RiderToday](body))
}

// RiderHistoryInput is RiderHistory's request.
type RiderHistoryInput struct {
	RiderID string `path:"rider_id"`
}

// RiderHistory is GET /riders/{rider_id}/history: The rider's past days.
func (ctrl *RiderReadsController) RiderHistory(ctx context.Context, _ *RiderHistoryInput) (*api.Out[*models.RiderHistory], error) {
	c := api.Gin(ctx)
	tenantID, riderID, err := riderReadIDs(c)
	if err != nil {
		return nil, err
	}
	query, err := riderDaysQuery(c)
	if err != nil {
		return nil, err
	}
	history, err := ctrl.reads.History(c.Request.Context(), tenantID, riderID, query,
		userIDAtLevel(c, tenancyModels.RoleOrganization), userIDAtLevel(c, tenancyModels.RolePortal))
	if err != nil {
		return nil, riderReadErrorResponse(c, err)
	}
	return api.OK(history)
}

// RiderAbsenceCutoffsInput is RiderAbsenceCutoffs's request.
type RiderAbsenceCutoffsInput struct {
	RiderID string `path:"rider_id"`
}

// RiderAbsenceCutoffs is GET /riders/{rider_id}/absence-cutoffs: Until when each day's absence can be reported.
func (ctrl *RiderReadsController) RiderAbsenceCutoffs(ctx context.Context, _ *RiderAbsenceCutoffsInput) (*api.Out[*models.RiderAbsenceCutoffs], error) {
	c := api.Gin(ctx)
	tenantID, riderID, err := riderReadIDs(c)
	if err != nil {
		return nil, err
	}
	query, err := riderDaysQuery(c)
	if err != nil {
		return nil, err
	}
	cutoffs, err := ctrl.reads.AbsenceCutoffs(c.Request.Context(), tenantID, riderID, query,
		userIDAtLevel(c, tenancyModels.RoleOrganization), userIDAtLevel(c, tenancyModels.RolePortal))
	if err != nil {
		return nil, riderReadErrorResponse(c, err)
	}
	return api.OK(cutoffs)
}
