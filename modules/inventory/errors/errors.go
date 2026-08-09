package errors

const (
	// Products
	ProductNotFound         = "product.not-found"
	ProductSkuAlreadyExists = "product.sku.already-exists"

	// Product variants
	ProductVariantGroupDuplicateType  = "product.variant-group.duplicate-type"
	ProductVariantOptionDuplicateName = "product.variant-option.duplicate-name"
	ProductVariantGroupNotInProduct   = "product.variant-group.not-in-product"
	ProductVariantOptionNotInGroup    = "product.variant-option.not-in-group"

	// Stock
	InsufficientStock = "inventory.insufficient-stock"

	// Movements
	InvalidMovementType = "inventory.invalid-movement-type"
	MovementNotFound    = "inventory.movement.not-found"

	// Batch creation errors
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

	// Categories
	CategoryNotFound          = "category.not-found"
	CategoryNameRequired      = "category.name-required"
	CategorySlugRequired      = "category.slug-required"
	CategorySlugAlreadyExists = "category.slug-already-exists"
	CategoryParentNotFound    = "category.parent-not-found"
	CategoryCircularHierarchy = "category.circular-hierarchy"
	CategoryHasProducts       = "category.has-products"
	ProductAlreadyInCategory  = "product.already-in-category"
)
