package interfaces

import (
	"context"

	"josex/web/modules/inventory/models"

	"github.com/google/uuid"
)

type ProductRepository interface {
	CreateProductWithVariants(
		ctx context.Context,
		tenantID uuid.UUID,
		sku, name string,
		description *string,
		basePrice float64,
		variantsJSON *string,
	) (*models.CreateProductResponse, error)

	UpdateProductWithVariants(
		ctx context.Context,
		tenantID uuid.UUID,
		productID string,
		name string,
		description *string,
		basePrice float64,
		variantsJSON *string,
	) (*models.UpdateProductResponse, error)

	GetProduct(ctx context.Context, tenantID uuid.UUID, productID string) (*models.Product, error)
	GetProductWithVariants(ctx context.Context, tenantID uuid.UUID, productID string) (*models.ProductDetail, error)
	GetProductBySkU(ctx context.Context, tenantID uuid.UUID, sku string) (*models.ProductDetail, error)
	ListProducts(ctx context.Context, tenantID uuid.UUID, limit, offset int, categoryID *uuid.UUID, search *string) ([]models.Product, error)
	AddProductMedia(ctx context.Context, tenantID uuid.UUID, productID string, dto models.AddProductMediaDto) (*models.AddProductMediaResponse, error)
	RemoveProductMedia(ctx context.Context, tenantID uuid.UUID, mediaID string) error
}
