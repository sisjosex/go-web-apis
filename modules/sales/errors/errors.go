package errors

// Sales module error constants
const (
	// Customer errors
	CustomerNotFound           = "customer.not-found"
	CustomerAlreadyExists      = "customer.already-exists"
	CustomerInvalidEmail       = "customer.invalid-email"
	CustomerInvalidPhoneNumber = "customer.invalid-phone-number"

	// Sales Order errors
	SalesOrderNotFound              = "sales-order.not-found"
	SalesOrderInvalidStatus         = "sales-order.invalid-status"
	SalesOrderCannotModify          = "sales-order.cannot-modify"
	SalesOrderEmptyItems            = "sales-order.empty-items"
	SalesOrderExceededMaxItems      = "sales-order.exceeded-max-items"
	SalesOrderInsufficientInventory = "sales-order.insufficient-inventory"

	// Order Item errors
	OrderItemNotFound        = "order-item.not-found"
	OrderItemInvalidQty      = "order-item.invalid-qty"
	OrderItemProductNotFound = "order-item.product-not-found"

	// Payment errors
	PaymentFailed              = "payment.failed"
	PaymentAlreadyProcessed    = "payment.already-processed"
	PaymentPartialRefundNeeded = "payment.partial-refund-needed"
)
