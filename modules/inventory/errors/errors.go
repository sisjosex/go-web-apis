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

	// An axis contributes exactly one option to every combination, so INV-007
	// pinned it to max_selections = 1 and is_required = TRUE.
	ProductVariantGroupAxisNotSingleChoice = "product.variant-group.axis-not-single-choice"

	// SKUs — the sellable combinations (INV-008)
	SkuNotFound                = "inventory.sku.not-found"
	SkuCombinationKeyMismatch  = "inventory.sku.combination-key-mismatch"
	SkusNoAxes                 = "inventory.skus.no-axes"
	SkusCapExceeded            = "inventory.skus.cap-exceeded"
	SkusSkuCollision           = "inventory.skus.sku-collision"
	SkusAxisHasStock           = "inventory.skus.axis-has-stock"
	SkusAxisHasHistory         = "inventory.skus.axis-has-history"
	SkusRedistributionMismatch = "inventory.skus.redistribution-mismatch"
	SkusRedistributionReserved = "inventory.skus.redistribution-reserved"

	// Stock
	InsufficientStock = "inventory.insufficient-stock"

	// Movements
	InvalidMovementType = "inventory.invalid-movement-type"
	MovementNotFound    = "inventory.movement.not-found"

	// A product whose stock lives on combinations has no product-level quantity to
	// move or hold: the write must name a SKU instead of defaulting to the
	// unassigned bucket (INV-011 D1).
	MovementSkuRequired = "inventory.movement.sku-required"
	ReserveSkuRequired  = "inventory.reserve.sku-required"

	// The movement type owns the sign (INV-013 D1). ADJUSTMENT and TRANSFER are the
	// two whose direction is a real question, so they raise the first; the other five
	// decide it themselves and raise the second for a direction that contradicts them.
	MovementDirectionRequired = "inventory.movement.direction-required"
	MovementDirectionConflict = "inventory.movement.direction-conflict"

	// unit_cost is what was paid, so it is required exactly where something was
	// bought — PURCHASE and PRODUCTION (INV-013 D2/D3).
	MovementUnitCostRequired = "inventory.movement.unit-cost-required"

	// Creating a lot on a product stocked by variant must name the combination:
	// the lot would otherwise land in the unassigned bucket, where
	// sp_add_order_item can never reach it (INV-014 D1).
	SkuRequired = "inventory.sku.required"

	// A lot that has been consumed is booked COGS. Correcting or voiding it would
	// rewrite money already recorded against sales (INV-014 D3).
	BatchHasMovements = "inventory.batch.has-movements"
	BatchVoided       = "inventory.batch.voided"

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
