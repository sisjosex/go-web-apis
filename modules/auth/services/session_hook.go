package services

import (
	"context"

	"github.com/google/uuid"
)

// SessionHook runs after a session is created for userID — another module's reaction to a sign-in
// (TRACK-062 D3: tenancy accepts the account's open invitations). routes.go wires it, so auth never
// imports the module behind it. A hook logs its own failures: a sign-in never fails on one.
type SessionHook func(ctx context.Context, userID uuid.UUID)

func runSessionHooks(ctx context.Context, hooks []SessionHook, userID uuid.UUID) {
	for _, hook := range hooks {
		hook(ctx, userID)
	}
}
