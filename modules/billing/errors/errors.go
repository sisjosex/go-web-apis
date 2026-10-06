package errors

const (
	// Subscription errors
	SubscriptionNotFound      = "billing.subscription.not-found"
	SubscriptionInvalidPlan   = "billing.subscription.invalid-plan"
	SubscriptionInvalidStatus = "billing.subscription.invalid-status"
	SubscriptionLimitReached  = "billing.subscription.limit-reached"

	// Payment errors
	PaymentSubscriptionNotFound = "billing.payment.subscription-not-found"
	PaymentCreateFailed         = "billing.payment.create-failed"
	PaymentListFailed           = "billing.payment.list-failed"
	// A confirmation for a payment that is not a business's, or not waiting (BILLING-001 D3).
	PaymentNotFound   = "billing.payment.not-found"
	PaymentNotPending = "billing.payment.not-pending"

	// General
	BillingValidationFailed = "billing.validation-failed"
	// PlanContactSales refuses a plan change or a payment written by anyone but the platform while
	// there is no checkout: without it any account could move itself to a paid plan (APP-009 D4).
	PlanContactSales = "billing.plan.contact-sales"
)
