package services

import (
	"context"

	"josex/web/modules/inventory/interfaces"
	"josex/web/modules/inventory/models"
)

type StockService struct {
	stockRepository interfaces.StockRepository
}

func NewStockService(stockRepository interfaces.StockRepository) *StockService {
	return &StockService{
		stockRepository: stockRepository,
	}
}

// UpdateReorderLevel updates the minimum reorder level for a product
func (s *StockService) UpdateReorderLevel(ctx context.Context, productID string, reorderLevel float64, userID *string) (*models.UpdateReorderLevelResponse, error) {
	return s.stockRepository.UpdateReorderLevel(ctx, productID, reorderLevel)
}

// ReserveStock reserves inventory for a sales order
func (s *StockService) ReserveStock(ctx context.Context, productID string, quantity float64) (*models.StockReservationResponse, error) {
	return s.stockRepository.ReserveStock(ctx, productID, quantity)
}

// ReleaseReservedStock releases reserved inventory when order is cancelled
func (s *StockService) ReleaseReservedStock(ctx context.Context, productID string, quantity float64) (*models.StockReservationResponse, error) {
	return s.stockRepository.ReleaseReservedStock(ctx, productID, quantity)
}
