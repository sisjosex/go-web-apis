package interfaces

import (
	"context"

	"josex/web/modules/inventory/models"

	"github.com/google/uuid"
)

type MovementRepository interface {
	RecordMovement(
		ctx context.Context,
		tenantID uuid.UUID,
		productID, movementType string,
		quantity float64,
		referenceType, referenceID *string,
		unitCost *float64,
		notes, createdBy, skuID *string,
	) (*models.RecordMovementResponse, error)

	GetMovement(ctx context.Context, tenantID uuid.UUID, movementID string) (*models.InventoryMovement, error)
	ListMovements(ctx context.Context, tenantID uuid.UUID, query models.ListMovementsQuery) ([]models.InventoryMovement, int64, error)
	GetProductStock(ctx context.Context, tenantID uuid.UUID, productID string) (*models.ProductStock, error)
}
