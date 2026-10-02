package repositories

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	trackingErrors "josex/web/modules/tracking/errors"
	"josex/web/modules/tracking/models"
)

// accessCodes turns the access SPs' refusals — tenancy's grant and tracking's links — into module codes.
var accessCodes = map[string]string{
	"access.web-account":                   trackingErrors.AccessWebAccount,
	"access.level-mismatch":                trackingErrors.AccessLevelMismatch,
	"access.user-not-found":                trackingErrors.AccessUserNotFound,
	"access.user-deleted":                  trackingErrors.AccessUserDeleted,
	"rider.guardian-not-found":             trackingErrors.RiderGuardianNotFound,
	"rider.not-found":                      trackingErrors.RiderNotFound,
	"driver.not-found":                     trackingErrors.DriverNotFound,
	"driver.user-already-linked":           trackingErrors.DriverUserAlreadyLinked,
	trackingErrors.OrganizationScopeDenied: trackingErrors.OrganizationScopeDenied,
}

func mapAccessError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		if code, ok := accessCodes[pgErr.Message]; ok {
			return &trackingErrors.TrackingError{Code: code, Err: pgErr}
		}
	}
	return &trackingErrors.TrackingError{Code: trackingErrors.AccessGrantFailed, Err: err}
}

// GrantAppAccess finds or creates the account and gives it the level, in the main database: the
// account and tenant_users live there even when tracking runs in a dedicated tenant database, so the
// call goes to the primary pool, not the request's.
func (r *TrackingRepository) GrantAppAccess(ctx context.Context, tenantID uuid.UUID, dto *models.GrantAccessDto, level string) (*models.AppAccessGrant, error) {
	var g models.AppAccessGrant
	err := r.dbService.GetPrimaryPool().QueryRow(ctx, `
		SELECT * FROM tenancy.sp_grant_app_access(
			p_tenant_id  := $1,
			p_user_id    := $2,
			p_email      := $3,
			p_first_name := $4,
			p_last_name  := $5,
			p_phone      := $6,
			p_level      := $7
		)`, tenantID, dto.UserID, dto.Email, dto.FirstName, dto.LastName, dto.Phone, level,
	).Scan(&g.UserID, &g.Email, &g.FirstName, &g.LastName, &g.Status, &g.MembershipAdded, &g.TenantName)
	if err != nil {
		return nil, mapAccessError(err)
	}
	return &g, nil
}

// RevokeAppAccess ends the app membership at level; a membership at any other level is left alone.
func (r *TrackingRepository) RevokeAppAccess(ctx context.Context, tenantID, userID uuid.UUID, level string) error {
	var revoked bool
	err := r.dbService.GetPrimaryPool().QueryRow(ctx,
		`SELECT tenancy.sp_revoke_app_access($1, $2, $3)`, tenantID, userID, level).Scan(&revoked)
	if err != nil {
		return mapAccessError(err)
	}
	return nil
}

// TenantName is the name the access notice signs with, read off the main database.
func (r *TrackingRepository) TenantName(ctx context.Context, tenantID uuid.UUID) (string, error) {
	var name string
	err := r.dbService.GetPrimaryPool().QueryRow(ctx,
		`SELECT tenancy.sp_get_tenant_name($1)`, tenantID).Scan(&name)
	if err != nil {
		return "", mapAccessError(err)
	}
	return name, nil
}

// UserLinked answers whether anything in the tenant's tracking data still names the account.
func (r *TrackingRepository) UserLinked(ctx context.Context, tenantID, userID uuid.UUID) (bool, error) {
	var linked bool
	err := r.dbService.QueryRow(ctx, `SELECT tracking.fn_user_linked($1, $2)`, tenantID, userID).Scan(&linked)
	if err != nil {
		return false, mapAccessError(err)
	}
	return linked, nil
}

func (r *TrackingRepository) ListRiderGuardians(ctx context.Context, tenantID, riderID uuid.UUID, scopeUserID *uuid.UUID) ([]*models.RiderGuardian, error) {
	rows, err := r.dbService.Query(ctx,
		`SELECT * FROM tracking.sp_list_rider_guardians($1, $2, $3)`, tenantID, riderID, scopeUserID)
	if err != nil {
		return nil, mapAccessError(err)
	}
	defer rows.Close()

	guardians := []*models.RiderGuardian{}
	for rows.Next() {
		var g models.RiderGuardian
		if err := rows.Scan(&g.UserID, &g.Email, &g.FirstName, &g.LastName, &g.Phone, &g.IsPrimary); err != nil {
			return nil, mapAccessError(err)
		}
		guardians = append(guardians, &g)
	}
	if err := rows.Err(); err != nil {
		return nil, mapAccessError(err)
	}
	return guardians, nil
}

func (r *TrackingRepository) AddRiderGuardian(ctx context.Context, tenantID, riderID uuid.UUID, grant *models.AppAccessGrant, phone *string, scopeUserID *uuid.UUID) error {
	name := joinName(grant.FirstName, grant.LastName)
	_, err := r.dbService.Execute(ctx, `
		SELECT tracking.sp_rider_guardian_add(
			p_tenant_id     := $1,
			p_rider_id      := $2,
			p_user_id       := $3,
			p_name          := $4,
			p_email         := $5,
			p_phone         := $6,
			p_scope_user_id := $7
		)`, tenantID, riderID, grant.UserID, name, grant.Email, phone, scopeUserID)
	if err != nil {
		return mapAccessError(err)
	}
	return nil
}

func (r *TrackingRepository) RemoveRiderGuardian(ctx context.Context, tenantID, riderID, userID uuid.UUID, scopeUserID *uuid.UUID) error {
	_, err := r.dbService.Execute(ctx,
		`SELECT tracking.sp_rider_guardian_remove($1, $2, $3, $4)`, tenantID, riderID, userID, scopeUserID)
	if err != nil {
		return mapAccessError(err)
	}
	return nil
}

// SetDriverAccount links userID (nil clears) and answers the account it replaced, if any.
func (r *TrackingRepository) SetDriverAccount(ctx context.Context, tenantID, driverID uuid.UUID, userID *uuid.UUID) (*uuid.UUID, error) {
	var previous *uuid.UUID
	err := r.dbService.QueryRow(ctx,
		`SELECT tracking.sp_driver_account_set($1, $2, $3)`, tenantID, driverID, userID).Scan(&previous)
	if err != nil {
		return nil, mapAccessError(err)
	}
	return previous, nil
}

func joinName(first, last *string) *string {
	name := ""
	if first != nil {
		name = *first
	}
	if last != nil && *last != "" {
		if name != "" {
			name += " "
		}
		name += *last
	}
	if name == "" {
		return nil
	}
	return &name
}
