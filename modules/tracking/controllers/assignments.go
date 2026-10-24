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

// The rider route assignment handlers of TRACK-009, beside tracking_controller.go: one slice, one file.

// assignmentStatus is the status each assignment code answers; a code not listed is a 500.
var assignmentStatus = map[string]int{
	trackingErrors.AssignmentNotFound:       http.StatusNotFound,
	trackingErrors.RiderNotFound:            http.StatusNotFound,
	trackingErrors.RouteNotFound:            http.StatusNotFound,
	trackingErrors.AssignmentOverlap:        http.StatusConflict,
	trackingErrors.AssignmentStopNotOnRoute: http.StatusBadRequest,
	trackingErrors.AssignmentDaysOfWeek:     http.StatusBadRequest,
	trackingErrors.AssignmentRange:          http.StatusBadRequest,
	trackingErrors.RiderLocationRequired:    http.StatusUnprocessableEntity,
	trackingErrors.OrganizationScopeDenied:  http.StatusForbidden,
}

// assignmentErrorResponse maps the assignment codes to statuses and reports whether it has already
// written the response. A code carrying a Detail — the refused item of a bulk request — answers it.
func assignmentErrorResponse(c *gin.Context, err error) error {
	var trackingErr *trackingErrors.TrackingError
	if !errors.As(err, &trackingErr) {
		return nil
	}
	status, ok := assignmentStatus[trackingErr.Code]
	if !ok {
		return nil
	}
	if trackingErr.Detail != nil {
		return api.Fail(status, coreErrors.BuildErrorDetail(c, trackingErr.Code, trackingErr.Detail))
	} else {
		return api.Fail(status, coreErrors.BuildErrorSingle(c, trackingErr.Code))
	}

}

// ListAssignmentsInput is ListAssignments's request.
type ListAssignmentsInput struct {
	Query api.Query[models.ListAssignmentsQuery]
}

// ListAssignments is GET /assignments: List rider route assignments.
func (ctrl *TrackingController) ListAssignments(ctx context.Context, in *ListAssignmentsInput) (*api.Out[*models.ListAssignmentsResponse], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}
	var query models.ListAssignmentsQuery
	if err := in.Query.Into(ctx, &query); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.AssignmentListFailed, utils.ExtractValidationError(c, err)))
	}

	result, err := ctrl.trackingService.ListAssignments(c.Request.Context(), tenantID, query, ctrl.scopeUserID(c))
	if err != nil {
		if err := assignmentErrorResponse(c, err); err != nil {
			return nil, err
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.OK(result)
}

// CreateAssignmentInput is CreateAssignment's request.
type CreateAssignmentInput struct {
	JSON api.Body[models.AssignRiderDto]
}

// CreateAssignment is POST /assignments: Assign a rider to a route.
func (ctrl *TrackingController) CreateAssignment(ctx context.Context, in *CreateAssignmentInput) (*api.Out[*models.AssignmentResponse], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}
	var dto models.AssignRiderDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.AssignmentCreateFailed, utils.ExtractValidationError(c, err)))
	}

	result, err := ctrl.trackingService.CreateAssignment(c.Request.Context(), tenantID, &dto)
	if err != nil {
		if err := assignmentErrorResponse(c, err); err != nil {
			return nil, err
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.OK(result)
}

// CreateAssignmentsBulkInput is CreateAssignmentsBulk's request.
type CreateAssignmentsBulkInput struct {
	JSON api.Body[[]models.AssignRiderDto]
}

// CreateAssignmentsBulk is POST /assignments/bulk: Assign many riders at once.
func (ctrl *TrackingController) CreateAssignmentsBulk(ctx context.Context, in *CreateAssignmentsBulkInput) (*api.Out[*models.BulkAssignmentResponse], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}
	dtos := []models.AssignRiderDto{}
	if err := in.JSON.Into(ctx, &dtos); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.AssignmentCreateFailed, utils.ExtractValidationError(c, err)))
	}
	if len(dtos) == 0 {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, trackingErrors.AssignmentCreateFailed))
	}

	result, err := ctrl.trackingService.CreateAssignments(c.Request.Context(), tenantID, dtos)
	if err != nil {
		if err := assignmentErrorResponse(c, err); err != nil {
			return nil, err
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.OK(result)
}

// BulkAssignRidersInput is BulkAssignRiders's request.
type BulkAssignRidersInput struct {
	RouteID string `path:"route_id"`
	JSON    api.Body[models.BulkAssignRidersDto]
}

// BulkAssignRiders is POST /routes/{route_id}/assignments/bulk: Add many riders to a route.
func (ctrl *TrackingController) BulkAssignRiders(ctx context.Context, in *BulkAssignRidersInput) (*api.Out[*models.BulkAssignRidersResponse], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}
	routeID, err := uuid.Parse(in.RouteID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
	}
	var dto models.BulkAssignRidersDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.AssignmentCreateFailed, utils.ExtractValidationError(c, err)))
	}
	result, err := ctrl.trackingService.BulkAssignRiders(c.Request.Context(), tenantID, routeID, &dto)
	if err != nil {
		if err := assignmentErrorResponse(c, err); err != nil {
			return nil, err
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.OK(result)
}

// ListRiderGroupsInput is ListRiderGroups's request.
type ListRiderGroupsInput struct {
	Query api.Query[models.ListRiderGroupsQuery]
}

// ListRiderGroupsResponse is ListRiderGroups's answer.
type ListRiderGroupsResponse struct {
	Groups []string `json:"groups"`
}

// ListRiderGroups is GET /riders/groups: The groups riders use.
func (ctrl *TrackingController) ListRiderGroups(ctx context.Context, in *ListRiderGroupsInput) (*api.Out[ListRiderGroupsResponse], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}
	var query models.ListRiderGroupsQuery
	if err := in.Query.Into(ctx, &query); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.RiderListFailed, utils.ExtractValidationError(c, err)))
	}
	groups, err := ctrl.trackingService.ListRiderGroups(c.Request.Context(), tenantID, query.OrganizationID)
	if err != nil {
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.OK(ListRiderGroupsResponse{Groups: groups})
}

// UpdateAssignmentInput is UpdateAssignment's request.
type UpdateAssignmentInput struct {
	AssignmentID string `path:"assignment_id"`
	JSON         api.Body[models.UpdateAssignmentDto]
}

// UpdateAssignment is PATCH /assignments/{assignment_id}: Edit an assignment.
func (ctrl *TrackingController) UpdateAssignment(ctx context.Context, in *UpdateAssignmentInput) (*api.Out[*models.AssignmentResponse], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}
	assignmentID, err := uuid.Parse(in.AssignmentID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
	}
	var dto models.UpdateAssignmentDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.AssignmentUpdateFailed, utils.ExtractValidationError(c, err)))
	}

	result, err := ctrl.trackingService.UpdateAssignment(c.Request.Context(), tenantID, assignmentID, &dto)
	if err != nil {
		if err := assignmentErrorResponse(c, err); err != nil {
			return nil, err
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.OK(result)
}

// DeleteAssignmentInput is DeleteAssignment's request.
type DeleteAssignmentInput struct {
	AssignmentID string `path:"assignment_id"`
	Twin         string `query:"twin"`
}

// DeleteAssignment is DELETE /assignments/{assignment_id}: Remove an assignment.
func (ctrl *TrackingController) DeleteAssignment(ctx context.Context, in *DeleteAssignmentInput) (*api.Empty, error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}
	assignmentID, err := uuid.Parse(in.AssignmentID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
	}

	twin := in.Twin != "false"
	if err := ctrl.trackingService.DeleteAssignment(c.Request.Context(), tenantID, assignmentID, twin); err != nil {
		if err := assignmentErrorResponse(c, err); err != nil {
			return nil, err
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.Done()
}
