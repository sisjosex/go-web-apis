package errors

const (
	// Products
	ProductNotFound         = "product.not-found"
	ProductSkuAlreadyExists = "product.sku.already-exists"

	// Stock
	InsufficientStock = "inventory.insufficient-stock"

	// Movements
	InvalidMovementType = "inventory.invalid-movement-type"
	MovementNotFound    = "inventory.movement.not-found"
)
