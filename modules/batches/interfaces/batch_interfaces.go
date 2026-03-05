package interfaces

import (
	"context"

	"josex/web/modules/batches/models"

	"github.com/google/uuid"
)

// BatchRepository defines the interface for batch data access
type BatchRepository interface {
	CreateBatch(ctx context.Context, batch *models.CreateBatchDto) (*models.BatchResponse, error)
	GetBatch(ctx context.Context, batchID uuid.UUID) (*models.BatchResponse, error)
	ListBatchesByProduct(ctx context.Context, productID uuid.UUID, onlyActive bool) (*models.ListBatchesResponse, error)
	GetOldestBatchForSale(ctx context.Context, productID uuid.UUID) (*models.BatchResponse, error)
	UpdateBatchQuantity(ctx context.Context, batchID uuid.UUID, quantityUsed float64) error
}

// BatchService defines the business logic interface for batches
type BatchService interface {
	CreateBatch(ctx context.Context, batch *models.CreateBatchDto) (*models.BatchResponse, error)
	GetBatch(ctx context.Context, batchID uuid.UUID) (*models.BatchResponse, error)
	ListBatchesByProduct(ctx context.Context, productID uuid.UUID, onlyActive bool) (*models.ListBatchesResponse, error)
	GetOldestBatchForSale(ctx context.Context, productID uuid.UUID) (*models.BatchResponse, error)
	GetExpiringBatches(ctx context.Context, warningDays int) ([]*models.BatchResponse, error)
	ConsumeBatchStock(ctx context.Context, batchID uuid.UUID, quantity float64) error
}
