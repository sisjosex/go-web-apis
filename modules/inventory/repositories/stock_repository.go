package repositories

import (
	"context"
	"log"

	coreServices "josex/web/modules/core/services"
	"josex/web/modules/inventory/models"
)

type StockRepository struct {
	dbService coreServices.DatabaseService
	logger    *log.Logger
}

func NewStockRepository(dbService coreServices.DatabaseService, logger *log.Logger) *StockRepository {
	return &StockRepository{
		dbService: dbService,
		logger:    logger,
	}
}

// ReserveStock reserves inventory for a sales order
func (r *StockRepository) ReserveStock(ctx context.Context, productID string, quantity float64) (*models.StockReservationResponse, error) {
	var response models.StockReservationResponse

	err := r.dbService.QueryRow(
		ctx,
		`SELECT reserved_quantity, available_quantity, status, message 
		 FROM inventory.sp_reserve_stock_for_order($1, $2)`,
		productID,
		quantity,
	).Scan(&response.ReservedQuantity, &response.AvailableQuantity, &response.Status, &response.Message)

	if err != nil {
		if r.logger != nil {
			r.logger.Printf("❌ Error reserving stock: %v", err)
		}
		return nil, err
	}

	return &response, nil
}

// ReleaseReservedStock releases reserved inventory when order is cancelled
func (r *StockRepository) ReleaseReservedStock(ctx context.Context, productID string, quantity float64) (*models.StockReservationResponse, error) {
	var response models.StockReservationResponse

	err := r.dbService.QueryRow(
		ctx,
		`SELECT reserved_quantity, available_quantity, status, message 
		 FROM inventory.sp_release_reserved_stock($1, $2)`,
		productID,
		quantity,
	).Scan(&response.ReservedQuantity, &response.AvailableQuantity, &response.Status, &response.Message)

	if err != nil {
		if r.logger != nil {
			r.logger.Printf("❌ Error releasing reserved stock: %v", err)
		}
		return nil, err
	}

	return &response, nil
}

// UpdateReorderLevel updates the minimum stock level for a product
func (r *StockRepository) UpdateReorderLevel(ctx context.Context, productID string, reorderLevel float64) (*models.UpdateReorderLevelResponse, error) {
	var response models.UpdateReorderLevelResponse

	err := r.dbService.QueryRow(
		ctx,
		`SELECT product_id::VARCHAR, reorder_level, current_quantity, status, message 
		 FROM inventory.sp_update_reorder_level($1::UUID, $2)`,
		productID,
		reorderLevel,
	).Scan(&response.ProductID, &response.ReorderLevel, &response.CurrentQty, &response.Status, &response.Message)

	if err != nil {
		if r.logger != nil {
			r.logger.Printf("❌ Error updating reorder level: %v", err)
		}
		return nil, err
	}

	return &response, nil
}
