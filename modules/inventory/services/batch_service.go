package services

import (
	"context"
	"time"

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
func (s *BatchService) CreateBatch(ctx context.Context, tenantID uuid.UUID, today time.Time, batch *models.CreateBatchDto) (*models.BatchResponse, error) {
	return s.repository.CreateBatch(ctx, tenantID, today, batch)
}

// GetBatch retrieves a batch by ID
func (s *BatchService) GetBatch(ctx context.Context, tenantID uuid.UUID, today time.Time, batchID uuid.UUID) (*models.BatchResponse, error) {
	return s.repository.GetBatch(ctx, tenantID, today, batchID)
}

// ListBatchesByProduct returns one page of a product's lots in FIFO order
func (s *BatchService) ListBatchesByProduct(ctx context.Context, tenantID uuid.UUID, today time.Time, productID uuid.UUID, query models.ListBatchesQuery) (*models.ListBatchesResponse, error) {
	batches, totalCount, err := s.repository.ListBatchesByProduct(ctx, tenantID, today, productID, query)
	if err != nil {
		return nil, err
	}
	return batchPage(batches, totalCount, query.Page, query.PageSize), nil
}

// GetOldestBatchForSale gets the oldest batch for FIFO sales
func (s *BatchService) GetOldestBatchForSale(ctx context.Context, tenantID uuid.UUID, today time.Time, productID uuid.UUID, skuID *uuid.UUID) (*models.BatchResponse, error) {
	return s.repository.GetOldestBatchForSale(ctx, tenantID, today, productID, skuID)
}

// GetExpiringBatches returns one page of the lots expiring within the window, soonest first
func (s *BatchService) GetExpiringBatches(ctx context.Context, tenantID uuid.UUID, today time.Time, query models.ListExpiringBatchesQuery) (*models.ListBatchesResponse, error) {
	batches, totalCount, err := s.repository.GetExpiringBatches(ctx, tenantID, today, query)
	if err != nil {
		return nil, err
	}
	return batchPage(batches, totalCount, query.Page, query.PageSize), nil
}

// batchPage wraps one page of lots in the list shape; an empty page is [] on the wire, never null.
func batchPage(batches []models.BatchResponse, totalCount int64, page, pageSize int) *models.ListBatchesResponse {
	if batches == nil {
		batches = []models.BatchResponse{}
	}
	return &models.ListBatchesResponse{Batches: batches, TotalCount: totalCount, Page: page, PageSize: pageSize}
}

// UpdateBatch corrects a lot
func (s *BatchService) UpdateBatch(ctx context.Context, tenantID uuid.UUID, today time.Time, batchID uuid.UUID, dto *models.UpdateBatchDto) (*models.BatchResponse, error) {
	return s.repository.UpdateBatch(ctx, tenantID, today, batchID, dto)
}

// VoidBatch writes a lot off
func (s *BatchService) VoidBatch(ctx context.Context, tenantID uuid.UUID, batchID uuid.UUID) (*models.BatchResponse, error) {
	return s.repository.VoidBatch(ctx, tenantID, batchID)
}

// RedistributeBatches splits a stranded lot onto real combinations
func (s *BatchService) RedistributeBatches(ctx context.Context, tenantID uuid.UUID, today time.Time, productID uuid.UUID, dto *models.RedistributeBatchesDto, createdBy *uuid.UUID) (*models.RedistributeBatchesResponse, error) {
	return s.repository.RedistributeBatches(ctx, tenantID, today, productID, dto, createdBy)
}

// ConsumeBatchStock reduces batch quantity after a sale
func (s *BatchService) ConsumeBatchStock(ctx context.Context, tenantID uuid.UUID, batchID uuid.UUID, quantity float64) error {
	return s.repository.UpdateBatchQuantity(ctx, tenantID, batchID, quantity)
}
