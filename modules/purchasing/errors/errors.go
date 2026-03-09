package errors

// Purchasing module error constants
const (
	// Supplier errors
	SupplierNotFound    = "supplier.not-found"
	SupplierCreateError = "supplier.create-error"
	SupplierUpdateError = "supplier.update-error"
	SupplierDeleteError = "supplier.delete-error"
	SupplierInvalidData = "supplier.invalid-data"

	// Purchase Order errors
	PONotFound        = "po.not-found"
	POCreateError     = "po.create-error"
	POCannotApprove   = "po.cannot-approve"
	POCannotReceive   = "po.cannot-receive"
	POInvalidStatus   = "po.invalid-status"
	PONoItems         = "po.no-items"
	POInvalidQuantity = "po.invalid-quantity"
	POInvalidCost     = "po.invalid-cost"

	// Invoice errors
	InvoiceNotFound        = "invoice.not-found"
	InvoiceAddError        = "invoice.add-error"
	InvoiceInvalidAmount   = "invoice.invalid-amount"
	InvoiceDuplicateNumber = "invoice.duplicate-number"

	// Phase 2A: Product errors
	ProductNotFound  = "product.not-found"
	InvalidProductID = "product.invalid-id"
	BatchNotFound    = "batch.not-found"

	// Phase 2A: RFQ errors
	RFQNotFound         = "rfq.not-found"
	RFQCreateError      = "rfq.create-error"
	RFQInvalidSuppliers = "rfq.invalid-suppliers"
	RFQResponseNotFound = "rfq.response.not-found"
	RFQInvalidItems     = "rfq.invalid-items"
)
