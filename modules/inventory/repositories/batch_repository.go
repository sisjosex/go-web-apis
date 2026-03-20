package repositories

import (
	"context"
	"database/sql"
	"errors"
	"time"

	coreServices "josex/web/modules/core/services"
	"josex/web/modules/inventory/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type BatchRepository struct {
	dbService coreServices.DatabaseService
}

func NewBatchRepository(dbService coreServices.DatabaseService) *BatchRepository {
	return &BatchRepository{dbService: dbService}
}

// CreateBatch creates a new batch for a product
func (r *BatchRepository) CreateBatch(ctx context.Context, tenantID uuid.UUID, batch *models.CreateBatchDto) (*models.BatchResponse, error) {
	productID, err := uuid.Parse(batch.ProductID)
	if err != nil {
		return nil, err
	}

	row := r.dbService.QueryRow(ctx,
		"SELECT id, product_id, lot_number, purchase_date, expiry_date, unit_cost, initial_quantity, current_quantity, status, message FROM inventory.sp_create_batch($1, $2, $3, $4, $5, $6, $7)",
		tenantID,
		productID,
		batch.LotNumber,
		time.Time(batch.PurchaseDate),
		time.Time(batch.ExpiryDate),
		batch.UnitCost,
		batch.InitialQuantity,
	)

	var response models.BatchResponse
	if err := row.Scan(
		&response.ID,
		&response.ProductID,
		&response.LotNumber,
		&response.PurchaseDate,
		&response.ExpiryDate,
		&response.UnitCost,
		&response.InitialQuantity,
		&response.CurrentQuantity,
		&response.Status,
		&response.Message,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return &response, nil
}

// GetBatch retrieves a batch by ID
func (r *BatchRepository) GetBatch(ctx context.Context, tenantID uuid.UUID, batchID uuid.UUID) (*models.BatchResponse, error) {
	row := r.dbService.QueryRow(ctx,
		"SELECT id, product_id, lot_number, purchase_date, expiry_date, unit_cost, initial_quantity, current_quantity, status, days_to_expiry FROM inventory.sp_get_batch($1, $2)",
		tenantID,
		batchID,
	)

	var response models.BatchResponse
	var daysToExpiry sql.NullInt64

	if err := row.Scan(
		&response.ID,
		&response.ProductID,
		&response.LotNumber,
		&response.PurchaseDate,
		&response.ExpiryDate,
		&response.UnitCost,
		&response.InitialQuantity,
		&response.CurrentQuantity,
		&response.Status,
		&daysToExpiry,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	if daysToExpiry.Valid {
		val := int(daysToExpiry.Int64)
		response.DaysToExpiry = &val
	}

	return &response, nil
}

// ListBatchesByProduct retrieves all batches for a product
func (r *BatchRepository) ListBatchesByProduct(ctx context.Context, tenantID uuid.UUID, productID uuid.UUID, onlyActive bool) (*models.ListBatchesResponse, error) {
	rows, err := r.dbService.Query(ctx,
		"SELECT id, product_id, lot_number, purchase_date, expiry_date, unit_cost, initial_quantity, current_quantity, status, days_to_expiry FROM inventory.sp_list_batches_by_product($1, $2, $3)",
		tenantID,
		productID,
		onlyActive,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var batches []models.BatchResponse

	for rows.Next() {
		var batch models.BatchResponse
		var daysToExpiry sql.NullInt64

		if err := rows.Scan(
			&batch.ID,
			&batch.ProductID,
			&batch.LotNumber,
			&batch.PurchaseDate,
			&batch.ExpiryDate,
			&batch.UnitCost,
			&batch.InitialQuantity,
			&batch.CurrentQuantity,
			&batch.Status,
			&daysToExpiry,
		); err != nil {
			return nil, err
		}

		if daysToExpiry.Valid {
			val := int(daysToExpiry.Int64)
			batch.DaysToExpiry = &val
		}

		batches = append(batches, batch)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &models.ListBatchesResponse{
		Batches: batches,
		Count:   len(batches),
		Message: "Batches retrieved successfully",
	}, nil
}

// GetOldestBatchForSale retrieves the oldest active batch (FIFO)
func (r *BatchRepository) GetOldestBatchForSale(ctx context.Context, tenantID uuid.UUID, productID uuid.UUID) (*models.BatchResponse, error) {
	row := r.dbService.QueryRow(ctx,
		"SELECT id, product_id, lot_number, purchase_date, expiry_date, unit_cost, current_quantity, status, message FROM inventory.sp_get_oldest_batch_for_sale($1, $2)",
		tenantID,
		productID,
	)

	var response models.BatchResponse
	if err := row.Scan(
		&response.ID,
		&response.ProductID,
		&response.LotNumber,
		&response.PurchaseDate,
		&response.ExpiryDate,
		&response.UnitCost,
		&response.CurrentQuantity,
		&response.Status,
		&response.Message,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return &response, nil
}

// GetExpiringBatches retrieves all batches expiring within the specified number of days
func (r *BatchRepository) GetExpiringBatches(ctx context.Context, tenantID uuid.UUID, warningDays int) ([]*models.BatchResponse, error) {
	rows, err := r.dbService.Query(ctx,
		"SELECT id, product_id, lot_number, purchase_date, expiry_date, unit_cost, initial_quantity, current_quantity, status, days_to_expiry FROM inventory.sp_get_expiring_batches($1, $2)",
		tenantID,
		warningDays,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var batches []*models.BatchResponse

	for rows.Next() {
		var batch models.BatchResponse
		var daysToExpiry sql.NullInt64

		if err := rows.Scan(
			&batch.ID,
			&batch.ProductID,
			&batch.LotNumber,
			&batch.PurchaseDate,
			&batch.ExpiryDate,
			&batch.UnitCost,
			&batch.InitialQuantity,
			&batch.CurrentQuantity,
			&batch.Status,
			&daysToExpiry,
		); err != nil {
			return nil, err
		}

		if daysToExpiry.Valid {
			val := int(daysToExpiry.Int64)
			batch.DaysToExpiry = &val
		}

		batches = append(batches, &batch)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return batches, nil
}

// UpdateBatchQuantity updates the quantity in a batch
func (r *BatchRepository) UpdateBatchQuantity(ctx context.Context, tenantID uuid.UUID, batchID uuid.UUID, quantityUsed float64) error {
	// This will be called after a sale to reduce batch quantity
	// Implementation will depend on final SP design
	return nil
}
