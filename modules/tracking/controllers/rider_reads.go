package controllers

import (
	"errors"
	"net/http"

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

func riderReadErrorResponse(c *gin.Context, err error) {
	if scopeRefused(c, err) {
		return
	}
	var trackingErr *trackingErrors.TrackingError
	if errors.As(err, &trackingErr) {
		if status, ok := riderReadStatus[trackingErr.Code]; ok {
			c.JSON(status, coreErrors.BuildErrorSingle(c, trackingErr.Code))
			return
		}
	}
	c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
}

// riderReadIDs are the tenant and the :rider_id of a read, or false once the refusal is written.
func riderReadIDs(c *gin.Context) (uuid.UUID, uuid.UUID, bool) {
	tenantID, err := uuid.Parse(c.GetString("tenant_id"))
	if err != nil {
		c.JSON(http.StatusForbidden, coreErrors.BuildErrorSingle(c, trackingErrors.TenantRequired))
		return uuid.Nil, uuid.Nil, false
	}
	riderID, ok := pathUUID(c, "rider_id")
	return tenantID, riderID, ok
}

func riderDaysQuery(c *gin.Context) (models.RiderDaysQuery, bool) {
	var query models.RiderDaysQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.RiderReadFailed, utils.ExtractValidationError(c, err)))
		return query, false
	}
	return query, true
}

// RiderToday godoc
// @Summary The rider's day
// @Description Every trip of each route's local today carrying the rider, in order, each with the pickup and dropoff — planned, arrived, task status — and the ETA on the trip in progress while the rider rides it; today's absence and the carrier's phone. ETag is the hash of the body; sent back as If-None-Match it answers 304
// @Tags Tracking - Riders
// @Produce json
// @Security BearerAuth
// @Param rider_id path string true "Rider ID (UUID)"
// @Param If-None-Match header string false "ETag of the day the phone holds"
// @Success 200 {object} models.RiderToday
// @Success 304
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 403 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Router /tracking/riders/{rider_id}/today [get]
func (ctrl *RiderReadsController) RiderToday(c *gin.Context) {
	tenantID, riderID, ok := riderReadIDs(c)
	if !ok {
		return
	}
	body, etag, err := ctrl.reads.Today(c.Request.Context(), tenantID, riderID,
		userIDAtLevel(c, tenancyModels.RoleOrganization), userIDAtLevel(c, tenancyModels.RolePortal))
	if err != nil {
		riderReadErrorResponse(c, err)
		return
	}
	c.Header("ETag", etag)
	c.Header("Cache-Control", "private, no-cache")
	if c.GetHeader("If-None-Match") == etag {
		c.Status(http.StatusNotModified)
		return
	}
	c.Data(http.StatusOK, "application/json; charset=utf-8", body)
}

// RiderHistory godoc
// @Summary The rider's past days
// @Description One row per day, newest first, with a status (rode, partial, no_show, absent, cancelled), a leg per trip with pickup and dropoff times, and the absence; days with neither are left out. Default the 30 days up to the rider's local yesterday, at most 31
// @Tags Tracking - Riders
// @Produce json
// @Security BearerAuth
// @Param rider_id path string true "Rider ID (UUID)"
// @Param from query string false "From (YYYY-MM-DD)"
// @Param to query string false "To (YYYY-MM-DD)"
// @Success 200 {object} models.RiderHistory
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 403 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Router /tracking/riders/{rider_id}/history [get]
func (ctrl *RiderReadsController) RiderHistory(c *gin.Context) {
	tenantID, riderID, ok := riderReadIDs(c)
	if !ok {
		return
	}
	query, ok := riderDaysQuery(c)
	if !ok {
		return
	}
	history, err := ctrl.reads.History(c.Request.Context(), tenantID, riderID, query,
		userIDAtLevel(c, tenancyModels.RoleOrganization), userIDAtLevel(c, tenancyModels.RolePortal))
	if err != nil {
		riderReadErrorResponse(c, err)
		return
	}
	c.JSON(http.StatusOK, history)
}

// RiderAbsenceCutoffs godoc
// @Summary Until when each day's absence can be reported
// @Description Per day: whether the rider has a pickup, the guardian's cut-off before it and whether it has passed — the rule POST /absences answers 409 cutoff on. Default today and the next 13 days, at most 31
// @Tags Tracking - Riders
// @Produce json
// @Security BearerAuth
// @Param rider_id path string true "Rider ID (UUID)"
// @Param from query string false "From (YYYY-MM-DD)"
// @Param to query string false "To (YYYY-MM-DD)"
// @Success 200 {object} models.RiderAbsenceCutoffs
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 403 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Router /tracking/riders/{rider_id}/absence-cutoffs [get]
func (ctrl *RiderReadsController) RiderAbsenceCutoffs(c *gin.Context) {
	tenantID, riderID, ok := riderReadIDs(c)
	if !ok {
		return
	}
	query, ok := riderDaysQuery(c)
	if !ok {
		return
	}
	cutoffs, err := ctrl.reads.AbsenceCutoffs(c.Request.Context(), tenantID, riderID, query,
		userIDAtLevel(c, tenancyModels.RoleOrganization), userIDAtLevel(c, tenancyModels.RolePortal))
	if err != nil {
		riderReadErrorResponse(c, err)
		return
	}
	c.JSON(http.StatusOK, cutoffs)
}
