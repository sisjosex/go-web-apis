package services

import (
	"context"
	"encoding/json"
	"log"

	"josex/web/modules/inventory/interfaces"
	"josex/web/modules/inventory/models"

	"github.com/google/uuid"
)

type ProductService struct {
	repository interfaces.ProductRepository
	logger     *log.Logger
}

func NewProductService(repository interfaces.ProductRepository, logger *log.Logger) *ProductService {
	return &ProductService{
		repository: repository,
		logger:     logger,
	}
}

func (s *ProductService) CreateProductWithVariants(
	ctx context.Context,
	tenantID uuid.UUID,
	dto models.CreateProductDto,
) (*models.CreateProductResponse, error) {
	var variantsJSON *string

	if dto.Variants != nil {
		jsonBytes, err := json.Marshal(dto.Variants)
		if err != nil {
			if s.logger != nil {
				s.logger.Printf("❌ Error marshaling variants: %v", err)
			}
			return nil, err
		}
		jsonStr := string(jsonBytes)
		variantsJSON = &jsonStr
	}

	return s.repository.CreateProductWithVariants(
		ctx,
		tenantID,
		dto.SKU,
		dto.Name,
		dto.Description,
		dto.BasePrice,
		variantsJSON,
	)
}

func (s *ProductService) GetProduct(ctx context.Context, tenantID uuid.UUID, productID string) (*models.Product, error) {
	return s.repository.GetProduct(ctx, tenantID, productID)
}

func (s *ProductService) GetProductBySkU(ctx context.Context, tenantID uuid.UUID, sku string) (*models.Product, error) {
	return s.repository.GetProductBySkU(ctx, tenantID, sku)
}

func (s *ProductService) ListProducts(ctx context.Context, tenantID uuid.UUID, limit, offset int) ([]models.Product, error) {
	return s.repository.ListProducts(ctx, tenantID, limit, offset)
}
