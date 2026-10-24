package controllers

import (
	"context"
	"net/http"
	"time"

	"josex/web/modules/core/api"
	coreErrors "josex/web/modules/core/errors"
	"josex/web/modules/core/utils"
	tenancyModels "josex/web/modules/tenancy/models"
	trackingErrors "josex/web/modules/tracking/errors"
	"josex/web/modules/tracking/models"
	"josex/web/modules/tracking/services"

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

// GetLiveFleetInput is GetLiveFleet's request.
type GetLiveFleetInput struct {
	Query api.Query[models.LiveFleetQuery]
}

// GetLiveFleet is GET /live/fleet: Every reporting vehicle, where it is now.
func (ctrl *LiveController) GetLiveFleet(ctx context.Context, in *GetLiveFleetInput) (*api.Out[models.LiveFleetResponse], error) {
	c := api.Gin(ctx)
	tenantID, err := uuid.Parse(c.GetString("tenant_id"))
	if err != nil {
		return nil, api.Fail(http.StatusForbidden, coreErrors.BuildErrorSingle(c, trackingErrors.TenantRequired))
	}
	var query models.LiveFleetQuery
	if err := in.Query.Into(ctx, &query); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.TripListFailed, utils.ExtractValidationError(c, err)))
	}
	vehicles, err := ctrl.live.Fleet(c.Request.Context(), tenantID, query.OrganizationID, userIDAtLevel(c, tenancyModels.RoleOrganization))
	if err != nil {
		if err := scopeRefused(c, err); err != nil {
			return nil, err
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.OK(models.LiveFleetResponse{Vehicles: vehicles})
}

// orNotFound is err, or a trip not found when there is none: a trip the user may not see is answered
// as one that does not exist.
func orNotFound(err error) error {
	if err != nil {
		return err
	}
	return &trackingErrors.TrackingError{Code: trackingErrors.TripNotFound}
}

// GetTripLiveInput is GetTripLive's request.
type GetTripLiveInput struct {
	TripID     string `path:"trip_id"`
	TrailSince string `query:"trail_since"`
	PathEtag   string `query:"path_etag"`
}

// GetTripLive is GET /trips/{trip_id}/live: A trip now: detail, vehicle position and ETA per pending stop.
func (ctrl *LiveController) GetTripLive(ctx context.Context, in *GetTripLiveInput) (*api.Out[*models.TripLive], error) {
	c := api.Gin(ctx)
	tenantID, err := uuid.Parse(c.GetString("tenant_id"))
	if err != nil {
		return nil, api.Fail(http.StatusForbidden, coreErrors.BuildErrorSingle(c, trackingErrors.TenantRequired))
	}
	tripID, err := pathUUID(c, "trip_id")
	if err != nil {
		return nil, err
	}
	// An organization user sees only a trip carrying a passenger of theirs (MOBILE-020 D2): one
	// indexed check for that level, none for the operator.
	if userID := userIDAtLevel(c, tenancyModels.RoleOrganization); userID != nil {
		visible, err := ctrl.live.TripVisibleToUser(c.Request.Context(), tenantID, tripID, *userID)
		if err != nil || !visible {
			return nil, tripErrorResponse(c, orNotFound(err))
		}
	}
	// A cursor that does not parse is no cursor: the whole trail comes back, which is never wrong.
	var trailSince *time.Time
	if since, err := time.Parse(time.RFC3339Nano, in.TrailSince); err == nil {
		trailSince = &since
	}
	live, _, err := ctrl.live.Trip(c.Request.Context(), tenantID, tripID, trailSince)
	if err != nil {
		return nil, tripErrorResponse(c, err)
	}
	live.DropKnownPath(in.PathEtag)
	return api.OK(live)
}

// GetRiderLiveInput is GetRiderLive's request.
type GetRiderLiveInput struct {
	RiderID  string `path:"rider_id"`
	PathEtag string `query:"path_etag" doc:"The planned_path.etag the caller holds: the line is then left out"`
}

// GetRiderLive is a rider's trip now: the rider's stops, the vehicle's position and the ETA to them —
// the guardian's map (MOBILE-010). Scoped as the rider's status; no trip in progress → trip_id null,
// no stops, no position.
func (ctrl *LiveController) GetRiderLive(ctx context.Context, in *GetRiderLiveInput) (*api.Out[*models.RiderLive], error) {
	c := api.Gin(ctx)
	tenantID, err := uuid.Parse(c.GetString("tenant_id"))
	if err != nil {
		return nil, api.Fail(http.StatusForbidden, coreErrors.BuildErrorSingle(c, trackingErrors.TenantRequired))
	}
	riderID, err := pathUUID(c, "rider_id")
	if err != nil {
		return nil, err
	}
	live, _, err := ctrl.live.Rider(c.Request.Context(), tenantID, riderID,
		userIDAtLevel(c, tenancyModels.RoleOrganization), userIDAtLevel(c, tenancyModels.RolePortal))
	if err != nil {
		if err := scopeRefused(c, err); err != nil {
			return nil, err
		}
		return nil, tripErrorResponse(c, err)
	}
	live.DropKnownPath(in.PathEtag)
	return api.OK(live)
}
