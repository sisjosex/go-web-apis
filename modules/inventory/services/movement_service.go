package services

import (
	"context"
	"log"

	"josex/web/modules/inventory/interfaces"
	"josex/web/modules/inventory/models"

	"github.com/google/uuid"
)

type MovementService struct {
	repository interfaces.MovementRepository
	logger     *log.Logger
}

func NewMovementService(repository interfaces.MovementRepository, logger *log.Logger) *MovementService {
	return &MovementService{
		repository: repository,
		logger:     logger,
	}
}

func (s *MovementService) RecordMovement(
	ctx context.Context,
	tenantID uuid.UUID,
	dto models.RecordMovementDto,
	createdBy *string,
) (*models.RecordMovementResponse, error) {
	return s.repository.RecordMovement(
		ctx,
		tenantID,
		dto.ProductID,
		dto.MovementType,
		dto.Quantity,
		dto.ReferenceType,
		dto.ReferenceID,
		dto.UnitCost,
		dto.Notes,
		createdBy,
	)
}

func (s *MovementService) GetProductStock(ctx context.Context, tenantID uuid.UUID, productID string) (*models.ProductStock, error) {
	return s.repository.GetProductStock(ctx, tenantID, productID)
}

func (s *MovementService) GetMovement(ctx context.Context, tenantID uuid.UUID, movementID string) (*models.InventoryMovement, error) {
	return s.repository.GetMovement(ctx, tenantID, movementID)
}

func (s *MovementService) ListMovements(
	ctx context.Context,
	tenantID uuid.UUID,
	query models.ListMovementsQuery,
) (*models.ListMovementsResponse, error) {
	movements, totalCount, err := s.repository.ListMovements(ctx, tenantID, query)
	if err != nil {
		return nil, err
	}

	return &models.ListMovementsResponse{
		Movements:  movements,
		TotalCount: totalCount,
		Page:       query.Page,
		PageSize:   query.PageSize,
	}, nil
}
