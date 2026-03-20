package services

import (
	"context"

	"josex/web/modules/inventory/interfaces"
	"josex/web/modules/inventory/models"

	"github.com/google/uuid"
)

type BatchService struct {
	repository interfaces.BatchRepository
}

func NewBatchService(repository interfaces.BatchRepository) interfaces.BatchService {
	return &BatchService{
		repository: repository,
	}
}

// CreateBatch creates a new batch
func (s *BatchService) CreateBatch(ctx context.Context, tenantID uuid.UUID, batch *models.CreateBatchDto) (*models.BatchResponse, error) {
	return s.repository.CreateBatch(ctx, tenantID, batch)
}

// GetBatch retrieves a batch by ID
func (s *BatchService) GetBatch(ctx context.Context, tenantID uuid.UUID, batchID uuid.UUID) (*models.BatchResponse, error) {
	return s.repository.GetBatch(ctx, tenantID, batchID)
}

// ListBatchesByProduct lists all batches for a product
func (s *BatchService) ListBatchesByProduct(ctx context.Context, tenantID uuid.UUID, productID uuid.UUID, onlyActive bool) (*models.ListBatchesResponse, error) {
	return s.repository.ListBatchesByProduct(ctx, tenantID, productID, onlyActive)
}

// GetOldestBatchForSale gets the oldest batch for FIFO sales
func (s *BatchService) GetOldestBatchForSale(ctx context.Context, tenantID uuid.UUID, productID uuid.UUID) (*models.BatchResponse, error) {
	return s.repository.GetOldestBatchForSale(ctx, tenantID, productID)
}

// GetExpiringBatches gets all batches expiring soon
func (s *BatchService) GetExpiringBatches(ctx context.Context, tenantID uuid.UUID, warningDays int) ([]*models.BatchResponse, error) {
	return s.repository.GetExpiringBatches(ctx, tenantID, warningDays)
}

// ConsumeBatchStock reduces batch quantity after a sale
func (s *BatchService) ConsumeBatchStock(ctx context.Context, tenantID uuid.UUID, batchID uuid.UUID, quantity float64) error {
	return s.repository.UpdateBatchQuantity(ctx, tenantID, batchID, quantity)
}
