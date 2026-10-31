package controllers

import (
	"context"
	"errors"
	"net/http"

	"josex/web/modules/core/api"
	coreErrors "josex/web/modules/core/errors"
	"josex/web/modules/core/utils"
	trackingErrors "josex/web/modules/tracking/errors"
	"josex/web/modules/tracking/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// The trip handlers of TRACK-020, beside tracking_controller.go: one slice, one file.

// tripStatus is the status each trip code answers; a code not listed is a 500.
var tripStatus = map[string]int{
	trackingErrors.TripNotFound:            http.StatusNotFound,
	trackingErrors.TripStopNotFound:        http.StatusNotFound,
	trackingErrors.TripTaskNotFound:        http.StatusNotFound,
	trackingErrors.TripInvalidTransition:   http.StatusConflict,
	trackingErrors.TripInvalidRange:        http.StatusBadRequest,
	trackingErrors.TripTaskTripNotActive:   http.StatusConflict,
	trackingErrors.TripTaskOpConflict:      http.StatusConflict,
	trackingErrors.DriverProofInvalid:      http.StatusBadRequest,
	trackingErrors.RouteCompanyMismatch:    http.StatusBadRequest,
	trackingErrors.ServiceBlocked:          http.StatusConflict,
	trackingErrors.TripServiceBlocked:      http.StatusConflict,
	trackingErrors.DriverNotLinked:         http.StatusForbidden,
	trackingErrors.RiderNotFound:           http.StatusNotFound,
	trackingErrors.OrganizationScopeDenied: http.StatusForbidden,
}

// The actions each path's last segment may name, as the SPs spell them.
var (
	tripActions     = map[string]string{"start": "start", "complete": "complete", "cancel": "cancel"}
	tripStopActions = map[string]string{"arrive": "arrive", "skip": "skip"}
	tripTaskActions = map[string]string{"done": "done", "no-show": "no_show"}
)

// tripErrorResponse answers the trip codes with their status and anything else with a 500. company-mismatch and service.blocked carry the offending field as their detail.
func tripErrorResponse(c *gin.Context, err error) error {
	var trackingErr *trackingErrors.TrackingError
	if errors.As(err, &trackingErr) {
		if status, ok := tripStatus[trackingErr.Code]; ok {
			if trackingErr.Detail != nil {
				return api.Fail(status, coreErrors.BuildErrorDetail(c, trackingErr.Code, trackingErr.Detail))
			} else {
				return api.Fail(status, coreErrors.BuildErrorSingle(c, trackingErr.Code))
			}

		}
	}
	return api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
}

// pathUUID parses a path id, answering 400 when it is not one.
func pathUUID(c *gin.Context, name string) (uuid.UUID, error) {
	id, err := uuid.Parse(c.Param(name))
	if err != nil {
		return uuid.Nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
	}
	return id, nil
}

// pathAction resolves the :action segment against one of the maps above; an action not listed is
// a path that does not exist, answered as gin answers any unknown path.
func pathAction(c *gin.Context, actions map[string]string) (string, error) {
	action, ok := actions[c.Param("action")]
	if !ok {
		return "", api.Fail(http.StatusNotFound, nil)
	}
	return action, nil
}

// ListTripsInput is ListTrips's request.
type ListTripsInput struct {
	Query api.Query[models.ListTripsQuery]
}

// ListTrips is GET /trips: List a day's trips.
func (ctrl *TrackingController) ListTrips(ctx context.Context, in *ListTripsInput) (*api.Out[*models.ListTripsResponse], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}
	var query models.ListTripsQuery
	if err := in.Query.Into(ctx, &query); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.TripListFailed, utils.ExtractValidationError(c, err)))
	}

	query.ScopeUserID = ctrl.scopeUserID(c)
	result, err := ctrl.trackingService.ListTrips(c.Request.Context(), tenantID, query)
	if err != nil {
		return nil, tripErrorResponse(c, err)
	}
	return api.OK(result)
}

// GetTripInput is GetTrip's request.
type GetTripInput struct {
	TripID string `path:"trip_id"`
}

// GetTrip is GET /trips/{trip_id}: Get a trip with its stops and tasks.
func (ctrl *TrackingController) GetTrip(ctx context.Context, _ *GetTripInput) (*api.Out[*models.TripDetail], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}
	tripID, err := pathUUID(c, "trip_id")
	if err != nil {
		return nil, err
	}

	result, err := ctrl.trackingService.GetTrip(c.Request.Context(), tenantID, tripID)
	if err != nil {
		return nil, tripErrorResponse(c, err)
	}
	return api.OK(result)
}

// GetTripTaskProofInput is GetTripTaskProof's request.
type GetTripTaskProofInput struct {
	TaskID string `path:"task_id"`
}

// GetTripTaskProof is GET /trip-stop-tasks/{task_id}/proof: Link to a drop-off's proof photo.
func (ctrl *TrackingController) GetTripTaskProof(ctx context.Context, _ *GetTripTaskProofInput) (*api.Out[*models.TripTaskProof], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}
	taskID, err := pathUUID(c, "task_id")
	if err != nil {
		return nil, err
	}

	// ctx is the request's context
	key, err := ctrl.trackingService.TripTaskProofKey(ctx, tenantID, taskID, ctrl.scopeUserID(c), ctrl.guardianUserID(c))
	if err != nil {
		return nil, tripErrorResponse(c, err)
	}
	proof, err := ctrl.proofFiles.URL(ctx, key)
	if err != nil {
		return nil, tripErrorResponse(c, err)
	}
	return api.OK(proof)
}

// GetTripStatusInput is GetTripStatus's request.
type GetTripStatusInput struct {
	TripID string `path:"trip_id"`
}

// GetTripStatus is GET /trips/{trip_id}/status: Get a trip's live status.
func (ctrl *TrackingController) GetTripStatus(ctx context.Context, _ *GetTripStatusInput) (*api.Out[*models.TripStatus], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}
	tripID, err := pathUUID(c, "trip_id")
	if err != nil {
		return nil, err
	}

	result, err := ctrl.trackingService.GetTripStatus(c.Request.Context(), tenantID, tripID)
	if err != nil {
		return nil, tripErrorResponse(c, err)
	}
	return api.OK(result)
}

// UpdateTripInput is UpdateTrip's request.
type UpdateTripInput struct {
	TripID string `path:"trip_id"`
	JSON   api.Body[models.UpdateTripDto]
}

// UpdateTrip is PATCH /trips/{trip_id}: Override a trip.
func (ctrl *TrackingController) UpdateTrip(ctx context.Context, in *UpdateTripInput) (*api.Out[*models.TripDetail], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}
	tripID, err := pathUUID(c, "trip_id")
	if err != nil {
		return nil, err
	}
	var dto models.UpdateTripDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.TripUpdateFailed, utils.ExtractValidationError(c, err)))
	}

	result, err := ctrl.trackingService.UpdateTrip(c.Request.Context(), tenantID, tripID, &dto, ctrl.actingUserID(c))
	if err != nil {
		return nil, tripErrorResponse(c, err)
	}
	return api.OK(result)
}

// TransitionTripInput is TransitionTrip's request.
type TransitionTripInput struct {
	Action string `path:"action"`
	TripID string `path:"trip_id"`
}

// TransitionTrip is POST /trips/{trip_id}/{action}: Start, complete or cancel a trip.
func (ctrl *TrackingController) TransitionTrip(ctx context.Context, _ *TransitionTripInput) (*api.Out[*models.TripDetail], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}
	tripID, err := pathUUID(c, "trip_id")
	if err != nil {
		return nil, err
	}
	action, err := pathAction(c, tripActions)
	if err != nil {
		return nil, err
	}

	result, err := ctrl.trackingService.TransitionTrip(c.Request.Context(), tenantID, tripID, action, ctrl.actingUserID(c))
	if err != nil {
		return nil, tripErrorResponse(c, err)
	}
	return api.OK(result)
}

// TransitionTripStopInput is TransitionTripStop's request.
type TransitionTripStopInput struct {
	Action string `path:"action"`
	StopID string `path:"stop_id"`
}

// TransitionTripStop is POST /trip-stops/{stop_id}/{action}: Arrive at or skip a stop.
func (ctrl *TrackingController) TransitionTripStop(ctx context.Context, _ *TransitionTripStopInput) (*api.Out[*models.TripDetail], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}
	stopID, err := pathUUID(c, "stop_id")
	if err != nil {
		return nil, err
	}
	action, err := pathAction(c, tripStopActions)
	if err != nil {
		return nil, err
	}

	result, err := ctrl.trackingService.TransitionTripStop(c.Request.Context(), tenantID, stopID, action, ctrl.actingUserID(c))
	if err != nil {
		return nil, tripErrorResponse(c, err)
	}
	return api.OK(result)
}

// TransitionTripTaskInput is TransitionTripTask's request.
type TransitionTripTaskInput struct {
	Action string `path:"action"`
	TaskID string `path:"task_id"`
	JSON   api.Body[models.TripTaskTransitionDto]
}

// TransitionTripTask is POST /trip-stop-tasks/{task_id}/{action}: Mark a pickup or dropoff done, or the rider a no-show.
func (ctrl *TrackingController) TransitionTripTask(ctx context.Context, in *TransitionTripTaskInput) (*api.Out[*models.TripDetail], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}
	taskID, err := pathUUID(c, "task_id")
	if err != nil {
		return nil, err
	}
	action, err := pathAction(c, tripTaskActions)
	if err != nil {
		return nil, err
	}
	var dto models.TripTaskTransitionDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.TripTransitionFailed, utils.ExtractValidationError(c, err)))
	}

	result, err := ctrl.trackingService.TransitionTripTask(c.Request.Context(), tenantID, taskID, action, &dto, ctrl.actingUserID(c))
	if err != nil {
		return nil, tripErrorResponse(c, err)
	}
	return api.OK(result)
}

// TrackingOverviewInput is TrackingOverview's request.
type TrackingOverviewInput struct {
	Query api.Query[models.TrackingOverviewQuery]
}

// TrackingOverview is GET /overview: The tracking home.
func (ctrl *TrackingController) TrackingOverview(ctx context.Context, in *TrackingOverviewInput) (*api.Out[*models.TrackingOverview], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}
	var query models.TrackingOverviewQuery
	if err := in.Query.Into(ctx, &query); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.TripListFailed, utils.ExtractValidationError(c, err)))
	}
	overview, err := ctrl.trackingService.TrackingOverview(c.Request.Context(), tenantID, query.Date, ctrl.scopeUserID(c))
	if err != nil {
		if err := scopeRefused(c, err); err != nil {
			return nil, err
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.OK(overview)
}

// OrganizationAbsentInput is OrganizationAbsent's request.
type OrganizationAbsentInput struct {
	Query api.Query[models.OrganizationAbsentQuery]
}

// OrganizationAbsent is GET /organization/absent: Who a school's or company's day counts as absent.
func (ctrl *TrackingController) OrganizationAbsent(ctx context.Context, in *OrganizationAbsentInput) (*api.Out[*models.OrganizationAbsent], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}
	var query models.OrganizationAbsentQuery
	if err := in.Query.Into(ctx, &query); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.OrganizationTodayFailed, utils.ExtractValidationError(c, err)))
	}
	absent, err := ctrl.trackingService.OrganizationAbsent(c.Request.Context(), tenantID, ctrl.scopeUserID(c), query)
	if err != nil {
		if err := scopeRefused(c, err); err != nil {
			return nil, err
		}
		var trackingErr *trackingErrors.TrackingError
		if errors.As(err, &trackingErr) {
			switch trackingErr.Code {
			case trackingErrors.OrganizationRequired:
				return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, trackingErr.Code))
			case trackingErrors.OrganizationNotFound:
				return nil, api.Fail(http.StatusNotFound, coreErrors.BuildErrorSingle(c, trackingErr.Code))
			}
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.OK(absent)
}

// OrganizationTodayInput is OrganizationToday's request.
type OrganizationTodayInput struct {
	Query api.Query[models.OrganizationTodayQuery]
}

// OrganizationToday is GET /organization/today: A school's or company's day.
func (ctrl *TrackingController) OrganizationToday(ctx context.Context, in *OrganizationTodayInput) (*api.Out[*models.OrganizationToday], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}
	var query models.OrganizationTodayQuery
	if err := in.Query.Into(ctx, &query); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.OrganizationTodayFailed, utils.ExtractValidationError(c, err)))
	}
	day, err := ctrl.trackingService.OrganizationToday(c.Request.Context(), tenantID, ctrl.scopeUserID(c), query)
	if err != nil {
		if err := scopeRefused(c, err); err != nil {
			return nil, err
		}
		var trackingErr *trackingErrors.TrackingError
		if errors.As(err, &trackingErr) {
			switch trackingErr.Code {
			case trackingErrors.OrganizationRequired:
				return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, trackingErr.Code))
			case trackingErrors.OrganizationNotFound:
				return nil, api.Fail(http.StatusNotFound, coreErrors.BuildErrorSingle(c, trackingErr.Code))
			}
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.OK(day)
}

// NavCounts is GET /nav-counts: The taypi go rail's badges.
func (ctrl *TrackingController) NavCounts(ctx context.Context, _ *api.None) (*api.Out[*models.NavCounts], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}
	counts, err := ctrl.trackingService.NavCounts(c.Request.Context(), tenantID, ctrl.scopeUserID(c))
	if err != nil {
		if err := scopeRefused(c, err); err != nil {
			return nil, err
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.OK(counts)
}

// TrackingSetup is GET /setup: The tracking home's setup assistant.
func (ctrl *TrackingController) TrackingSetup(ctx context.Context, _ *api.None) (*api.Out[*models.TrackingSetup], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}
	setup, err := ctrl.trackingService.TrackingSetup(c.Request.Context(), tenantID)
	if err != nil {
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.OK(setup)
}
