package interfaces

import (
	"context"

	"josex/web/modules/billing/models"

	"github.com/google/uuid"
)

// BillingRepository defines data-access operations for the billing module.
type BillingRepository interface {
	// GetSubscription returns the most recent subscription for a user (active/trial first).
	// Returns nil, nil when the user has no subscription.
	GetSubscription(ctx context.Context, userID uuid.UUID) (*models.Subscription, error)

	// UpsertSubscription creates or updates the active subscription for a user.
	UpsertSubscription(ctx context.Context, userID uuid.UUID, dto *models.UpsertSubscriptionDto) (*models.Subscription, error)

	// GetPlanInfo returns the resolved plan + all feature limits for a user.
	GetPlanInfo(ctx context.Context, userID uuid.UUID) (*models.PlanInfo, error)

	// GetFeatureLimit returns the limit value for a specific feature given the user's active plan.
	// Returns -1 for unlimited, 0+ for a hard cap.
	GetFeatureLimit(ctx context.Context, userID uuid.UUID, feature string) (int, error)

	// RecordPayment inserts a payment record and returns it.
	RecordPayment(ctx context.Context, userID uuid.UUID, dto *models.RecordPaymentDto) (*models.Payment, error)

	// ListPayments returns paginated payment history for a user.
	ListPayments(ctx context.Context, userID uuid.UUID, page, limit int) (*models.PaymentListResponse, error)

	// Business plans (BILLING-001).
	GetTenantPlan(ctx context.Context, tenantID uuid.UUID) (*models.TenantPlan, error)
	NotifyPayment(ctx context.Context, tenantID, userID uuid.UUID, dto *models.NotifyPaymentDto) (*models.TenantPayment, error)
	ConfirmPayment(ctx context.Context, paymentID, confirmedBy uuid.UUID) (*models.TenantPayment, error)
	ListNotifiedPayments(ctx context.Context) ([]models.NotifiedPayment, error)
	ExpireSubscriptions(ctx context.Context, graceDays int) (int, error)
	GetTenantUsage(ctx context.Context, tenantID uuid.UUID) (map[string]int64, error)
}
