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
}
