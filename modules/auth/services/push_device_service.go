package services

import (
	"context"

	"josex/web/modules/auth/interfaces"
	authModels "josex/web/modules/auth/models"

	"github.com/google/uuid"
)

// pushDeviceService is the phone registration of TRACK-012.
type pushDeviceService struct {
	repo interfaces.PushDeviceRepository
}

func NewPushDeviceService(repo interfaces.PushDeviceRepository) interfaces.PushDeviceService {
	return &pushDeviceService{repo: repo}
}

func (s *pushDeviceService) Register(ctx context.Context, userID uuid.UUID, dto authModels.RegisterPushDeviceDto) error {
	return s.repo.Upsert(ctx, userID, dto)
}

func (s *pushDeviceService) Forget(ctx context.Context, userID uuid.UUID, deviceID string) error {
	return s.repo.Delete(ctx, userID, deviceID)
}
