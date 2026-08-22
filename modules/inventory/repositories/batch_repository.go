package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
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

	// Stays NULL when the caller named no combination: the SP resolves it to the
	// product default, or refuses when the product is stocked by variant.
	var skuID *uuid.UUID
	if batch.SkuID != nil && *batch.SkuID != "" {
		parsed, err := uuid.Parse(*batch.SkuID)
		if err != nil {
			return nil, err
		}
		skuID = &parsed
	}

	row := r.dbService.QueryRow(ctx,
		"SELECT id, product_id, lot_number, purchase_date, expiry_date, unit_cost, initial_quantity, current_quantity, status, message FROM inventory.sp_create_batch($1, $2, $3, $4, $5, $6, $7, $8)",
		tenantID,
		productID,
		batch.LotNumber,
		time.Time(batch.PurchaseDate),
		time.Time(batch.ExpiryDate),
		batch.UnitCost,
		batch.InitialQuantity,
		skuID,
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
func (r *BatchRepository) ListBatchesByProduct(ctx context.Context, tenantID uuid.UUID, productID uuid.UUID, onlyActive bool, skuID *uuid.UUID) (*models.ListBatchesResponse, error) {
	rows, err := r.dbService.Query(ctx,
		"SELECT id, product_id, lot_number, purchase_date, expiry_date, unit_cost, initial_quantity, current_quantity, status, days_to_expiry, sku_id, sku FROM inventory.sp_list_batches_by_product($1, $2, $3, $4)",
		tenantID,
		productID,
		onlyActive,
		skuID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var batches []models.BatchResponse

	for rows.Next() {
		var batch models.BatchResponse
		var daysToExpiry sql.NullInt64
		var skuIDValue uuid.UUID
		var skuValue string

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
			&skuIDValue,
			&skuValue,
		); err != nil {
			return nil, err
		}

		if daysToExpiry.Valid {
			val := int(daysToExpiry.Int64)
			batch.DaysToExpiry = &val
		}

		batch.SkuID = &skuIDValue
		batch.Sku = &skuValue

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
func (r *BatchRepository) GetOldestBatchForSale(ctx context.Context, tenantID uuid.UUID, productID uuid.UUID, skuID *uuid.UUID) (*models.BatchResponse, error) {
	row := r.dbService.QueryRow(ctx,
		"SELECT id, product_id, lot_number, purchase_date, expiry_date, unit_cost, current_quantity, status, message FROM inventory.sp_get_oldest_batch_for_sale($1, $2, $3)",
		tenantID,
		productID,
		skuID,
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
		"SELECT id, product_id, lot_number, purchase_date, expiry_date, unit_cost, initial_quantity, current_quantity, status, days_to_expiry, product_name, sku_id, sku FROM inventory.sp_get_expiring_batches($1, $2)",
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
		var productName sql.NullString
		var skuIDValue uuid.UUID
		var skuValue string

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
			&productName,
			&skuIDValue,
			&skuValue,
		); err != nil {
			return nil, err
		}

		batch.SkuID = &skuIDValue
		batch.Sku = &skuValue

		if daysToExpiry.Valid {
			val := int(daysToExpiry.Int64)
			batch.DaysToExpiry = &val
		}

		if productName.Valid {
			val := productName.String
			batch.ProductName = &val
		}

		batches = append(batches, &batch)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return batches, nil
}

// UpdateBatch corrects a lot's lot_number and expiry_date (INV-014 D3). The SP
// owns every guard: not-found, already void, and the has-movements refusal.
func (r *BatchRepository) UpdateBatch(ctx context.Context, tenantID uuid.UUID, batchID uuid.UUID, dto *models.UpdateBatchDto) (*models.BatchResponse, error) {
	var lotNumber *string
	if dto.LotNumber != nil {
		lotNumber = dto.LotNumber
	}

	var expiryDate *time.Time
	if dto.ExpiryDate != nil {
		parsed := time.Time(*dto.ExpiryDate)
		expiryDate = &parsed
	}

	row := r.dbService.QueryRow(ctx,
		"SELECT id, product_id, sku_id, sku, lot_number, purchase_date, expiry_date, unit_cost, initial_quantity, current_quantity, status, days_to_expiry, message FROM inventory.sp_update_batch($1, $2, $3, $4)",
		tenantID,
		batchID,
		lotNumber,
		expiryDate,
	)

	var response models.BatchResponse
	var skuIDValue uuid.UUID
	var skuValue string
	var daysToExpiry sql.NullInt64

	if err := row.Scan(
		&response.ID,
		&response.ProductID,
		&skuIDValue,
		&skuValue,
		&response.LotNumber,
		&response.PurchaseDate,
		&response.ExpiryDate,
		&response.UnitCost,
		&response.InitialQuantity,
		&response.CurrentQuantity,
		&response.Status,
		&daysToExpiry,
		&response.Message,
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

	response.SkuID = &skuIDValue
	response.Sku = &skuValue

	return &response, nil
}

// VoidBatch writes a lot off with status = 'void'. Never a DELETE — the row is
// pointed at by inventory_movements and sales.order_batch_assignments.
func (r *BatchRepository) VoidBatch(ctx context.Context, tenantID uuid.UUID, batchID uuid.UUID) (*models.BatchResponse, error) {
	row := r.dbService.QueryRow(ctx,
		"SELECT id, product_id, sku_id, sku, lot_number, purchase_date, expiry_date, unit_cost, initial_quantity, current_quantity, status, message FROM inventory.sp_void_batch($1, $2)",
		tenantID,
		batchID,
	)

	var response models.BatchResponse
	var skuIDValue uuid.UUID
	var skuValue string

	if err := row.Scan(
		&response.ID,
		&response.ProductID,
		&skuIDValue,
		&skuValue,
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

	response.SkuID = &skuIDValue
	response.Sku = &skuValue

	return &response, nil
}

// RedistributeBatches splits one default-bucket lot onto real combinations
// (INV-014 D2). The SP owns every guard; the children come back in SKU order.
func (r *BatchRepository) RedistributeBatches(ctx context.Context, tenantID uuid.UUID, productID uuid.UUID, dto *models.RedistributeBatchesDto, createdBy *uuid.UUID) (*models.RedistributeBatchesResponse, error) {
	batchID, err := uuid.Parse(dto.BatchID)
	if err != nil {
		return nil, err
	}

	targets, err := json.Marshal(dto.Targets)
	if err != nil {
		return nil, err
	}

	rows, err := r.dbService.Query(ctx,
		"SELECT id, product_id, sku_id, sku, lot_number, purchase_date, expiry_date, unit_cost, initial_quantity, current_quantity, status, message FROM inventory.sp_redistribute_product_batches($1, $2, $3, $4, $5)",
		tenantID,
		productID,
		batchID,
		targets,
		createdBy,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	response := models.RedistributeBatchesResponse{
		BatchID: batchID,
		Batches: []models.BatchResponse{},
	}

	for rows.Next() {
		var batch models.BatchResponse
		var skuIDValue uuid.UUID
		var skuValue string

		if err := rows.Scan(
			&batch.ID,
			&batch.ProductID,
			&skuIDValue,
			&skuValue,
			&batch.LotNumber,
			&batch.PurchaseDate,
			&batch.ExpiryDate,
			&batch.UnitCost,
			&batch.InitialQuantity,
			&batch.CurrentQuantity,
			&batch.Status,
			&response.Message,
		); err != nil {
			return nil, err
		}

		batch.SkuID = &skuIDValue
		batch.Sku = &skuValue

		response.MovedQuantity += batch.CurrentQuantity
		response.Batches = append(response.Batches, batch)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	response.TargetCount = len(response.Batches)

	return &response, nil
}

// UpdateBatchQuantity updates the quantity in a batch
func (r *BatchRepository) UpdateBatchQuantity(ctx context.Context, tenantID uuid.UUID, batchID uuid.UUID, quantityUsed float64) error {
	// This will be called after a sale to reduce batch quantity
	// Implementation will depend on final SP design
	return nil
}
