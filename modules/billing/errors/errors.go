package errors

const (
	// Subscription errors
	SubscriptionNotFound   = "billing.subscription.not-found"
	SubscriptionInvalidPlan   = "billing.subscription.invalid-plan"
	SubscriptionInvalidStatus = "billing.subscription.invalid-status"
	SubscriptionLimitReached  = "billing.subscription.limit-reached"

	// Payment errors
	PaymentSubscriptionNotFound = "billing.payment.subscription-not-found"
	PaymentCreateFailed         = "billing.payment.create-failed"
	PaymentListFailed           = "billing.payment.list-failed"

	// General
	BillingValidationFailed = "billing.validation-failed"
)
