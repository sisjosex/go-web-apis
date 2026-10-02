package services

import (
	"context"
	"encoding/json"
	"log"

	coreServices "josex/web/modules/core/services"
	inventoryConfig "josex/web/modules/inventory/config"
	"josex/web/modules/inventory/interfaces"
	"josex/web/modules/inventory/models"

	"github.com/google/uuid"
)

type ProductService struct {
	repository interfaces.ProductRepository
	config     *inventoryConfig.InventoryConfig
	logger     *log.Logger
	// media deletes the stored image a removed row pointed at (INFRA-007); nil deletes nothing.
	media coreServices.MediaService
}

func NewProductService(
	repository interfaces.ProductRepository,
	config *inventoryConfig.InventoryConfig,
	logger *log.Logger,
	media coreServices.MediaService,
) *ProductService {
	return &ProductService{
		repository: repository,
		config:     config,
		logger:     logger,
		media:      media,
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

	// The create SP generates the combinations of a payload that declares an
	// axis (INV-012 D2), so the caps have to travel with it — the same two
	// InventoryConfig values SkuService hands the generator directly.
	return s.repository.CreateProductWithVariants(
		ctx,
		tenantID,
		dto.SKU,
		dto.Name,
		dto.Description,
		dto.BasePrice,
		variantsJSON,
		s.config.MaxAxes,
		s.config.MaxCombinations,
	)
}

func (s *ProductService) UpdateProductWithVariants(
	ctx context.Context,
	tenantID uuid.UUID,
	productID string,
	dto models.UpdateProductDto,
) (*models.UpdateProductResponse, error) {
	var variantsJSON *string

	// A nil Variants means "leave the tree alone" — the SP reads NULL as that.
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

	return s.repository.UpdateProductWithVariants(
		ctx,
		tenantID,
		productID,
		dto.Name,
		dto.Description,
		dto.BasePrice,
		variantsJSON,
	)
}

func (s *ProductService) GetProduct(ctx context.Context, tenantID uuid.UUID, productID string) (*models.ProductDetail, error) {
	return s.repository.GetProductWithVariants(ctx, tenantID, productID)
}

func (s *ProductService) GetProductBySkU(ctx context.Context, tenantID uuid.UUID, sku string) (*models.ProductDetail, error) {
	return s.repository.GetProductBySkU(ctx, tenantID, sku)
}

func (s *ProductService) ListProducts(
	ctx context.Context,
	tenantID uuid.UUID,
	query models.ListProductsQuery,
) (*models.ListProductsResponse, error) {
	products, totalCount, err := s.repository.ListProducts(ctx, tenantID, query)
	if err != nil {
		return nil, err
	}

	return &models.ListProductsResponse{
		Products:   products,
		TotalCount: totalCount,
		Page:       query.Page,
		PageSize:   query.PageSize,
	}, nil
}

func (s *ProductService) AddProductMedia(ctx context.Context, tenantID uuid.UUID, productID string, dto models.AddProductMediaDto) (*models.AddProductMediaResponse, error) {
	return s.repository.AddProductMedia(ctx, tenantID, productID, dto)
}

// RemoveProductMedia deletes the row, then the stored image behind it. The row is what the caller
// asked about, so an image that cannot be deleted is logged, never a failed removal.
func (s *ProductService) RemoveProductMedia(ctx context.Context, tenantID uuid.UUID, mediaID string) error {
	url, err := s.repository.RemoveProductMedia(ctx, tenantID, mediaID)
	if err != nil {
		return err
	}
	if s.media != nil {
		if err := s.media.Delete(ctx, url); err != nil {
			log.Printf("⚠️  inventory: delete image of removed media %s: %v", mediaID, err)
		}
	}
	return nil
}
