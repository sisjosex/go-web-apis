package services

import (
	"context"
	"encoding/json"

	trackingErrors "josex/web/modules/tracking/errors"
	"josex/web/modules/tracking/interfaces"
	"josex/web/modules/tracking/models"

	"github.com/google/uuid"
)

// notificationService is the portal's feed and settings of TRACK-012.
type notificationService struct {
	repo interfaces.NotificationRepository
}

func NewNotificationService(repo interfaces.NotificationRepository) interfaces.NotificationService {
	return &notificationService{repo: repo}
}

func (s *notificationService) List(ctx context.Context, tenantID, userID uuid.UUID, query models.ListNotificationsQuery) (*models.ListNotificationsResponse, error) {
	return s.repo.ListNotifications(ctx, tenantID, userID, query)
}

func (s *notificationService) MarkRead(ctx context.Context, tenantID, userID uuid.UUID, dto models.MarkNotificationsReadDto) error {
	return s.repo.MarkNotificationsRead(ctx, tenantID, userID, dto)
}

func (s *notificationService) Settings(ctx context.Context, tenantID, userID uuid.UUID) ([]models.NotificationSetting, error) {
	return s.repo.NotificationSettings(ctx, tenantID, userID)
}

func (s *notificationService) SetSettings(ctx context.Context, tenantID, userID uuid.UUID, settings []models.NotificationSetting) ([]models.NotificationSetting, error) {
	// A rider sent without `muted` has every notice on: nil would marshal to null, which the SP
	// cannot iterate (MOBILE-004 — the phone leaves an empty list out).
	for i := range settings {
		if settings[i].Muted == nil {
			settings[i].Muted = []string{}
		}
	}
	payload, err := json.Marshal(settings)
	if err != nil {
		return nil, &trackingErrors.TrackingError{Code: trackingErrors.NotificationSettingsFailed, Err: err}
	}
	return s.repo.SetNotificationSettings(ctx, tenantID, userID, payload)
}
