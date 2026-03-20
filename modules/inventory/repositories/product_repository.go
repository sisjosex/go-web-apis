package repositories

import (
	"context"
	"log"

	"josex/web/modules/core/services"
	"josex/web/modules/inventory/models"

	"github.com/google/uuid"
)

type ProductRepository struct {
	dbService services.DatabaseService
	logger    *log.Logger
}

func NewProductRepository(dbService services.DatabaseService, logger *log.Logger) *ProductRepository {
	return &ProductRepository{
		dbService: dbService,
		logger:    logger,
	}
}

func (r *ProductRepository) CreateProductWithVariants(
	ctx context.Context,
	tenantID uuid.UUID,
	sku, name string,
	description *string,
	basePrice float64,
	variantsJSON *string,
) (*models.CreateProductResponse, error) {
	var productID, respSku, respName, message string

	err := r.dbService.QueryRow(
		ctx,
		`SELECT CAST(product_id AS VARCHAR), sku, name, message FROM inventory.sp_create_product_with_variants($1, $2, $3, $4, $5, $6::JSONB, NULL::UUID)`,
		tenantID, sku, name, description, basePrice, variantsJSON,
	).Scan(&productID, &respSku, &respName, &message)

	if err != nil {
		if r.logger != nil {
			r.logger.Printf("❌ Error creating product: %v", err)
		}
		return nil, err
	}

	if r.logger != nil {
		r.logger.Printf("✅ Product created: ID=%s, SKU=%s, Name=%s", productID, respSku, respName)
	}
	return &models.CreateProductResponse{
		ProductID: productID,
		SKU:       respSku,
		Name:      respName,
		Message:   message,
	}, nil
}

func (r *ProductRepository) GetProduct(ctx context.Context, tenantID uuid.UUID, productID string) (*models.Product, error) {
	var product models.Product

	err := r.dbService.QueryRow(
		ctx,
		`SELECT id, sku, name, description, base_price, has_variants, status, created_at
		 FROM inventory.sp_get_product($1, $2)`,
		tenantID, productID,
	).Scan(&product.ID, &product.SKU, &product.Name, &product.Description, &product.BasePrice, &product.HasVariants, &product.Status, &product.CreatedAt)

	if err != nil {
		if r.logger != nil {
			r.logger.Printf("❌ Error getting product: %v", err)
		}
		return nil, err
	}

	return &product, nil
}

func (r *ProductRepository) GetProductBySkU(ctx context.Context, tenantID uuid.UUID, sku string) (*models.Product, error) {
	var product models.Product

	err := r.dbService.QueryRow(
		ctx,
		`SELECT id, sku, name, description, base_price, has_variants, status, created_at
		 FROM inventory.sp_get_product_by_sku($1, $2)`,
		tenantID, sku,
	).Scan(&product.ID, &product.SKU, &product.Name, &product.Description, &product.BasePrice, &product.HasVariants, &product.Status, &product.CreatedAt)

	if err != nil {
		if r.logger != nil {
			r.logger.Printf("❌ Error getting product by SKU: %v", err)
		}
		return nil, err
	}

	return &product, nil
}

func (r *ProductRepository) ListProducts(ctx context.Context, tenantID uuid.UUID, limit, offset int) ([]models.Product, error) {
	rows, err := r.dbService.Query(
		ctx,
		`SELECT id, sku, name, description, base_price, has_variants, status, created_at
		 FROM inventory.sp_list_products($1, $2, $3)`,
		tenantID, limit, offset,
	)
	if err != nil {
		if r.logger != nil {
			r.logger.Printf("❌ Error listing products: %v", err)
		}
		return nil, err
	}
	defer rows.Close()

	var products []models.Product
	for rows.Next() {
		var p models.Product
		if err := rows.Scan(&p.ID, &p.SKU, &p.Name, &p.Description, &p.BasePrice, &p.HasVariants, &p.Status, &p.CreatedAt); err != nil {
			if r.logger != nil {
				r.logger.Printf("❌ Error scanning product: %v", err)
			}
			continue
		}
		products = append(products, p)
	}

	return products, nil
}
