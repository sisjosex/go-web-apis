package repositories

import (
	"context"
	"encoding/json"
	"errors"

	trackingErrors "josex/web/modules/tracking/errors"
	"josex/web/modules/tracking/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

// The guardian's rider reads of TRACK-061: each SP builds its whole answer as one JSON value.

var riderReadCodes = map[string]string{
	"rider.history-range": trackingErrors.RiderHistoryRange,
	"rider.cutoffs-range": trackingErrors.RiderCutoffsRange,
}

// riderRead runs one of the reads and decodes its value into into.
func (r *TrackingRepository) riderRead(ctx context.Context, into any, query string, args ...any) error {
	var raw []byte
	if err := r.dbService.QueryRow(ctx, query, args...).Scan(&raw); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			if code, ok := riderReadCodes[pgErr.Message]; ok {
				return &trackingErrors.TrackingError{Code: code, Err: pgErr}
			}
		}
		return scopedErr(err, trackingErrors.RiderReadFailed)
	}
	if err := json.Unmarshal(raw, into); err != nil {
		return &trackingErrors.TrackingError{Code: trackingErrors.RiderReadFailed, Err: err}
	}
	return nil
}

func (r *TrackingRepository) RiderToday(ctx context.Context, tenantID, riderID uuid.UUID, scopeUserID, guardianUserID *uuid.UUID) (*models.RiderToday, error) {
	var day models.RiderToday
	err := r.riderRead(ctx, &day, `SELECT tracking.sp_rider_today($1, $2, $3, $4)`,
		tenantID, riderID, scopeUserID, guardianUserID)
	return &day, err
}

func (r *TrackingRepository) RiderHistory(ctx context.Context, tenantID, riderID uuid.UUID, query models.RiderDaysQuery, scopeUserID, guardianUserID *uuid.UUID) (*models.RiderHistory, error) {
	var history models.RiderHistory
	err := r.riderRead(ctx, &history, `SELECT tracking.sp_rider_history($1, $2, $3::DATE, $4::DATE, $5, $6)`,
		tenantID, riderID, query.From, query.To, scopeUserID, guardianUserID)
	return &history, err
}

func (r *TrackingRepository) RiderAbsenceCutoffs(ctx context.Context, tenantID, riderID uuid.UUID, query models.RiderDaysQuery, scopeUserID, guardianUserID *uuid.UUID) (*models.RiderAbsenceCutoffs, error) {
	var cutoffs models.RiderAbsenceCutoffs
	err := r.riderRead(ctx, &cutoffs, `SELECT tracking.sp_rider_absence_cutoffs($1, $2, $3::DATE, $4::DATE, $5, $6)`,
		tenantID, riderID, query.From, query.To, scopeUserID, guardianUserID)
	return &cutoffs, err
}
