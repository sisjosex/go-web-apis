package interfaces

import (
	"context"

	"josex/web/modules/inventory/models"
)

type StockRepository interface {
	ReserveStock(ctx context.Context, productID string, quantity float64) (*models.StockReservationResponse, error)
	ReleaseReservedStock(ctx context.Context, productID string, quantity float64) (*models.StockReservationResponse, error)
	UpdateReorderLevel(ctx context.Context, productID string, reorderLevel float64) (*models.UpdateReorderLevelResponse, error)
}
