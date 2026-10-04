package controllers

import (
	"errors"
	"net/http"

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
	trackingErrors.OrganizationScopeDenied:  http.StatusForbidden,
}

// assignmentErrorResponse maps the assignment codes to statuses and reports whether it has already
// written the response. A code carrying a Detail — the refused item of a bulk request — answers it.
func assignmentErrorResponse(c *gin.Context, err error) bool {
	var trackingErr *trackingErrors.TrackingError
	if !errors.As(err, &trackingErr) {
		return false
	}
	status, ok := assignmentStatus[trackingErr.Code]
	if !ok {
		return false
	}
	if trackingErr.Detail != nil {
		c.JSON(status, coreErrors.BuildErrorDetail(c, trackingErr.Code, trackingErr.Detail))
	} else {
		c.JSON(status, coreErrors.BuildErrorSingle(c, trackingErr.Code))
	}
	return true
}

// ListAssignments godoc
// @Summary List rider route assignments
// @Description Every row carries the rider, route and stop names and the route's direction; date narrows to the assignments in force that day
// @Tags Tracking - Assignments
// @Produce json
// @Security BearerAuth
// @Param rider_id query string false "Rider ID (UUID)"
// @Param route_id query string false "Route ID (UUID)"
// @Param date query string false "In force on this day (YYYY-MM-DD)"
// @Param page query int false "Page number (default 1)"
// @Param page_size query int false "Page size (default 20, max 100)"
// @Success 200 {object} models.ListAssignmentsResponse
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/assignments [get]
func (ctrl *TrackingController) ListAssignments(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}
	var query models.ListAssignmentsQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.AssignmentListFailed, utils.ExtractValidationError(c, err)))
		return
	}

	result, err := ctrl.trackingService.ListAssignments(c.Request.Context(), tenantID, query, ctrl.scopeUserID(c))
	if err != nil {
		if assignmentErrorResponse(c, err) {
			return
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.JSON(http.StatusOK, result)
}

// CreateAssignment godoc
// @Summary Assign a rider to a route
// @Description days_of_week is a bitmask, bit 0 Monday … bit 6 Sunday; valid_until empty is open-ended. Answers the route's capacity warnings, which never block
// @Tags Tracking - Assignments
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param assignment body models.AssignRiderDto true "Rider, route, weekdays, stops and validity"
// @Success 201 {object} models.AssignmentResponse
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 409 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/assignments [post]
func (ctrl *TrackingController) CreateAssignment(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}
	var dto models.AssignRiderDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.AssignmentCreateFailed, utils.ExtractValidationError(c, err)))
		return
	}

	result, err := ctrl.trackingService.CreateAssignment(c.Request.Context(), tenantID, &dto)
	if err != nil {
		if assignmentErrorResponse(c, err) {
			return
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.JSON(http.StatusCreated, result)
}

// CreateAssignmentsBulk godoc
// @Summary Assign many riders at once
// @Description All or none: the first item refused rolls back every one and the error's detail names its zero-based index
// @Tags Tracking - Assignments
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param assignments body []models.AssignRiderDto true "The assignments to create"
// @Success 201 {object} models.BulkAssignmentResponse
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 409 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/assignments/bulk [post]
func (ctrl *TrackingController) CreateAssignmentsBulk(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}
	dtos := []models.AssignRiderDto{}
	if err := c.ShouldBindJSON(&dtos); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.AssignmentCreateFailed, utils.ExtractValidationError(c, err)))
		return
	}
	if len(dtos) == 0 {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, trackingErrors.AssignmentCreateFailed))
		return
	}

	result, err := ctrl.trackingService.CreateAssignments(c.Request.Context(), tenantID, dtos)
	if err != nil {
		if assignmentErrorResponse(c, err) {
			return
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.JSON(http.StatusCreated, result)
}

// BulkAssignRiders godoc
// @Summary Add many riders to a route
// @Description Up to 100 riders in one call (TRACK-039 D1), each with its home as its stop and on the return too when the route has one; a rider without a home pin or already on another route that day is skipped with its reason and the rest still go in
// @Tags Tracking - Assignments
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param route_id path string true "Route ID (UUID)"
// @Param body body models.BulkAssignRidersDto true "Riders, and optionally days and start"
// @Success 200 {object} models.BulkAssignRidersResponse
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Router /tracking/routes/{route_id}/assignments/bulk [post]
func (ctrl *TrackingController) BulkAssignRiders(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}
	routeID, err := uuid.Parse(c.Param("route_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
		return
	}
	var dto models.BulkAssignRidersDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.AssignmentCreateFailed, utils.ExtractValidationError(c, err)))
		return
	}
	result, err := ctrl.trackingService.BulkAssignRiders(c.Request.Context(), tenantID, routeID, &dto)
	if err != nil {
		if assignmentErrorResponse(c, err) {
			return
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.JSON(http.StatusOK, result)
}

// ListRiderGroups godoc
// @Summary The groups riders use
// @Description The distinct groups ("5to B") of the tenant's riders, optionally of one organization (TRACK-039 D2)
// @Tags Tracking - Riders
// @Produce json
// @Security BearerAuth
// @Param organization_id query string false "Organization ID (UUID)"
// @Success 200 {object} map[string][]string
// @Router /tracking/riders/groups [get]
func (ctrl *TrackingController) ListRiderGroups(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}
	var query models.ListRiderGroupsQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.RiderListFailed, utils.ExtractValidationError(c, err)))
		return
	}
	groups, err := ctrl.trackingService.ListRiderGroups(c.Request.Context(), tenantID, query.OrganizationID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.JSON(http.StatusOK, gin.H{"groups": groups})
}

// UpdateAssignment godoc
// @Summary Edit an assignment
// @Description Every field but the rider; a field left out keeps its value. Answers the route's capacity warnings
// @Tags Tracking - Assignments
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param assignment_id path string true "Assignment ID (UUID)"
// @Param assignment body models.UpdateAssignmentDto true "The fields to change"
// @Success 200 {object} models.AssignmentResponse
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 409 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/assignments/{assignment_id} [patch]
func (ctrl *TrackingController) UpdateAssignment(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}
	assignmentID, err := uuid.Parse(c.Param("assignment_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
		return
	}
	var dto models.UpdateAssignmentDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.AssignmentUpdateFailed, utils.ExtractValidationError(c, err)))
		return
	}

	result, err := ctrl.trackingService.UpdateAssignment(c.Request.Context(), tenantID, assignmentID, &dto)
	if err != nil {
		if assignmentErrorResponse(c, err) {
			return
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.JSON(http.StatusOK, result)
}

// DeleteAssignment godoc
// @Summary Remove an assignment
// @Tags Tracking - Assignments
// @Produce json
// @Security BearerAuth
// @Param assignment_id path string true "Assignment ID (UUID)"
// @Success 204 "No Content"
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/assignments/{assignment_id} [delete]
func (ctrl *TrackingController) DeleteAssignment(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}
	assignmentID, err := uuid.Parse(c.Param("assignment_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
		return
	}

	if err := ctrl.trackingService.DeleteAssignment(c.Request.Context(), tenantID, assignmentID); err != nil {
		if assignmentErrorResponse(c, err) {
			return
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.Status(http.StatusNoContent)
}
