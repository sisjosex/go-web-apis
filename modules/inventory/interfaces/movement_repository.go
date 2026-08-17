package interfaces

import (
	"context"

	"josex/web/modules/inventory/models"

	"github.com/google/uuid"
)

type MovementRepository interface {
	// direction is appended last for the same reason skuID was: every existing
	// positional call site keeps meaning what it means, and nil is the normal case —
	// only ADJUSTMENT and TRANSFER need it (INV-013 D1).
	RecordMovement(
		ctx context.Context,
		tenantID uuid.UUID,
		productID, movementType string,
		quantity float64,
		referenceType, referenceID *string,
		unitCost *float64,
		notes, createdBy, skuID, direction *string,
	) (*models.RecordMovementResponse, error)

	GetMovement(ctx context.Context, tenantID uuid.UUID, movementID string) (*models.InventoryMovement, error)
	ListMovements(ctx context.Context, tenantID uuid.UUID, query models.ListMovementsQuery) ([]models.InventoryMovement, int64, error)
	GetProductStock(ctx context.Context, tenantID uuid.UUID, productID string) (*models.ProductStock, error)
}
