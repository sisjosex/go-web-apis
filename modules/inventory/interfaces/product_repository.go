package interfaces

import (
	"context"

	"josex/web/modules/inventory/models"

	"github.com/google/uuid"
)

type ProductRepository interface {
	// maxAxes and maxCombinations are the InventoryConfig caps (INV-008 D5). The
	// create SP generates the combinations of a payload that declares an axis
	// (INV-012 D2), so it is the create path's turn to enforce them.
	CreateProductWithVariants(
		ctx context.Context,
		tenantID uuid.UUID,
		sku, name string,
		description *string,
		basePrice float64,
		variantsJSON *string,
		maxAxes, maxCombinations int,
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
	ListProducts(ctx context.Context, tenantID uuid.UUID, query models.ListProductsQuery) ([]models.Product, int64, error)
	AddProductMedia(ctx context.Context, tenantID uuid.UUID, productID string, dto models.AddProductMediaDto) (*models.AddProductMediaResponse, error)
	// RemoveProductMedia deletes the row and answers its URL, so the stored object can follow it.
	RemoveProductMedia(ctx context.Context, tenantID uuid.UUID, mediaID string) (string, error)
}
