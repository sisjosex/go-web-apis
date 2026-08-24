package interfaces

import (
	"context"

	"josex/web/modules/inventory/models"

	"github.com/google/uuid"
)

type StockRepository interface {
	ReserveStock(ctx context.Context, tenantID uuid.UUID, productID string, quantity float64, skuID *string) (*models.StockReservationResponse, error)
	ReleaseReservedStock(ctx context.Context, tenantID uuid.UUID, productID string, quantity float64, skuID *string) (*models.StockReservationResponse, error)
	UpdateReorderLevel(ctx context.Context, tenantID uuid.UUID, productID string, reorderLevel float64, skuID *string) (*models.UpdateReorderLevelResponse, error)
}
