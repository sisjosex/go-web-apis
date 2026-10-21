package repositories

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	trackingErrors "josex/web/modules/tracking/errors"
	"josex/web/modules/tracking/models"
)

// The invitations of TRACK-062: who to invite and the invited are tracking reads (the tenant's
// database); the invitations themselves live in the main database beside the accounts (D3).

var inviteCodes = map[string]string{
	"invite.not-found":                     trackingErrors.InviteNotFound,
	"invite.target-required":               trackingErrors.InviteTargetRequired,
	"organization.not-found":               trackingErrors.OrganizationNotFound,
	trackingErrors.OrganizationScopeDenied: trackingErrors.OrganizationScopeDenied,
}

func mapInviteError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		if code, ok := inviteCodes[pgErr.Message]; ok {
			return &trackingErrors.TrackingError{Code: code, Err: pgErr}
		}
	}
	return &trackingErrors.TrackingError{Code: trackingErrors.InviteFailed, Err: err}
}

// ListInvitableContacts answers the guardian contacts without an account of an organization's or the
// given riders, one person's rows adjacent.
func (r *TrackingRepository) ListInvitableContacts(ctx context.Context, tenantID uuid.UUID, organizationID *uuid.UUID, riderIDs []uuid.UUID, scopeUserID *uuid.UUID) ([]models.InvitableContact, error) {
	rows, err := r.dbService.Query(ctx,
		`SELECT * FROM tracking.sp_list_invitable_contacts($1, $2, $3, $4)`, tenantID, organizationID, riderIDs, scopeUserID)
	if err != nil {
		return nil, mapInviteError(err)
	}
	contacts, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (models.InvitableContact, error) {
		var c models.InvitableContact
		err := row.Scan(&c.ContactID, &c.RiderID, &c.RiderName, &c.Name, &c.Phone, &c.Email)
		return c, err
	})
	if err != nil {
		return nil, mapInviteError(err)
	}
	return contacts, nil
}

// ListOrganizationInvited answers one page of the guardian accounts of an organization's riders;
// userIDs non-nil keeps only those accounts.
func (r *TrackingRepository) ListOrganizationInvited(ctx context.Context, tenantID, organizationID uuid.UUID, scopeUserID *uuid.UUID, userIDs []uuid.UUID, page, pageSize int) ([]*models.OrganizationInvite, int64, error) {
	rows, err := r.dbService.Query(ctx,
		`SELECT * FROM tracking.sp_list_organization_invited($1, $2, $3, $4, $5, $6)`,
		tenantID, organizationID, scopeUserID, userIDs, page, pageSize)
	if err != nil {
		return nil, 0, mapInviteError(err)
	}
	defer rows.Close()
	invites := []*models.OrganizationInvite{}
	var total int64
	for rows.Next() {
		var inv models.OrganizationInvite
		if err := rows.Scan(&inv.UserID, &inv.FirstName, &inv.LastName, &inv.Phone, &inv.Email, &inv.Riders, &total); err != nil {
			return nil, 0, err
		}
		invites = append(invites, &inv)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, mapInviteError(err)
	}
	return invites, total, nil
}

// InvitedGuardian answers the guardian account an invitation goes to, in the caller's write scope.
func (r *TrackingRepository) InvitedGuardian(ctx context.Context, tenantID, userID uuid.UUID, scopeUserID *uuid.UUID) (*models.InvitedGuardian, error) {
	var g models.InvitedGuardian
	err := r.dbService.QueryRow(ctx, `SELECT * FROM tracking.sp_get_invited_guardian($1, $2, $3)`, tenantID, userID, scopeUserID).
		Scan(&g.UserID, &g.FirstName, &g.Email, &g.Phone)
	if err != nil {
		return nil, mapInviteError(err)
	}
	return &g, nil
}

// ListAppInvites answers the business's invitations of userIDs (nil: any) in status ("": any), from
// the main database.
func (r *TrackingRepository) ListAppInvites(ctx context.Context, tenantID uuid.UUID, userIDs []uuid.UUID, status string) ([]models.AppInvite, error) {
	var statusArg *string
	if status != "" {
		statusArg = &status
	}
	rows, err := r.dbService.GetPrimaryPool().Query(ctx,
		`SELECT user_id, channel, sent_at, status FROM tenancy.sp_list_app_invites($1, $2, $3)`, tenantID, userIDs, statusArg)
	if err != nil {
		return nil, mapInviteError(err)
	}
	invites, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (models.AppInvite, error) {
		var inv models.AppInvite
		err := row.Scan(&inv.UserID, &inv.Channel, &inv.SentAt, &inv.Status)
		return inv, err
	})
	if err != nil {
		return nil, mapInviteError(err)
	}
	return invites, nil
}

// MarkAppInviteSent records that the account's invitation in the business went out now.
func (r *TrackingRepository) MarkAppInviteSent(ctx context.Context, tenantID, userID uuid.UUID) (time.Time, error) {
	var sent time.Time
	err := r.dbService.GetPrimaryPool().QueryRow(ctx, `SELECT tenancy.sp_mark_app_invite_sent($1, $2)`, tenantID, userID).Scan(&sent)
	if err != nil {
		return time.Time{}, mapInviteError(err)
	}
	return sent, nil
}
