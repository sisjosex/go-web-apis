package models

import (
	"time"

	"github.com/google/uuid"
)

// Subscription represents a user's billing subscription.
type Subscription struct {
	ID               uuid.UUID  `json:"id"`
	UserID           uuid.UUID  `json:"user_id"`
	Plan             string     `json:"plan"`   // free | pro | enterprise
	Status           string     `json:"status"` // active | expired | canceled | trial
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

// ── Business plans (BILLING-001) ──────────────────────────────────────────────

// PlanPrice is what a plan costs per cycle.
type PlanPrice struct {
	Plan     string  `json:"plan"`
	Cycle    string  `json:"cycle"` // monthly | annual
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
}

// PaymentInstructions is where a customer pays (D3): the platform's bank QR and account, from config.
type PaymentInstructions struct {
	QRImageURL string `json:"qr_image_url"`
	Bank       string `json:"bank"`
	Account    string `json:"account"`
	Holder     string `json:"holder"`
}

// TenantPlan is a business's plan: what it is on, until when, and the limits it has now — the free
// plan's once expired (D1, D3).
type TenantPlan struct {
	Plan          string              `json:"plan"`
	Cycle         string              `json:"cycle"`
	Status        string              `json:"status"`
	PeriodEnd     *time.Time          `json:"period_end"`
	EffectivePlan string              `json:"effective_plan"`
	Limits        map[string]int      `json:"limits"`
	Prices        []PlanPrice         `json:"prices"`
	Payment       PaymentInstructions `json:"payment"`
}

// TenantPayment is a business's payment: notified by the customer, completed by the platform.
type TenantPayment struct {
	ID          uuid.UUID  `json:"id"`
	TenantID    uuid.UUID  `json:"tenant_id"`
	UserID      *uuid.UUID `json:"user_id"`
	Plan        string     `json:"plan"`
	Cycle       string     `json:"cycle"`
	Amount      float64    `json:"amount"`
	Currency    string     `json:"currency"`
	Status      string     `json:"status"`
	Reference   *string    `json:"reference"`
	PaidAt      *time.Time `json:"paid_at"`
	PeriodStart *time.Time `json:"period_start"`
	PeriodEnd   *time.Time `json:"period_end"`
	CreatedAt   time.Time  `json:"created_at"`
}

// NotifiedPayment is a row of the platform's confirmation queue.
type NotifiedPayment struct {
	ID         uuid.UUID  `json:"id"`
	TenantID   uuid.UUID  `json:"tenant_id"`
	TenantName string     `json:"tenant_name"`
	UserID     *uuid.UUID `json:"user_id"`
	Plan       string     `json:"plan"`
	Cycle      string     `json:"cycle"`
	Amount     float64    `json:"amount"`
	Currency   string     `json:"currency"`
	Reference  *string    `json:"reference"`
	CreatedAt  time.Time  `json:"created_at"`
}

// NotifyPaymentDto is "Ya pagué" (D3): the plan and cycle paid for and the bank reference.
type NotifyPaymentDto struct {
	Plan      string `json:"plan" binding:"required,oneof=pro"`
	Cycle     string `json:"cycle" binding:"required,oneof=monthly annual"`
	Reference string `json:"reference" binding:"required,min=3,max=255" conform:"trim"`
}

// UsageFeature is how much of one limited feature a business uses.
type UsageFeature struct {
	Feature string `json:"feature"`
	Used    int64  `json:"used"`
	Limit   int    `json:"limit"` // -1 = unlimited
}

// UsageResponse is GET /billing/usage.
type UsageResponse struct {
	Features []UsageFeature `json:"features"`
}
