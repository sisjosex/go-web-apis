package repositories

import (
	"context"
	"log"

	"josex/web/modules/core/services"

	"github.com/google/uuid"
)

// InviteRepository is the sign-in side of tenancy.app_invites (TRACK-062 D3). It always runs on the
// platform database, where the accounts and their invitations live.
type InviteRepository struct {
	dbService services.DatabaseService
}

// NewInviteRepository creates an InviteRepository.
func NewInviteRepository(dbService services.DatabaseService) *InviteRepository {
	return &InviteRepository{dbService: dbService}
}

// AcceptAppInvites marks the account's open invitations accepted — the auth SessionHook run after
// every sign-in. A failure is logged and the sign-in stands: the next one accepts them.
func (r *InviteRepository) AcceptAppInvites(ctx context.Context, userID uuid.UUID) {
	if _, err := r.dbService.GetPrimaryPool().Exec(ctx, `SELECT tenancy.sp_accept_app_invites($1)`, userID); err != nil {
		log.Printf("⚠️  invites: could not accept the invitations of %s: %v", userID, err)
	}
}
