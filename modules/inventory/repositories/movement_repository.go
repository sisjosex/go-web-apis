package repositories

import (
	"context"
	"errors"
	"log"

	"josex/web/modules/core/services"
	"josex/web/modules/inventory/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
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
	tenantID uuid.UUID,
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
	var createdByParam any
	if createdBy == nil || *createdBy == "" {
		createdByParam = nil
	} else {
		createdByParam = *createdBy
	}

	err := r.dbService.QueryRow(
		ctx,
		`SELECT movement_id, new_stock_quantity, message
		 FROM inventory.sp_record_movement($1, $2, $3, $4, $5, $6::UUID, $7::DECIMAL, $8, $9::UUID)`,
		tenantID, productID, movementType, quantity, referenceType, referenceID, unitCost, notes, createdByParam,
	).Scan(&movementID, &newStock, &message)

	if err != nil {
		if r.logger != nil {
			r.logger.Printf("❌ Error recording movement: %v", err)
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return nil, pgErr
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

func (r *MovementRepository) GetMovement(ctx context.Context, tenantID uuid.UUID, movementID string) (*models.InventoryMovement, error) {
	var movement models.InventoryMovement

	err := r.dbService.QueryRow(
		ctx,
		`SELECT id, product_id, movement_type, quantity, reference_type, reference_id, unit_cost, notes, created_by, created_at
		 FROM inventory.sp_get_movement($1, $2)`,
		tenantID, movementID,
	).Scan(&movement.ID, &movement.ProductID, &movement.MovementType, &movement.Quantity, &movement.ReferenceType, &movement.ReferenceID, &movement.UnitCost, &movement.Notes, &movement.CreatedBy, &movement.CreatedAt)

	if err != nil {
		if r.logger != nil {
			r.logger.Printf("❌ Error getting movement: %v", err)
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return nil, pgErr
		}
		return nil, err
	}

	return &movement, nil
}

// ListMovements returns one page of the audit trail plus the total the same filters
// match. total_count comes back on every row, so it is read from whichever row is
// scanned last and stays 0 when the page is empty.
func (r *MovementRepository) ListMovements(
	ctx context.Context,
	tenantID uuid.UUID,
	query models.ListMovementsQuery,
) ([]models.InventoryMovement, int64, error) {
	rows, err := r.dbService.Query(
		ctx,
		`SELECT id, product_id, movement_type, quantity, reference_type, reference_id, unit_cost, notes, created_by, created_at, total_count
		 FROM inventory.sp_list_movements($1, $2::UUID, $3::VARCHAR, $4::DATE, $5::DATE, $6, $7)`,
		tenantID, query.ProductID, query.MovementType, query.DateFrom, query.DateTo, query.Page, query.PageSize,
	)
	if err != nil {
		if r.logger != nil {
			r.logger.Printf("❌ Error listing movements: %v", err)
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return nil, 0, pgErr
		}
		return nil, 0, err
	}
	defer rows.Close()

	movements := []models.InventoryMovement{}
	var totalCount int64
	for rows.Next() {
		var m models.InventoryMovement
		if err := rows.Scan(
			&m.ID,
			&m.ProductID,
			&m.MovementType,
			&m.Quantity,
			&m.ReferenceType,
			&m.ReferenceID,
			&m.UnitCost,
			&m.Notes,
			&m.CreatedBy,
			&m.CreatedAt,
			&totalCount,
		); err != nil {
			return nil, 0, err
		}
		movements = append(movements, m)
	}
	if err = rows.Err(); err != nil {
		return nil, 0, err
	}

	return movements, totalCount, nil
}

func (r *MovementRepository) GetProductStock(ctx context.Context, tenantID uuid.UUID, productID string) (*models.ProductStock, error) {
	var stock models.ProductStock

	err := r.dbService.QueryRow(
		ctx,
		`SELECT current_quantity, reserved_quantity, available_quantity, reorder_level, status, last_updated_at
		 FROM inventory.sp_get_product_stock($1, $2)`,
		tenantID, productID,
	).Scan(&stock.CurrentQuantity, &stock.ReservedQuantity, &stock.AvailableQuantity, &stock.ReorderLevel, &stock.Status, &stock.LastUpdatedAt)

	if err != nil {
		if r.logger != nil {
			r.logger.Printf("❌ Error getting product stock: %v", err)
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return nil, pgErr
		}
		return nil, err
	}

	return &stock, nil
}
