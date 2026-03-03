package interfaces

import (
	"context"

	"josex/web/modules/inventory/models"
)

type ProductRepository interface {
	CreateProductWithVariants(
		ctx context.Context,
		sku, name string,
		description *string,
		basePrice float64,
		variantsJSON *string,
	) (*models.CreateProductResponse, error)

	GetProduct(ctx context.Context, productID string) (*models.Product, error)
	GetProductBySkU(ctx context.Context, sku string) (*models.Product, error)
	ListProducts(ctx context.Context, limit, offset int) ([]models.Product, error)
}
