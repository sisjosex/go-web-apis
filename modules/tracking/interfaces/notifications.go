package interfaces

import (
	"context"

	"josex/web/modules/tracking/models"

	"github.com/google/uuid"
)

// NotificationRepository is the portal's feed and settings (TRACK-012); every call is scoped to the
// account in its SP.
type NotificationRepository interface {
	ListNotifications(ctx context.Context, tenantID, userID uuid.UUID, query models.ListNotificationsQuery) (*models.ListNotificationsResponse, error)
	MarkNotificationsRead(ctx context.Context, tenantID, userID uuid.UUID, dto models.MarkNotificationsReadDto) error
	NotificationSettings(ctx context.Context, tenantID, userID uuid.UUID) ([]models.NotificationSetting, error)
	SetNotificationSettings(ctx context.Context, tenantID, userID uuid.UUID, settings []byte) ([]models.NotificationSetting, error)
}

// NotificationService is what the /mobile/portal notice endpoints call.
type NotificationService interface {
	List(ctx context.Context, tenantID, userID uuid.UUID, query models.ListNotificationsQuery) (*models.ListNotificationsResponse, error)
	MarkRead(ctx context.Context, tenantID, userID uuid.UUID, dto models.MarkNotificationsReadDto) error
	Settings(ctx context.Context, tenantID, userID uuid.UUID) ([]models.NotificationSetting, error)
	SetSettings(ctx context.Context, tenantID, userID uuid.UUID, settings []models.NotificationSetting) ([]models.NotificationSetting, error)
}
