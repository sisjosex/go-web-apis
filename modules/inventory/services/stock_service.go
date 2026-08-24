package services

import (
	"context"

	"josex/web/modules/inventory/interfaces"
	"josex/web/modules/inventory/models"

	"github.com/google/uuid"
)

type StockService struct {
	stockRepository interfaces.StockRepository
}

func NewStockService(stockRepository interfaces.StockRepository) *StockService {
	return &StockService{
		stockRepository: stockRepository,
	}
}

// UpdateReorderLevel sets the reorder level of one SKU — the product's default
// bucket when skuID is nil, which is what every caller but the CSV import sends.
func (s *StockService) UpdateReorderLevel(ctx context.Context, tenantID uuid.UUID, productID string, reorderLevel float64, userID *string, skuID *string) (*models.UpdateReorderLevelResponse, error) {
	return s.stockRepository.UpdateReorderLevel(ctx, tenantID, productID, reorderLevel, skuID)
}

// ReserveStock reserves inventory for a sales order
func (s *StockService) ReserveStock(ctx context.Context, tenantID uuid.UUID, productID string, quantity float64, skuID *string) (*models.StockReservationResponse, error) {
	return s.stockRepository.ReserveStock(ctx, tenantID, productID, quantity, skuID)
}

// ReleaseReservedStock releases reserved inventory when order is cancelled
func (s *StockService) ReleaseReservedStock(ctx context.Context, tenantID uuid.UUID, productID string, quantity float64, skuID *string) (*models.StockReservationResponse, error) {
	return s.stockRepository.ReleaseReservedStock(ctx, tenantID, productID, quantity, skuID)
}
