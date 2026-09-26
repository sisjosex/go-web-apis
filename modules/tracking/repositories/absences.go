package repositories

import (
	"context"
	"errors"

	trackingErrors "josex/web/modules/tracking/errors"
	"josex/web/modules/tracking/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

// The absence and suggestion slice of TRACK-022: one slice, one file.

// absenceCodes turns the absence SPs' own refusals into module codes; the scope refusals go through
// scopedErr.
var absenceCodes = map[string]string{
	"absence.not-found":      trackingErrors.AbsenceNotFound,
	"absence.overlap":        trackingErrors.AbsenceOverlap,
	"absence.cutoff":         trackingErrors.AbsenceCutoff,
	"absence.invalid-range":  trackingErrors.AbsenceInvalidRange,
	"rider.no-home-location": trackingErrors.RiderNoHomeLocation,
}

func mapAbsenceError(err error, fallbackCode string) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		if code, ok := absenceCodes[pgErr.Message]; ok {
			return &trackingErrors.TrackingError{Code: code, Err: pgErr}
		}
	}
	return scopedErr(err, fallbackCode)
}

func scanAbsence(a *models.RiderAbsence) []any {
	return []any{
		&a.ID, &a.RiderID, &a.DateFrom, &a.DateTo, &a.Direction, &a.Reason, &a.ReportedBy,
		&a.ReportedByName, &a.ReportedVia, &a.CreatedAt,
	}
}

func (r *TrackingRepository) ListRiderAbsences(ctx context.Context, tenantID, riderID uuid.UUID, query models.ListRiderAbsencesQuery, scopeUserID, guardianUserID *uuid.UUID) ([]*models.RiderAbsence, error) {
	rows, err := r.dbService.Query(ctx, `
		SELECT * FROM tracking.sp_list_rider_absences(
			p_tenant_id        := $1,
			p_rider_id         := $2,
			p_from             := $3::DATE,
			p_to               := $4::DATE,
			p_scope_user_id    := $5,
			p_guardian_user_id := $6
		)
	`, tenantID, riderID, query.From, query.To, scopeUserID, guardianUserID)
	if err != nil {
		return nil, mapAbsenceError(err, trackingErrors.AbsenceListFailed)
	}
	defer rows.Close()
	absences := []*models.RiderAbsence{}
	for rows.Next() {
		var a models.RiderAbsence
		if err := rows.Scan(scanAbsence(&a)...); err != nil {
			return nil, err
		}
		absences = append(absences, &a)
	}
	if err := rows.Err(); err != nil {
		return nil, mapAbsenceError(err, trackingErrors.AbsenceListFailed)
	}
	return absences, nil
}

func (r *TrackingRepository) CreateRiderAbsence(ctx context.Context, tenantID, riderID uuid.UUID, dto *models.CreateRiderAbsenceDto, userID, scopeUserID, guardianUserID *uuid.UUID) (*models.CreateRiderAbsenceResponse, error) {
	var a models.RiderAbsence
	var cancelled int32
	err := r.dbService.QueryRow(ctx, `
		SELECT * FROM tracking.sp_create_rider_absence(
			p_tenant_id        := $1,
			p_rider_id         := $2,
			p_date_from        := $3::DATE,
			p_date_to          := $4::DATE,
			p_direction        := $5,
			p_reason           := $6,
			p_user_id          := $7,
			p_scope_user_id    := $8,
			p_guardian_user_id := $9
		)
	`, tenantID, riderID, dto.DateFrom, dto.DateTo, dto.Direction, dto.Reason, userID, scopeUserID, guardianUserID,
	).Scan(append(scanAbsence(&a), &cancelled)...)
	if err != nil {
		return nil, mapAbsenceError(err, trackingErrors.AbsenceCreateFailed)
	}
	return &models.CreateRiderAbsenceResponse{Absence: &a, CancelledTasks: cancelled}, nil
}

func (r *TrackingRepository) DeleteRiderAbsence(ctx context.Context, tenantID, riderID, absenceID uuid.UUID, scopeUserID, guardianUserID *uuid.UUID) error {
	_, err := r.dbService.Execute(ctx, `
		SELECT tracking.sp_delete_rider_absence(
			p_tenant_id        := $1,
			p_rider_id         := $2,
			p_absence_id       := $3,
			p_scope_user_id    := $4,
			p_guardian_user_id := $5
		)
	`, tenantID, riderID, absenceID, scopeUserID, guardianUserID)
	if err != nil {
		return mapAbsenceError(err, trackingErrors.AbsenceDeleteFailed)
	}
	return nil
}

func (r *TrackingRepository) SuggestRiderStops(ctx context.Context, tenantID, riderID uuid.UUID, query models.SuggestRiderStopsQuery) ([]*models.RiderStopSuggestion, error) {
	rows, err := r.dbService.Query(ctx, `
		SELECT * FROM tracking.sp_suggest_rider_stops(
			p_tenant_id  := $1,
			p_rider_id   := $2,
			p_direction  := $3,
			p_days       := $4::SMALLINT,
			p_max_walk_m := $5,
			p_limit      := $6
		)
	`, tenantID, riderID, query.Direction, query.Days, query.MaxWalkM, query.Limit)
	if err != nil {
		return nil, mapAbsenceError(err, trackingErrors.RiderSuggestionsFailed)
	}
	defer rows.Close()
	suggestions := []*models.RiderStopSuggestion{}
	for rows.Next() {
		var s models.RiderStopSuggestion
		if err := rows.Scan(&s.RouteID, &s.RouteName, &s.Direction, &s.StopPlaceID, &s.StopName,
			&s.Latitude, &s.Longitude, &s.WalkDistanceM, &s.Capacity, &s.Assigned, &s.FreeSeats); err != nil {
			return nil, err
		}
		suggestions = append(suggestions, &s)
	}
	if err := rows.Err(); err != nil {
		return nil, mapAbsenceError(err, trackingErrors.RiderSuggestionsFailed)
	}
	return suggestions, nil
}
