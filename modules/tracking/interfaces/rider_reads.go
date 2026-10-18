package interfaces

import (
	"context"

	"josex/web/modules/tracking/models"

	"github.com/google/uuid"
)

// RiderReadsRepository is the guardian's rider reads (TRACK-061), scoped as the rider's status.
type RiderReadsRepository interface {
	RiderToday(ctx context.Context, tenantID, riderID uuid.UUID, scopeUserID, guardianUserID *uuid.UUID) (*models.RiderToday, error)
	RiderHistory(ctx context.Context, tenantID, riderID uuid.UUID, query models.RiderDaysQuery, scopeUserID, guardianUserID *uuid.UUID) (*models.RiderHistory, error)
	RiderAbsenceCutoffs(ctx context.Context, tenantID, riderID uuid.UUID, query models.RiderDaysQuery, scopeUserID, guardianUserID *uuid.UUID) (*models.RiderAbsenceCutoffs, error)
}

// PortalDirectory is what GET /mobile/portal/me reads from the platform database (TRACK-061 D1): the
// account's name and the tenants where it holds the portal role — active, not suspended, newest
// membership first — each with its database.
type PortalDirectory interface {
	PortalAccount(ctx context.Context, userID uuid.UUID) (models.PortalUser, []models.PortalMembership, error)
}
