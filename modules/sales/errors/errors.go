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
	OrderItemInvalidPrice    = "order-item.invalid-price"

	// SKU errors (INV-010) — the client named no combination on a product whose
	// stock is tracked per combination, or named one that is not this product's.
	OrderItemSkuRequired = "sales.order-item.sku-required"
	OrderItemSkuNotFound = "sales.order-item.sku-not-found"

	// Payment errors
	PaymentFailed              = "payment.failed"
	PaymentAlreadyProcessed    = "payment.already-processed"
	PaymentPartialRefundNeeded = "payment.partial-refund-needed"
	PaymentInvalidAmount       = "payment.invalid-amount"
	PaymentInvalidMethod       = "payment.invalid-method"

	// Return errors
	ReturnNotFound      = "return.not-found"
	ReturnInvalidStatus = "return.invalid-status"

	// Batch errors
	ProductBatchNotFound      = "product-batch.not-found"
	BatchInsufficientQuantity = "batch.insufficient-quantity"
)
