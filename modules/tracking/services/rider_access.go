package services

import (
	"context"

	"github.com/google/uuid"

	tenancyModels "josex/web/modules/tenancy/models"
	trackingErrors "josex/web/modules/tracking/errors"
	"josex/web/modules/tracking/models"
)

// A passenger's access by QR (MOBILE-042): the office issues a one-time code, the phone scans it, and the
// account signs in with the emailed code it would use anyway. The code carries the tenant's slug, as a
// share link's token does, because the phone has no tenant yet.

// IssueRiderAccessCode answers a new code; it is shown once, only its hash is kept.
func (s *AccessService) IssueRiderAccessCode(ctx context.Context, tenantID uuid.UUID, tenantSlug string, riderID uuid.UUID, createdBy, scopeUserID *uuid.UUID) (*models.RiderAccessCode, error) {
	code, hash, err := newShareToken(tenantSlug)
	if err != nil {
		return nil, err
	}
	expires, err := s.repo.IssueRiderAccessCode(ctx, tenantID, riderID, hash, createdBy, scopeUserID)
	if err != nil {
		return nil, err
	}
	return &models.RiderAccessCode{Code: code, ExpiresAt: expires}, nil
}

// RevokeRiderAccessCode ends the rider's live code, if any.
func (s *AccessService) RevokeRiderAccessCode(ctx context.Context, tenantID, riderID uuid.UUID, scopeUserID *uuid.UUID) error {
	return s.repo.RevokeRiderAccessCode(ctx, tenantID, riderID, scopeUserID)
}

// CheckRiderAccessCode readies the sign-in: a code that still opens finds or creates the email's account,
// at the portal level, so the emailed code has an account to open. The account reaches no rider until
// the signed-in phone redeems the code, and that is what uses it up: a mistyped email costs nothing.
func (s *AccessService) CheckRiderAccessCode(ctx context.Context, tenantID uuid.UUID, token, email string) error {
	_, hash, ok := ParseShareToken(token)
	if !ok {
		return &trackingErrors.TrackingError{Code: trackingErrors.RiderAccessNotFound}
	}
	name, err := s.repo.CheckRiderAccessCode(ctx, tenantID, hash)
	if err != nil {
		return err
	}
	first, last := splitName(&name)
	_, err = s.repo.GrantAppAccess(ctx, tenantID, &models.GrantAccessDto{Email: &email, FirstName: first, LastName: last}, tenancyModels.RolePortal, models.InviteEmail, nil)
	return err
}

// RedeemRiderAccessCode uses the code up: the signed-in account becomes the rider's own contact.
func (s *AccessService) RedeemRiderAccessCode(ctx context.Context, tenantID uuid.UUID, token string, userID uuid.UUID) (uuid.UUID, error) {
	_, hash, ok := ParseShareToken(token)
	if !ok {
		return uuid.Nil, &trackingErrors.TrackingError{Code: trackingErrors.RiderAccessNotFound}
	}
	return s.repo.RedeemRiderAccessCode(ctx, tenantID, hash, userID)
}
