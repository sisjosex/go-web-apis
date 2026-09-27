package interfaces

import (
	"context"

	authModels "josex/web/modules/auth/models"

	"github.com/google/uuid"
)

// PushDeviceRepository is where an account's phones receive pushes (TRACK-012 D3), in the main
// database whatever tenant a notice comes from.
type PushDeviceRepository interface {
	Upsert(ctx context.Context, userID uuid.UUID, dto authModels.RegisterPushDeviceDto) error
	Delete(ctx context.Context, userID uuid.UUID, deviceID string) error
}

// PushDeviceService is what /mobile/devices calls.
type PushDeviceService interface {
	Register(ctx context.Context, userID uuid.UUID, dto authModels.RegisterPushDeviceDto) error
	Forget(ctx context.Context, userID uuid.UUID, deviceID string) error
}
