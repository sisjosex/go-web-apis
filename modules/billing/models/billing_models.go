package models

import (
	"time"

	"github.com/google/uuid"
)

// Subscription represents a user's billing subscription.
type Subscription struct {
	ID               uuid.UUID  `json:"id"`
	UserID           uuid.UUID  `json:"user_id"`
	Plan             string     `json:"plan"`    // free | pro | enterprise
	Status           string     `json:"status"`  // active | expired | canceled | trial
	StartedAt        time.Time  `json:"started_at"`
	ExpiresAt        *time.Time `json:"expires_at"`
	CanceledAt       *time.Time `json:"canceled_at"`
	StripeSubID      *string    `json:"stripe_sub_id,omitempty"`
	StripeCustomerID *string    `json:"stripe_customer_id,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

// PlanLimit is a single feature → limit mapping for a plan.
type PlanLimit struct {
	Feature    string `json:"feature"`
	LimitValue int    `json:"limit_value"` // -1 = unlimited
}

// PlanInfo bundles the active subscription header with all feature limits.
// When no subscription exists the plan defaults to "free" and status to "none".
type PlanInfo struct {
	Plan      string      `json:"plan"`
	Status    string      `json:"status"`
	ExpiresAt *time.Time  `json:"expires_at"`
	Limits    []PlanLimit `json:"limits"`
	IsActive  bool        `json:"is_active"`
	IsExpired bool        `json:"is_expired"`
}

// Payment is a single payment record.
type Payment struct {
	ID                uuid.UUID  `json:"id"`
	SubscriptionID    uuid.UUID  `json:"subscription_id"`
	Amount            float64    `json:"amount"`
	Currency          string     `json:"currency"`
	Status            string     `json:"status"`
	Provider          *string    `json:"provider"`
	ProviderPaymentID *string    `json:"provider_payment_id"`
	PaidAt            *time.Time `json:"paid_at"`
	PeriodStart       *time.Time `json:"period_start"`
	PeriodEnd         *time.Time `json:"period_end"`
	CreatedAt         time.Time  `json:"created_at"`
}

// PaymentListResponse wraps a paginated list of payments.
type PaymentListResponse struct {
	Data       []Payment `json:"data"`
	TotalCount int64     `json:"total_count"`
	Page       int       `json:"page"`
	Limit      int       `json:"limit"`
}

// UpsertSubscriptionDto is the request body for creating or upgrading a subscription.
type UpsertSubscriptionDto struct {
	Plan             string     `json:"plan"   binding:"required,oneof=free pro enterprise"`
	Status           string     `json:"status" binding:"omitempty,oneof=active expired canceled trial"`
	ExpiresAt        *time.Time `json:"expires_at"`
	StripeSubID      *string    `json:"stripe_sub_id"`
	StripeCustomerID *string    `json:"stripe_customer_id"`
}

// RecordPaymentDto is the request body for recording a payment.
type RecordPaymentDto struct {
	SubscriptionID    uuid.UUID  `json:"subscription_id"     binding:"required"`
	Amount            float64    `json:"amount"              binding:"required,gt=0"`
	Currency          string     `json:"currency"            binding:"omitempty,len=3"`
	Status            string     `json:"status"              binding:"omitempty,oneof=pending completed failed refunded"`
	Provider          *string    `json:"provider"`
	ProviderPaymentID *string    `json:"provider_payment_id"`
	PeriodStart       *time.Time `json:"period_start"`
	PeriodEnd         *time.Time `json:"period_end"`
}
