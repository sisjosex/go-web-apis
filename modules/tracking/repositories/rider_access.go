package repositories

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	trackingErrors "josex/web/modules/tracking/errors"
)

// riderAccessErr names a code that does not open; anything else is the scope's or a failure.
func riderAccessErr(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Message {
		case "rider-access.not-found":
			return &trackingErrors.TrackingError{Code: trackingErrors.RiderAccessNotFound, Err: err}
		case "rider-access.used":
			return &trackingErrors.TrackingError{Code: trackingErrors.RiderAccessUsed, Err: err}
		case "rider-access.expired":
			return &trackingErrors.TrackingError{Code: trackingErrors.RiderAccessExpired, Err: err}
		}
	}
	return scopedErr(err, trackingErrors.RiderAccessFailed)
}

// IssueRiderAccessCode stores a new code's hash and answers when it expires.
func (r *TrackingRepository) IssueRiderAccessCode(ctx context.Context, tenantID, riderID uuid.UUID, hash []byte, createdBy, scopeUserID *uuid.UUID) (time.Time, error) {
	var expires time.Time
	err := r.dbService.QueryRow(ctx, `
		SELECT tracking.sp_rider_access_code_issue(p_tenant_id := $1, p_rider_id := $2, p_code_hash := $3, p_created_by := $4, p_scope_user_id := $5)
	`, tenantID, riderID, hash, createdBy, scopeUserID).Scan(&expires)
	if err != nil {
		return time.Time{}, scopedErr(err, trackingErrors.RiderAccessFailed)
	}
	return expires, nil
}

// RevokeRiderAccessCode ends the rider's live code.
func (r *TrackingRepository) RevokeRiderAccessCode(ctx context.Context, tenantID, riderID uuid.UUID, scopeUserID *uuid.UUID) error {
	_, err := r.dbService.Execute(ctx, `
		SELECT tracking.sp_rider_access_code_revoke(p_tenant_id := $1, p_rider_id := $2, p_scope_user_id := $3)
	`, tenantID, riderID, scopeUserID)
	if err != nil {
		return scopedErr(err, trackingErrors.RiderAccessFailed)
	}
	return nil
}

// CheckRiderAccessCode answers the passenger's name when the code still opens.
func (r *TrackingRepository) CheckRiderAccessCode(ctx context.Context, tenantID uuid.UUID, hash []byte) (string, error) {
	var name string
	err := r.dbService.QueryRow(ctx, `
		SELECT tracking.sp_rider_access_code_check(p_tenant_id := $1, p_code_hash := $2)
	`, tenantID, hash).Scan(&name)
	if err != nil {
		return "", riderAccessErr(err)
	}
	return name, nil
}

// RedeemRiderAccessCode uses the code and answers its rider.
func (r *TrackingRepository) RedeemRiderAccessCode(ctx context.Context, tenantID uuid.UUID, hash []byte, userID uuid.UUID) (uuid.UUID, error) {
	var riderID uuid.UUID
	err := r.dbService.QueryRow(ctx, `
		SELECT tracking.sp_rider_access_code_redeem(p_tenant_id := $1, p_code_hash := $2, p_user_id := $3)
	`, tenantID, hash, userID).Scan(&riderID)
	if err != nil {
		return uuid.Nil, riderAccessErr(err)
	}
	return riderID, nil
}
