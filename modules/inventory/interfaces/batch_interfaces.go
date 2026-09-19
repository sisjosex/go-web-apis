package interfaces

import (
	"context"

	"josex/web/modules/inventory/models"

	"github.com/google/uuid"
)

// BatchRepository defines the interface for batch data access
type BatchRepository interface {
	CreateBatch(ctx context.Context, tenantID uuid.UUID, batch *models.CreateBatchDto) (*models.BatchResponse, error)
	GetBatch(ctx context.Context, tenantID uuid.UUID, batchID uuid.UUID) (*models.BatchResponse, error)
	ListBatchesByProduct(ctx context.Context, tenantID uuid.UUID, productID uuid.UUID, query models.ListBatchesQuery) ([]models.BatchResponse, int64, error)
	GetOldestBatchForSale(ctx context.Context, tenantID uuid.UUID, productID uuid.UUID, skuID *uuid.UUID) (*models.BatchResponse, error)
	GetExpiringBatches(ctx context.Context, tenantID uuid.UUID, query models.ListExpiringBatchesQuery) ([]models.BatchResponse, int64, error)
	UpdateBatch(ctx context.Context, tenantID uuid.UUID, batchID uuid.UUID, dto *models.UpdateBatchDto) (*models.BatchResponse, error)
	VoidBatch(ctx context.Context, tenantID uuid.UUID, batchID uuid.UUID) (*models.BatchResponse, error)
	RedistributeBatches(ctx context.Context, tenantID uuid.UUID, productID uuid.UUID, dto *models.RedistributeBatchesDto, createdBy *uuid.UUID) (*models.RedistributeBatchesResponse, error)
	UpdateBatchQuantity(ctx context.Context, tenantID uuid.UUID, batchID uuid.UUID, quantityUsed float64) error
}

// BatchService defines the business logic interface for batches
type BatchService interface {
	CreateBatch(ctx context.Context, tenantID uuid.UUID, batch *models.CreateBatchDto) (*models.BatchResponse, error)
	GetBatch(ctx context.Context, tenantID uuid.UUID, batchID uuid.UUID) (*models.BatchResponse, error)
	ListBatchesByProduct(ctx context.Context, tenantID uuid.UUID, productID uuid.UUID, query models.ListBatchesQuery) (*models.ListBatchesResponse, error)
	GetOldestBatchForSale(ctx context.Context, tenantID uuid.UUID, productID uuid.UUID, skuID *uuid.UUID) (*models.BatchResponse, error)
	GetExpiringBatches(ctx context.Context, tenantID uuid.UUID, query models.ListExpiringBatchesQuery) (*models.ListBatchesResponse, error)
	UpdateBatch(ctx context.Context, tenantID uuid.UUID, batchID uuid.UUID, dto *models.UpdateBatchDto) (*models.BatchResponse, error)
	VoidBatch(ctx context.Context, tenantID uuid.UUID, batchID uuid.UUID) (*models.BatchResponse, error)
	RedistributeBatches(ctx context.Context, tenantID uuid.UUID, productID uuid.UUID, dto *models.RedistributeBatchesDto, createdBy *uuid.UUID) (*models.RedistributeBatchesResponse, error)
	ConsumeBatchStock(ctx context.Context, tenantID uuid.UUID, batchID uuid.UUID, quantity float64) error
}
