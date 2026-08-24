package repositories

import (
	"context"
	"encoding/json"
	"errors"
	"log"

	"josex/web/modules/core/services"
	"josex/web/modules/inventory/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

type SkuRepository struct {
	dbService services.DatabaseService
	logger    *log.Logger
}

func NewSkuRepository(dbService services.DatabaseService, logger *log.Logger) *SkuRepository {
	return &SkuRepository{
		dbService: dbService,
		logger:    logger,
	}
}

func (r *SkuRepository) ListProductSkus(
	ctx context.Context,
	tenantID uuid.UUID,
	productID string,
	limit, offset int,
) ([]models.ProductSku, int64, error) {
	rows, err := r.dbService.Query(
		ctx,
		`SELECT sku_id, sku, combination_key, is_default, status, sellable,
		        price_modifier, options, current_quantity, reserved_quantity,
		        available_quantity, reorder_level, stock_status, total_count
		 FROM inventory.sp_list_product_skus($1, $2, $3, $4)`,
		tenantID, productID, limit, offset,
	)
	if err != nil {
		if r.logger != nil {
			r.logger.Printf("❌ Error listing product SKUs: %v", err)
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return nil, 0, pgErr
		}
		return nil, 0, err
	}
	defer rows.Close()

	skus := []models.ProductSku{}
	var totalCount int64

	for rows.Next() {
		var s models.ProductSku
		var optionsJSON []byte

		// Every column the SP returns, in its declaration order
		if err := rows.Scan(
			&s.SkuID,
			&s.SKU,
			&s.CombinationKey,
			&s.IsDefault,
			&s.Status,
			&s.Sellable,
			&s.PriceModifier,
			&optionsJSON,
			&s.CurrentQuantity,
			&s.ReservedQuantity,
			&s.AvailableQuantity,
			&s.ReorderLevel,
			&s.StockStatus,
			&totalCount,
		); err != nil {
			return nil, 0, err
		}

		s.Options = []models.ProductSkuOption{}
		if len(optionsJSON) > 0 {
			if err := json.Unmarshal(optionsJSON, &s.Options); err != nil {
				return nil, 0, err
			}
		}

		skus = append(skus, s)
	}
	if err = rows.Err(); err != nil {
		return nil, 0, err
	}

	return skus, totalCount, nil
}

func (r *SkuRepository) GenerateSkus(
	ctx context.Context,
	tenantID uuid.UUID,
	productID string,
	maxAxes, maxCombinations int,
) (*models.GenerateSkusResponse, error) {
	var result models.GenerateSkusResponse

	err := r.dbService.QueryRow(
		ctx,
		`SELECT CAST(product_id AS VARCHAR), axis_count, combination_count,
		        created_count, existing_count, stock_by_variant, message
		 FROM inventory.sp_generate_product_skus($1, $2, $3, $4)`,
		tenantID, productID, maxAxes, maxCombinations,
	).Scan(
		&result.ProductID,
		&result.AxisCount,
		&result.CombinationCount,
		&result.CreatedCount,
		&result.ExistingCount,
		&result.StockByVariant,
		&result.Message,
	)

	if err != nil {
		if r.logger != nil {
			r.logger.Printf("❌ Error generating product SKUs: %v", err)
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return nil, pgErr
		}
		return nil, err
	}

	return &result, nil
}

func (r *SkuRepository) RedistributeStock(
	ctx context.Context,
	tenantID uuid.UUID,
	productID string,
	targetsJSON string,
	createdBy *string,
) (*models.RedistributeStockResponse, error) {
	var result models.RedistributeStockResponse

	err := r.dbService.QueryRow(
		ctx,
		`SELECT CAST(product_id AS VARCHAR), CAST(default_sku_id AS VARCHAR),
		        moved_quantity, target_count, message
		 FROM inventory.sp_redistribute_product_stock($1, $2, $3::JSONB, $4::UUID)`,
		tenantID, productID, targetsJSON, createdBy,
	).Scan(
		&result.ProductID,
		&result.DefaultSkuID,
		&result.MovedQuantity,
		&result.TargetCount,
		&result.Message,
	)

	if err != nil {
		if r.logger != nil {
			r.logger.Printf("❌ Error redistributing product stock: %v", err)
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return nil, pgErr
		}
		return nil, err
	}

	return &result, nil
}

// CreateCombination calls the INV-016 SP that creates one named combination.
// Everything it enforces — the product exists, the axis cap, the combination is
// new, the code is free — is in the SP, so nothing here validates anything.
func (r *SkuRepository) CreateCombination(
	ctx context.Context,
	tenantID uuid.UUID,
	productID string,
	axesJSON string,
	sku *string,
	maxAxes int,
) (*models.CreateCombinationResponse, error) {
	var result models.CreateCombinationResponse

	err := r.dbService.QueryRow(
		ctx,
		`SELECT CAST(product_id AS VARCHAR), CAST(sku_id AS VARCHAR), sku,
		        axis_count
		 FROM inventory.sp_create_product_sku_combination($1, $2, $3, $4, $5)`,
		tenantID, productID, axesJSON, sku, maxAxes,
	).Scan(
		&result.ProductID,
		&result.SkuID,
		&result.SKU,
		&result.AxisCount,
	)

	if err != nil {
		if r.logger != nil {
			r.logger.Printf("❌ Error creating product SKU combination: %v", err)
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return nil, pgErr
		}
		return nil, err
	}

	return &result, nil
}
