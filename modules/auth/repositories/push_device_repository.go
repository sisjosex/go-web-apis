package repositories

import (
	"context"

	"josex/web/modules/auth/interfaces"
	authModels "josex/web/modules/auth/models"
	database "josex/web/modules/core/services"

	"github.com/google/uuid"
)

type pushDeviceRepository struct {
	dbService database.DatabaseService
}

func NewPushDeviceRepository(dbService database.DatabaseService) interfaces.PushDeviceRepository {
	return &pushDeviceRepository{dbService: dbService}
}

// Upsert registers or refreshes the device; its token leaves any other account it sat under.
func (r *pushDeviceRepository) Upsert(ctx context.Context, userID uuid.UUID, dto authModels.RegisterPushDeviceDto) error {
	_, err := r.dbService.Execute(ctx,
		`SELECT auth.sp_upsert_push_device(p_user_id := $1, p_device_id := $2, p_platform := $3, p_token := $4)`,
		userID, dto.DeviceID, dto.Platform, dto.Token)
	return err
}

// Delete forgets the device; an unknown one changes nothing.
func (r *pushDeviceRepository) Delete(ctx context.Context, userID uuid.UUID, deviceID string) error {
	_, err := r.dbService.Execute(ctx,
		`SELECT auth.sp_delete_push_device(p_user_id := $1, p_device_id := $2)`, userID, deviceID)
	return err
}
