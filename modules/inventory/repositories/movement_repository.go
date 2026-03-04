package repositories

import (
	"context"
	"log"

	"josex/web/modules/core/services"
	"josex/web/modules/inventory/models"
)

type MovementRepository struct {
	dbService services.DatabaseService
	logger    *log.Logger
}

func NewMovementRepository(dbService services.DatabaseService, logger *log.Logger) *MovementRepository {
	return &MovementRepository{
		dbService: dbService,
		logger:    logger,
	}
}

func (r *MovementRepository) RecordMovement(
	ctx context.Context,
	productID, movementType string,
	quantity float64,
	referenceType, referenceID *string,
	unitCost *float64,
	notes, createdBy *string,
) (*models.RecordMovementResponse, error) {
	var movementID string
	var newStock float64
	var message string

	// Handle nil or empty createdBy
	var createdByParam interface{}
	if createdBy == nil || *createdBy == "" {
		createdByParam = nil
	} else {
		createdByParam = *createdBy
	}

	err := r.dbService.QueryRow(
		ctx,
		`SELECT movement_id, new_stock_quantity, message 
		 FROM inventory.sp_record_movement($1, $2, $3, $4, $5::UUID, $6::DECIMAL, $7, $8::UUID)`,
		productID, movementType, quantity, referenceType, referenceID, unitCost, notes, createdByParam,
	).Scan(&movementID, &newStock, &message)

	if err != nil {
		if r.logger != nil {
			r.logger.Printf("❌ Error recording movement: %v", err)
		}
		return nil, err
	}

	if r.logger != nil {
		r.logger.Printf("✅ Movement recorded: %s (qty: %v, new stock: %v)", movementID, quantity, newStock)
	}
	return &models.RecordMovementResponse{
		MovementID:       movementID,
		NewStockQuantity: newStock,
		Message:          message,
	}, nil
}

func (r *MovementRepository) GetMovement(ctx context.Context, movementID string) (*models.InventoryMovement, error) {
	var movement models.InventoryMovement

	err := r.dbService.QueryRow(
		ctx,
		`SELECT id, product_id, movement_type, quantity, reference_type, reference_id, unit_cost, notes, created_by, created_at
		 FROM inventory.sp_get_movement($1)`,
		movementID,
	).Scan(&movement.ID, &movement.ProductID, &movement.MovementType, &movement.Quantity, &movement.ReferenceType, &movement.ReferenceID, &movement.UnitCost, &movement.Notes, &movement.CreatedBy, &movement.CreatedAt)

	if err != nil {
		if r.logger != nil {
			r.logger.Printf("❌ Error getting movement: %v", err)
		}
		return nil, err
	}

	return &movement, nil
}

func (r *MovementRepository) GetProductStock(ctx context.Context, productID string) (*models.ProductStock, error) {
	var stock models.ProductStock

	err := r.dbService.QueryRow(
		ctx,
		`SELECT current_quantity, reserved_quantity, available_quantity, reorder_level, status, last_updated_at 
		 FROM inventory.sp_get_product_stock($1)`,
		productID,
	).Scan(&stock.CurrentQuantity, &stock.ReservedQuantity, &stock.AvailableQuantity, &stock.ReorderLevel, &stock.Status, &stock.LastUpdatedAt)

	if err != nil {
		if r.logger != nil {
			r.logger.Printf("❌ Error getting product stock: %v", err)
		}
		return nil, err
	}

	return &stock, nil
}
