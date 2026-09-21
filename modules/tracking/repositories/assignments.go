package repositories

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	trackingErrors "josex/web/modules/tracking/errors"
	"josex/web/modules/tracking/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// The rider route assignment slice of TRACK-009, beside tracking_repository.go for the same reason
// schedules_calendars.go is: one slice, one file.

// scanAssignment is the one scan order every assignment read and write shares.
func scanAssignment(a *models.RiderRouteAssignment) []any {
	return []any{
		&a.ID, &a.RiderID, &a.RiderName, &a.RouteID, &a.RouteName, &a.Direction, &a.DaysOfWeek,
		&a.PickupStopPlaceID, &a.PickupStopName, &a.DropoffStopPlaceID, &a.DropoffStopName,
		&a.ValidFrom, &a.ValidUntil, &a.CreatedAt, &a.UpdatedAt,
	}
}

// mapAssignmentError turns the SP's own codes into module codes, falling back to fallbackCode for
// anything else. A bulk refusal carries DETAIL 'index=<n>', which becomes the error's Detail.
func mapAssignmentError(err error, fallbackCode string) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return &trackingErrors.TrackingError{Code: fallbackCode, Err: err}
	}
	var code string
	switch pgErr.Message {
	case "assignment.not-found":
		code = trackingErrors.AssignmentNotFound
	case "assignment.overlap":
		code = trackingErrors.AssignmentOverlap
	case "assignment.stop-not-on-route":
		code = trackingErrors.AssignmentStopNotOnRoute
	case "assignment.days-of-week":
		code = trackingErrors.AssignmentDaysOfWeek
	case "assignment.range":
		code = trackingErrors.AssignmentRange
	case "rider.not-found":
		code = trackingErrors.RiderNotFound
	case "route.not-found":
		code = trackingErrors.RouteNotFound
	default:
		return scopedErr(err, fallbackCode)
	}
	mapped := &trackingErrors.TrackingError{Code: code, Err: pgErr}
	if index, ok := strings.CutPrefix(pgErr.Detail, "index="); ok {
		if n, convErr := strconv.Atoi(index); convErr == nil {
			mapped.Detail = map[string]int{"index": n}
		}
	}
	return mapped
}

// scanAssignmentWrites reads a write SP's rows: the assignments, each carrying the same warnings.
func scanAssignmentWrites(rows pgx.Rows) ([]*models.RiderRouteAssignment, []models.AssignmentWarning, error) {
	defer rows.Close()
	assignments := []*models.RiderRouteAssignment{}
	warnings := []models.AssignmentWarning{}
	var raw []byte
	for rows.Next() {
		var a models.RiderRouteAssignment
		if err := rows.Scan(append(scanAssignment(&a), &raw)...); err != nil {
			return nil, nil, err
		}
		assignments = append(assignments, &a)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	if raw != nil {
		if err := json.Unmarshal(raw, &warnings); err != nil {
			return nil, nil, err
		}
	}
	return assignments, warnings, nil
}

// CreateAssignments creates every item in one transaction or none; a single POST is a list of one.
func (r *TrackingRepository) CreateAssignments(ctx context.Context, tenantID uuid.UUID, dtos []models.AssignRiderDto) (*models.BulkAssignmentResponse, error) {
	payload, err := json.Marshal(dtos)
	if err != nil {
		return nil, &trackingErrors.TrackingError{Code: trackingErrors.AssignmentCreateFailed, Err: err}
	}
	rows, err := r.dbService.Query(ctx, `
		SELECT * FROM tracking.sp_create_rider_route_assignments(
			p_tenant_id := $1,
			p_items     := $2
		)
	`, tenantID, payload)
	if err != nil {
		return nil, mapAssignmentError(err, trackingErrors.AssignmentCreateFailed)
	}
	assignments, warnings, err := scanAssignmentWrites(rows)
	if err != nil {
		return nil, mapAssignmentError(err, trackingErrors.AssignmentCreateFailed)
	}
	return &models.BulkAssignmentResponse{Assignments: assignments, Warnings: warnings}, nil
}

func (r *TrackingRepository) UpdateAssignment(ctx context.Context, tenantID uuid.UUID, assignmentID uuid.UUID, dto *models.UpdateAssignmentDto) (*models.AssignmentResponse, error) {
	changes, err := json.Marshal(dto)
	if err != nil {
		return nil, &trackingErrors.TrackingError{Code: trackingErrors.AssignmentUpdateFailed, Err: err}
	}
	rows, err := r.dbService.Query(ctx, `
		SELECT * FROM tracking.sp_update_rider_route_assignment(
			p_tenant_id := $1,
			p_id        := $2,
			p_changes   := $3
		)
	`, tenantID, assignmentID, changes)
	if err != nil {
		return nil, mapAssignmentError(err, trackingErrors.AssignmentUpdateFailed)
	}
	assignments, warnings, err := scanAssignmentWrites(rows)
	if err != nil {
		return nil, mapAssignmentError(err, trackingErrors.AssignmentUpdateFailed)
	}
	return &models.AssignmentResponse{Assignment: assignments[0], Warnings: warnings}, nil
}

func (r *TrackingRepository) DeleteAssignment(ctx context.Context, tenantID uuid.UUID, assignmentID uuid.UUID) error {
	_, err := r.dbService.Execute(ctx, `SELECT tracking.sp_delete_rider_route_assignment(p_tenant_id := $1, p_id := $2)`, tenantID, assignmentID)
	if err != nil {
		return mapAssignmentError(err, trackingErrors.AssignmentDeleteFailed)
	}
	return nil
}

// ListAssignments returns one page of the tenant's assignments plus the total the same filters match.
func (r *TrackingRepository) ListAssignments(ctx context.Context, tenantID uuid.UUID, query models.ListAssignmentsQuery, scopeUserID *uuid.UUID) ([]*models.RiderRouteAssignment, int64, error) {
	rows, err := r.dbService.Query(ctx, `
		SELECT * FROM tracking.sp_list_rider_route_assignments(
			p_tenant_id     := $1,
			p_rider_id      := $2::UUID,
			p_route_id      := $3::UUID,
			p_date          := $4::DATE,
			p_scope_user_id := $5,
			p_page          := $6,
			p_page_size     := $7
		)
	`, tenantID, query.RiderID, query.RouteID, query.Date, scopeUserID, query.Page, query.PageSize)
	if err != nil {
		return nil, 0, scopedErr(err, trackingErrors.AssignmentListFailed)
	}
	defer rows.Close()
	assignments := []*models.RiderRouteAssignment{}
	var totalCount int64
	for rows.Next() {
		var a models.RiderRouteAssignment
		if err := rows.Scan(append(scanAssignment(&a), &totalCount)...); err != nil {
			return nil, 0, err
		}
		assignments = append(assignments, &a)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, scopedErr(err, trackingErrors.AssignmentListFailed)
	}
	return assignments, totalCount, nil
}
