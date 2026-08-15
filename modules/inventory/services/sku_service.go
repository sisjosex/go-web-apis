package services

import (
	"context"
	"encoding/json"
	"log"

	inventoryConfig "josex/web/modules/inventory/config"
	"josex/web/modules/inventory/interfaces"
	"josex/web/modules/inventory/models"

	"github.com/google/uuid"
)

type SkuService struct {
	repository interfaces.SkuRepository
	config     *inventoryConfig.InventoryConfig
	logger     *log.Logger
}

func NewSkuService(
	repository interfaces.SkuRepository,
	config *inventoryConfig.InventoryConfig,
	logger *log.Logger,
) *SkuService {
	return &SkuService{
		repository: repository,
		config:     config,
		logger:     logger,
	}
}

func (s *SkuService) ListProductSkus(
	ctx context.Context,
	tenantID uuid.UUID,
	productID string,
	query models.ListProductSkusQuery,
) (*models.ListProductSkusResponse, error) {
	offset := (query.Page - 1) * query.PageSize

	skus, totalCount, err := s.repository.ListProductSkus(
		ctx, tenantID, productID, query.PageSize, offset,
	)
	if err != nil {
		return nil, err
	}

	return &models.ListProductSkusResponse{
		Skus:       skus,
		TotalCount: totalCount,
		Page:       query.Page,
		PageSize:   query.PageSize,
	}, nil
}

// GenerateSkus hands the SP the caps from InventoryConfig (D5). The SP enforces
// them — it is the only place that can, since it is the one that counts.
func (s *SkuService) GenerateSkus(
	ctx context.Context,
	tenantID uuid.UUID,
	productID string,
) (*models.GenerateSkusResponse, error) {
	return s.repository.GenerateSkus(
		ctx, tenantID, productID, s.config.MaxAxes, s.config.MaxCombinations,
	)
}

func (s *SkuService) RedistributeStock(
	ctx context.Context,
	tenantID uuid.UUID,
	productID string,
	dto models.RedistributeStockDto,
	createdBy *string,
) (*models.RedistributeStockResponse, error) {
	targetsJSON, err := json.Marshal(dto.Targets)
	if err != nil {
		if s.logger != nil {
			s.logger.Printf("❌ Error marshaling redistribution targets: %v", err)
		}
		return nil, err
	}

	return s.repository.RedistributeStock(
		ctx, tenantID, productID, string(targetsJSON), createdBy,
	)
}
