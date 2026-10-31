package errors

import (
	"net/http"

	coreErrors "josex/web/modules/core/errors"
)

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

	// Stock counts (INV-026)
	CountNotFound     = "inventory.count.not-found"
	CountAlreadyOpen  = "inventory.count.already-open"
	CountIncomplete   = "inventory.count.incomplete"
	CountInvalidState = "inventory.count.invalid-state"

	// Warehouses and transfers (INV-025)
	WarehouseNotFound       = "inventory.warehouse.not-found"
	WarehouseHasStock       = "inventory.warehouse.has-stock"
	WarehouseDefault        = "inventory.warehouse.default"
	TransferNotFound        = "inventory.transfer.not-found"
	TransferSameWarehouse   = "inventory.transfer.same-warehouse"
	TransferInvalidState    = "inventory.transfer.invalid-state"
	TransferInvalidQuantity = "inventory.transfer.invalid-quantity"

	// Attributes (INV-024)
	AttributeNotFound  = "inventory.attribute.not-found"
	AttributeInvalid   = "inventory.attribute.invalid"
	AttributeInUse     = "inventory.attribute.in-use"
	AttributeDuplicate = "inventory.attribute.duplicate"

	// Units and codes (INV-023)
	UnitNotFound     = "inventory.unit.not-found"
	UnitNoFraction   = "inventory.unit.no-fraction"
	UnitBaseRequired = "inventory.unit.base-required"
	UnitFactor       = "inventory.unit.factor-invalid"
	CodeNotFound     = "inventory.code.not-found"
	CodeDuplicate    = "inventory.code.duplicate"

	// A download past the row cap (INV-021 D1).
	ExportTooLarge = "inventory.export.too-large"

	// Stock
	InsufficientStock = "inventory.insufficient-stock"
	StockNotFound     = "inventory.stock-not-found"

	// Product-category mapping
	CategoryMappingNotFound = "inventory.category-mapping.not-found"

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

// CSV import (INV-015). Every code that quotes a cell back travels with the
// value appended after the separator the wizard splits on ("…|Bebidas").
const (
	ImportSkuRequired           = "inventory.import.sku-required"
	ImportSkuDuplicate          = "inventory.import.sku-duplicate"
	ImportSkuUnknown            = "inventory.import.sku-unknown"
	ImportNameRequired          = "inventory.import.name-required"
	ImportCategoryUnknown       = "inventory.import.category-unknown"
	ImportCategoryCreated       = "inventory.import.category-created"
	ImportCategoryDuplicate     = "inventory.import.category-duplicate"
	ImportCategoryParentUnknown = "inventory.import.category-parent-unknown"
	ImportCombinationUnknown    = "inventory.import.combination-unknown"
	ImportQuantityInvalid       = "inventory.import.quantity-invalid"
	ImportReorderLevelInvalid   = "inventory.import.reorder-level-invalid"
	ImportLotNumberRequired     = "inventory.import.lot-number-required"
	ImportDateInvalid           = "inventory.import.date-invalid"
	ImportExpiryBeforePurchase  = "inventory.import.expiry-before-purchase"
	ImportUnitCostInvalid       = "inventory.import.unit-cost-invalid"
	ImportImageMissing          = "inventory.import.image-missing"
	ImportCreateFailed          = "inventory.import.create-failed"
	ImportMediaFailed           = "inventory.import.media-failed"
	ImportCategoryAssignFailed  = "inventory.import.category-assign-failed"
	ImportStockFailed           = "inventory.import.stock-failed"
	ImportBatchFailed           = "inventory.import.batch-failed"
)

// The catalogue file: one row per sellable thing, one column per axis, no cell
// grammar (INV-016). `sku` names the product and `variant_sku` the combination,
// so the two are told apart by name rather than by context.
const (
	ImportProductSkuRequired      = "inventory.import.product-sku-required"
	ImportVariantSkuDuplicate     = "inventory.import.variant-sku-duplicate"
	ImportVariantSkuTooLong       = "inventory.import.variant-sku-too-long"
	ImportAxisColumnInvalid       = "inventory.import.axis-column-invalid"
	ImportAxesNone                = "inventory.import.axes-none"
	ImportAxesIncomplete          = "inventory.import.axes-incomplete"
	ImportAxesInconsistent        = "inventory.import.axes-inconsistent"
	ImportAxesTooMany             = "inventory.import.axes-too-many"
	ImportPriceInvalid            = "inventory.import.price-invalid"
	ImportPriceZero               = "inventory.import.price-zero"
	ImportPriceInconsistent       = "inventory.import.price-inconsistent"
	ImportPriceAttributed         = "inventory.import.price-attributed"
	ImportProductInconsistent     = "inventory.import.product-inconsistent"
	ImportOptionImageInconsistent = "inventory.import.option-image-inconsistent"
	ImportCombinationDuplicate    = "inventory.import.combination-duplicate"
	ImportCombinationFailed       = "inventory.import.combination-failed"
)

// The statuses a code's name does not tell (INV-023).
func init() {
	coreErrors.RegisterStatus(CountAlreadyOpen, http.StatusConflict)
	coreErrors.RegisterStatus(CountIncomplete, http.StatusUnprocessableEntity)
	coreErrors.RegisterStatus(CountInvalidState, http.StatusConflict)
	coreErrors.RegisterStatus(WarehouseHasStock, http.StatusConflict)
	coreErrors.RegisterStatus(WarehouseDefault, http.StatusConflict)
	coreErrors.RegisterStatus(TransferSameWarehouse, http.StatusUnprocessableEntity)
	coreErrors.RegisterStatus(TransferInvalidState, http.StatusConflict)
	coreErrors.RegisterStatus(TransferInvalidQuantity, http.StatusUnprocessableEntity)
	coreErrors.RegisterStatus(AttributeInvalid, http.StatusUnprocessableEntity)
	coreErrors.RegisterStatus(AttributeInUse, http.StatusConflict)
	coreErrors.RegisterStatus(CategoryCircularHierarchy, http.StatusBadRequest)
	coreErrors.RegisterStatus(UnitNoFraction, http.StatusUnprocessableEntity)
	coreErrors.RegisterStatus(UnitBaseRequired, http.StatusBadRequest)
	coreErrors.RegisterStatus(CodeDuplicate, http.StatusConflict)
}
