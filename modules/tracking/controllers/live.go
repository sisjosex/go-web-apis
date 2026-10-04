package controllers

import (
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

// LiveController serves the live snapshots (TRACK-025): what a live screen reads when it opens, and
// again whenever its socket reconnects or skips a seq.
type LiveController struct {
	live *services.LiveService
}

func NewLiveController(live *services.LiveService) *LiveController {
	return &LiveController{live: live}
}

// GetLiveFleet godoc
// @Summary Every reporting vehicle, where it is now
// @Description Each vehicle's last position with its trip in progress and route; organization_id keeps the vehicles whose trip carries one of its riders
// @Tags Tracking - Live
// @Produce json
// @Security BearerAuth
// @Param organization_id query string false "Organization ID (UUID)"
// @Success 200 {object} models.LiveFleetResponse
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 403 {object} coreErrors.ErrorResponse
// @Router /tracking/live/fleet [get]
func (ctrl *LiveController) GetLiveFleet(c *gin.Context) {
	tenantID, err := uuid.Parse(c.GetString("tenant_id"))
	if err != nil {
		c.JSON(http.StatusForbidden, coreErrors.BuildErrorSingle(c, trackingErrors.TenantRequired))
		return
	}
	var query models.LiveFleetQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.TripListFailed, utils.ExtractValidationError(c, err)))
		return
	}
	vehicles, err := ctrl.live.Fleet(c.Request.Context(), tenantID, query.OrganizationID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.JSON(http.StatusOK, models.LiveFleetResponse{Vehicles: vehicles})
}

// orNotFound is err, or a trip not found when there is none: a trip the user may not see is answered
// as one that does not exist.
func orNotFound(err error) error {
	if err != nil {
		return err
	}
	return &trackingErrors.TrackingError{Code: trackingErrors.TripNotFound}
}

// GetTripLive godoc
// @Summary A trip now: detail, vehicle position and ETA per pending stop
// @Description The ETA is computed on demand and cached 30 s per trip; estimate_source says whether it is valhalla, historical or fallback
// @Tags Tracking - Live
// @Produce json
// @Security BearerAuth
// @Param trip_id path string true "Trip ID (UUID)"
// @Success 200 {object} models.TripLive
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Router /tracking/trips/{trip_id}/live [get]
func (ctrl *LiveController) GetTripLive(c *gin.Context) {
	tenantID, err := uuid.Parse(c.GetString("tenant_id"))
	if err != nil {
		c.JSON(http.StatusForbidden, coreErrors.BuildErrorSingle(c, trackingErrors.TenantRequired))
		return
	}
	tripID, ok := pathUUID(c, "trip_id")
	if !ok {
		return
	}
	// An organization user sees only a trip carrying a passenger of theirs (MOBILE-020 D2): one
	// indexed check for that level, none for the operator.
	if userID := userIDAtLevel(c, tenancyModels.RoleOrganization); userID != nil {
		visible, err := ctrl.live.TripVisibleToUser(c.Request.Context(), tenantID, tripID, *userID)
		if err != nil || !visible {
			tripErrorResponse(c, orNotFound(err))
			return
		}
	}
	live, _, err := ctrl.live.Trip(c.Request.Context(), tenantID, tripID)
	if err != nil {
		tripErrorResponse(c, err)
		return
	}
	c.JSON(http.StatusOK, live)
}

// GetRiderLive godoc
// @Summary A rider's trip now: the rider's stops, the vehicle's position and the ETA to them
// @Description The guardian's map (MOBILE-010). Scoped as the rider's status; no trip in progress → trip_id null, no stops, no position
// @Tags Tracking - Live
// @Produce json
// @Security BearerAuth
// @Param rider_id path string true "Rider ID (UUID)"
// @Success 200 {object} models.RiderLive
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 403 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Router /tracking/riders/{rider_id}/live [get]
func (ctrl *LiveController) GetRiderLive(c *gin.Context) {
	tenantID, err := uuid.Parse(c.GetString("tenant_id"))
	if err != nil {
		c.JSON(http.StatusForbidden, coreErrors.BuildErrorSingle(c, trackingErrors.TenantRequired))
		return
	}
	riderID, ok := pathUUID(c, "rider_id")
	if !ok {
		return
	}
	live, _, err := ctrl.live.Rider(c.Request.Context(), tenantID, riderID,
		userIDAtLevel(c, tenancyModels.RoleOrganization), userIDAtLevel(c, tenancyModels.RolePortal))
	if err != nil {
		if !scopeRefused(c, err) {
			tripErrorResponse(c, err)
		}
		return
	}
	c.JSON(http.StatusOK, live)
}
