package interfaces

import (
	"context"

	"josex/web/modules/tracking/models"

	"github.com/google/uuid"
)

// PositionsRepository is the GPS slice of the tracking repository (TRACK-010): what the ingest
// endpoint and the stream consumer call.
type PositionsRepository interface {
	IngestPositions(ctx context.Context, tenantID uuid.UUID, points []byte, radii models.IngestRadii) ([]models.IngestedVehicle, error)
	DriverActiveVehicle(ctx context.Context, tenantID, userID uuid.UUID) (*uuid.UUID, error)
}
