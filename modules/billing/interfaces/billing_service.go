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

	// GetTenantPlan returns a business's plan, its limits now, the prices and where to pay (BILLING-001).
	GetTenantPlan(ctx context.Context, tenantID uuid.UUID) (*models.TenantPlan, error)
	// TenantLimit answers a business's limit for a feature, from a 60 s cache; -1 is unlimited.
	TenantLimit(ctx context.Context, tenantID uuid.UUID, feature string) (int, error)
	// GetTenantUsage answers each limited feature's use against the business's limit.
	GetTenantUsage(ctx context.Context, tenantID uuid.UUID) (*models.UsageResponse, error)
	NotifyPayment(ctx context.Context, tenantID, userID uuid.UUID, dto *models.NotifyPaymentDto) (*models.TenantPayment, error)
	// ConfirmPayment completes a notified payment; the business's cached plan is dropped.
	ConfirmPayment(ctx context.Context, paymentID, confirmedBy uuid.UUID) (*models.TenantPayment, error)
	ListNotifiedPayments(ctx context.Context) ([]models.NotifiedPayment, error)
	// ExpireSubscriptions is the daily job: paid plans past their grace go back to the free limits.
	ExpireSubscriptions(ctx context.Context) (int, error)
}
