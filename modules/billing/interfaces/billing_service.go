package interfaces

import (
	"context"

	"josex/web/modules/billing/models"

	"github.com/google/uuid"
)

// BillingService defines the business operations for the billing module.
type BillingService interface {
	// GetSubscription returns the user's current subscription, or nil if none.
	GetSubscription(ctx context.Context, userID uuid.UUID) (*models.Subscription, error)

	// UpsertSubscription creates a new subscription or upgrades/renews an existing one.
	UpsertSubscription(ctx context.Context, userID uuid.UUID, dto *models.UpsertSubscriptionDto) (*models.Subscription, error)

	// GetPlanInfo returns the resolved plan, status, expiry, and all feature limits.
	GetPlanInfo(ctx context.Context, userID uuid.UUID) (*models.PlanInfo, error)

	// GetFeatureLimit returns the effective limit for a feature (-1 = unlimited).
	GetFeatureLimit(ctx context.Context, userID uuid.UUID, feature string) (int, error)

	// IsLimitReached returns true when the current count meets or exceeds the plan limit.
	IsLimitReached(ctx context.Context, userID uuid.UUID, feature string, currentCount int) (bool, error)

	// RecordPayment inserts a payment record.
	RecordPayment(ctx context.Context, userID uuid.UUID, dto *models.RecordPaymentDto) (*models.Payment, error)

	// ListPayments returns paginated payment history.
	ListPayments(ctx context.Context, userID uuid.UUID, page, limit int) (*models.PaymentListResponse, error)
}
