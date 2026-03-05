package errors

const (
	// Batch creation errors
	ProductNotFound                 = "product.not-found"
	BatchLotNumberRequired          = "batch.lot-number-required"
	BatchExpiryDateMustBeFuture     = "batch.expiry-date-must-be-future"
	BatchPurchaseDateCannotBeFuture = "batch.purchase-date-cannot-be-future"
	BatchQuantityMustBePositive     = "batch.quantity-must-be-positive"
	BatchUnitCostMustBePositive     = "batch.unit-cost-must-be-positive"

	// Batch retrieval errors
	BatchNotFound = "batch.not-found"
	BatchExpired  = "batch.expired"

	// Batch depletion errors
	InsufficientBatchQuantity = "batch.insufficient-quantity"
)
