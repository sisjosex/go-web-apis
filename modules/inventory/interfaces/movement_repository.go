package interfaces

import (
	"context"

	"josex/web/modules/inventory/models"
)

type MovementRepository interface {
	RecordMovement(
		ctx context.Context,
		productID, movementType string,
		quantity float64,
		referenceType, referenceID *string,
		unitCost *float64,
		notes, createdBy *string,
	) (*models.RecordMovementResponse, error)

	GetMovement(ctx context.Context, movementID string) (*models.InventoryMovement, error)
	GetProductStock(ctx context.Context, productID string) (*models.ProductStock, error)
}
