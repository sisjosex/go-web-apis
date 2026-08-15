package interfaces

import (
	"context"

	"josex/web/modules/inventory/models"

	"github.com/google/uuid"
)

// SkuRepository is the three SP calls INV-008 adds. Everything they enforce —
// the caps, the guards, the exact-sum rule — lives in PostgreSQL, so nothing
// here validates anything.
type SkuRepository interface {
	ListProductSkus(
		ctx context.Context,
		tenantID uuid.UUID,
		productID string,
		limit, offset int,
	) ([]models.ProductSku, int64, error)

	GenerateSkus(
		ctx context.Context,
		tenantID uuid.UUID,
		productID string,
		maxAxes, maxCombinations int,
	) (*models.GenerateSkusResponse, error)

	RedistributeStock(
		ctx context.Context,
		tenantID uuid.UUID,
		productID string,
		targetsJSON string,
		createdBy *string,
	) (*models.RedistributeStockResponse, error)
}

type SkuService interface {
	ListProductSkus(
		ctx context.Context,
		tenantID uuid.UUID,
		productID string,
		query models.ListProductSkusQuery,
	) (*models.ListProductSkusResponse, error)

	GenerateSkus(
		ctx context.Context,
		tenantID uuid.UUID,
		productID string,
	) (*models.GenerateSkusResponse, error)

	RedistributeStock(
		ctx context.Context,
		tenantID uuid.UUID,
		productID string,
		dto models.RedistributeStockDto,
		createdBy *string,
	) (*models.RedistributeStockResponse, error)
}
