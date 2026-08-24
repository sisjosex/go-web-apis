package interfaces

import (
	"context"

	"josex/web/modules/inventory/models"

	"github.com/google/uuid"
)

type StockService interface {
	// skuID is appended last for the reason every other SKU parameter was: each
	// existing positional call site keeps meaning what it means, and nil is the
	// normal case — the product's default bucket (INV-015). sp_update_reorder_level
	// has always taken p_sku_id; nothing in Go could reach it until now.
	UpdateReorderLevel(ctx context.Context, tenantID uuid.UUID, productID string, reorderLevel float64, userID *string, skuID *string) (*models.UpdateReorderLevelResponse, error)
	ReserveStock(ctx context.Context, tenantID uuid.UUID, productID string, quantity float64, skuID *string) (*models.StockReservationResponse, error)
	ReleaseReservedStock(ctx context.Context, tenantID uuid.UUID, productID string, quantity float64, skuID *string) (*models.StockReservationResponse, error)
}
