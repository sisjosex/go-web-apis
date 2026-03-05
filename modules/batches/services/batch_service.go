package services

import (
	"context"

	"josex/web/modules/batches/interfaces"
	"josex/web/modules/batches/models"

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
func (s *BatchService) CreateBatch(ctx context.Context, batch *models.CreateBatchDto) (*models.BatchResponse, error) {
	return s.repository.CreateBatch(ctx, batch)
}

// GetBatch retrieves a batch by ID
func (s *BatchService) GetBatch(ctx context.Context, batchID uuid.UUID) (*models.BatchResponse, error) {
	return s.repository.GetBatch(ctx, batchID)
}

// ListBatchesByProduct lists all batches for a product
func (s *BatchService) ListBatchesByProduct(ctx context.Context, productID uuid.UUID, onlyActive bool) (*models.ListBatchesResponse, error) {
	return s.repository.ListBatchesByProduct(ctx, productID, onlyActive)
}

// GetOldestBatchForSale gets the oldest batch for FIFO sales
func (s *BatchService) GetOldestBatchForSale(ctx context.Context, productID uuid.UUID) (*models.BatchResponse, error) {
	return s.repository.GetOldestBatchForSale(ctx, productID)
}

// GetExpiringBatches gets all batches expiring soon
func (s *BatchService) GetExpiringBatches(ctx context.Context, warningDays int) ([]*models.BatchResponse, error) {
	// TODO: Implement query for batches expiring within warningDays
	return []*models.BatchResponse{}, nil
}

// ConsumeBatchStock reduces batch quantity after a sale
func (s *BatchService) ConsumeBatchStock(ctx context.Context, batchID uuid.UUID, quantity float64) error {
	return s.repository.UpdateBatchQuantity(ctx, batchID, quantity)
}
