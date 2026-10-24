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
)

// The absence and suggestion handlers of TRACK-022: one slice, one file.

// absenceStatus is the status each absence code answers; a code not listed is a 500.
var absenceStatus = map[string]int{
	trackingErrors.RiderNotFound:       http.StatusNotFound,
	trackingErrors.AbsenceNotFound:     http.StatusNotFound,
	trackingErrors.AbsenceOverlap:      http.StatusConflict,
	trackingErrors.AbsenceCutoff:       http.StatusConflict,
	trackingErrors.AbsenceInvalidRange: http.StatusBadRequest,
	trackingErrors.RiderNoHomeLocation: http.StatusConflict,
}

func absenceErrorResponse(c *gin.Context, err error) error {
	if err := scopeRefused(c, err); err != nil {
		return err
	}
	var trackingErr *trackingErrors.TrackingError
	if errors.As(err, &trackingErr) {
		if status, ok := absenceStatus[trackingErr.Code]; ok {
			return api.Fail(status, coreErrors.BuildErrorSingle(c, trackingErr.Code))
		}
	}
	return api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
}

// ListRiderAbsencesInput is ListRiderAbsences's request.
type ListRiderAbsencesInput struct {
	RiderID string `path:"rider_id"`
	Query   api.Query[models.ListRiderAbsencesQuery]
}

// ListRiderAbsences is GET /riders/{rider_id}/absences: List a rider's absences.
func (ctrl *TrackingController) ListRiderAbsences(ctx context.Context, in *ListRiderAbsencesInput) (*api.Out[[]*models.RiderAbsence], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}
	riderID, err := pathUUID(c, "rider_id")
	if err != nil {
		return nil, err
	}
	var query models.ListRiderAbsencesQuery
	if err := in.Query.Into(ctx, &query); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.AbsenceListFailed, utils.ExtractValidationError(c, err)))
	}

	result, err := ctrl.trackingService.ListRiderAbsences(c.Request.Context(), tenantID, riderID, query, ctrl.scopeUserID(c), ctrl.guardianUserID(c))
	if err != nil {
		return nil, absenceErrorResponse(c, err)
	}
	return api.OK(result)
}

// CreateRiderAbsenceInput is CreateRiderAbsence's request.
type CreateRiderAbsenceInput struct {
	RiderID string `path:"rider_id"`
	JSON    api.Body[models.CreateRiderAbsenceDto]
}

// CreateRiderAbsence is POST /riders/{rider_id}/absences: Report a rider's absence.
func (ctrl *TrackingController) CreateRiderAbsence(ctx context.Context, in *CreateRiderAbsenceInput) (*api.Out[*models.CreateRiderAbsenceResponse], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}
	riderID, err := pathUUID(c, "rider_id")
	if err != nil {
		return nil, err
	}
	var dto models.CreateRiderAbsenceDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.AbsenceCreateFailed, utils.ExtractValidationError(c, err)))
	}

	result, err := ctrl.trackingService.CreateRiderAbsence(c.Request.Context(), tenantID, riderID, &dto,
		ctrl.actingUserID(c), ctrl.scopeUserID(c), ctrl.guardianUserID(c))
	if err != nil {
		return nil, absenceErrorResponse(c, err)
	}
	return api.OK(result)
}

// DeleteRiderAbsenceInput is DeleteRiderAbsence's request.
type DeleteRiderAbsenceInput struct {
	AbsenceID string `path:"absence_id"`
	RiderID   string `path:"rider_id"`
}

// DeleteRiderAbsence is DELETE /riders/{rider_id}/absences/{absence_id}: Delete a rider's absence.
func (ctrl *TrackingController) DeleteRiderAbsence(ctx context.Context, _ *DeleteRiderAbsenceInput) (*api.Empty, error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}
	riderID, err := pathUUID(c, "rider_id")
	if err != nil {
		return nil, err
	}
	absenceID, err := pathUUID(c, "absence_id")
	if err != nil {
		return nil, err
	}

	if err := ctrl.trackingService.DeleteRiderAbsence(c.Request.Context(), tenantID, riderID, absenceID,
		ctrl.scopeUserID(c), ctrl.guardianUserID(c)); err != nil {
		return nil, absenceErrorResponse(c, err)
	}
	return api.Done()
}

// SuggestRiderStopsInput is SuggestRiderStops's request.
type SuggestRiderStopsInput struct {
	RiderID string `path:"rider_id"`
	Query   api.Query[models.SuggestRiderStopsQuery]
}

// SuggestRiderStops is GET /riders/{rider_id}/suggestions: Suggest stops near a rider's home.
func (ctrl *TrackingController) SuggestRiderStops(ctx context.Context, in *SuggestRiderStopsInput) (*api.Out[*models.SuggestRiderStopsResponse], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}
	riderID, err := pathUUID(c, "rider_id")
	if err != nil {
		return nil, err
	}
	var query models.SuggestRiderStopsQuery
	if err := in.Query.Into(ctx, &query); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.RiderSuggestionsFailed, utils.ExtractValidationError(c, err)))
	}

	result, err := ctrl.trackingService.SuggestRiderStops(c.Request.Context(), tenantID, riderID, query)
	if err != nil {
		return nil, absenceErrorResponse(c, err)
	}
	return api.OK(result)
}
