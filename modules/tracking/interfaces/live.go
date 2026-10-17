package interfaces

import (
	"context"
	"time"

	"josex/web/modules/tracking/models"

	"github.com/google/uuid"
)

// LiveRepository is the realtime slice of the tracking repository (TRACK-025): the snapshots and the
// subscribe check.
type LiveRepository interface {
	LiveFleet(ctx context.Context, tenantID uuid.UUID, organizationID *string) ([]models.LiveVehicle, error)
	TripLive(ctx context.Context, tenantID, tripID uuid.UUID, trailSince *time.Time) (*models.TripLive, []models.PendingStop, error)
	RiderLive(ctx context.Context, tenantID, riderID uuid.UUID, scopeUserID, guardianUserID *uuid.UUID) (*models.RiderLive, []models.PendingStop, error)
	CanSubscribe(ctx context.Context, tenantID, userID uuid.UUID, guardian bool, channel string) (bool, error)
	TripVisibleToUser(ctx context.Context, tenantID, tripID, userID uuid.UUID) (bool, error)
	TripDelayCheck(ctx context.Context, tenantID, tripID uuid.UUID, eta []models.StopEta) ([]uuid.UUID, error)
	// SharedTrip is a share link's trip (TRACK-060); nil when the token opens nothing.
	SharedTrip(ctx context.Context, tenantID uuid.UUID, tokenHash []byte) (*models.SharedTrip, error)
}

type tenantKey struct{}

// WithTenant carries the tenant into calls whose signature has no room for it — geo's historical
// ETA source reads it back with TenantFrom (TRACK-066 D2).
func WithTenant(ctx context.Context, tenantID uuid.UUID) context.Context {
	return context.WithValue(ctx, tenantKey{}, tenantID)
}

func TenantFrom(ctx context.Context) (uuid.UUID, bool) {
	tenantID, ok := ctx.Value(tenantKey{}).(uuid.UUID)
	return tenantID, ok
}
