package interfaces

import (
	"context"

	"josex/web/modules/inventory/models"

	"github.com/google/uuid"
)

type MovementService interface {
	RecordMovement(ctx context.Context, tenantID uuid.UUID, dto models.RecordMovementDto, createdBy *string) (*models.RecordMovementResponse, error)
	GetMovement(ctx context.Context, tenantID uuid.UUID, movementID string) (*models.InventoryMovement, error)
	ListMovements(ctx context.Context, tenantID uuid.UUID, query models.ListMovementsQuery) (*models.ListMovementsResponse, error)
	GetProductStock(ctx context.Context, tenantID uuid.UUID, productID string) (*models.ProductStock, error)
}
