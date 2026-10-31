package repositories

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"time"

	"josex/web/modules/core/limits"
	"josex/web/modules/core/services"
	"josex/web/modules/inventory/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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
	maxAxes, maxCombinations int,
	actedBy *string,
	posVisible, ordersVisible *bool,
) (*models.CreateProductResponse, error) {
	var productID, respSku, respName, message string

	err := r.dbService.QueryRow(
		ctx,
		// The plan's product limit is checked in the same statement (BILLING-001 D2).
		`SELECT CAST(p.product_id AS VARCHAR), p.sku, p.name, p.message
		 FROM public.fn_within_limit($1, (SELECT COUNT(*) FROM inventory.products x WHERE x.tenant_id = $1), $9::INT, 'inventory_items') lim(tenant_id)
		 CROSS JOIN LATERAL inventory.sp_create_product_with_variants(lim.tenant_id, $2, $3, $4, $5, $6::JSONB, $10::UUID, $7, $8, $11::BOOLEAN, $12::BOOLEAN) p`,
		tenantID, sku, name, description, basePrice, variantsJSON, maxAxes, maxCombinations,
		limits.For(ctx, tenantID, limits.InventoryItems), actedBy, posVisible, ordersVisible,
	).Scan(&productID, &respSku, &respName, &message)

	if err != nil {
		if r.logger != nil {
			r.logger.Printf("❌ Error creating product: %v", err)
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return nil, pgErr
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

func (r *ProductRepository) UpdateProductWithVariants(
	ctx context.Context,
	tenantID uuid.UUID,
	productID string,
	name string,
	description *string,
	basePrice float64,
	variantsJSON *string,
	actedBy *string,
	posVisible, ordersVisible *bool,
) (*models.UpdateProductResponse, error) {
	var respProductID, respSku, respName, message string

	err := r.dbService.QueryRow(
		ctx,
		`SELECT CAST(product_id AS VARCHAR), sku, name, message FROM inventory.sp_update_product_with_variants($1, $2, $3, $4, $5, $6::JSONB, $7::UUID, $8::BOOLEAN, $9::BOOLEAN)`,
		tenantID, productID, name, description, basePrice, variantsJSON, actedBy, posVisible, ordersVisible,
	).Scan(&respProductID, &respSku, &respName, &message)

	if err != nil {
		if r.logger != nil {
			r.logger.Printf("❌ Error updating product: %v", err)
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return nil, pgErr
		}
		return nil, err
	}

	if r.logger != nil {
		r.logger.Printf("✅ Product updated: ID=%s, SKU=%s, Name=%s", respProductID, respSku, respName)
	}
	return &models.UpdateProductResponse{
		ProductID: respProductID,
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
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return nil, pgErr
		}
		return nil, err
	}

	return &product, nil
}

func (r *ProductRepository) GetProductWithVariants(ctx context.Context, tenantID uuid.UUID, productID string) (*models.ProductDetail, error) {
	var p models.ProductDetail
	var mediaJSON, variantsJSON, unitsJSON []byte

	err := r.dbService.QueryRow(
		ctx,
		`SELECT id, sku, name, description, base_price, has_variants, status, created_at, media, variants, updated_at, updated_by_name, pos_visible, orders_visible, units
		 FROM inventory.sp_get_product_with_variants($1, $2)`,
		tenantID, productID,
	).Scan(&p.ID, &p.SKU, &p.Name, &p.Description, &p.BasePrice, &p.HasVariants, &p.Status, &p.CreatedAt, &mediaJSON, &variantsJSON, &p.UpdatedAt, &p.UpdatedByName, &p.PosVisible, &p.OrdersVisible, &unitsJSON)

	if err != nil {
		if r.logger != nil {
			r.logger.Printf("❌ Error getting product with variants: %v", err)
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return nil, pgErr
		}
		return nil, err
	}

	if err := json.Unmarshal(unitsJSON, &p.Units); err != nil {
		return nil, err
	}

	p.Media = []models.ProductMedia{}
	if len(mediaJSON) > 0 {
		if err := json.Unmarshal(mediaJSON, &p.Media); err != nil {
			return nil, err
		}
	}

	p.Variants = []models.VariantGroup{}
	if len(variantsJSON) > 0 {
		if err := json.Unmarshal(variantsJSON, &p.Variants); err != nil {
			return nil, err
		}
	}

	return &p, nil
}

func (r *ProductRepository) GetProductBySkU(ctx context.Context, tenantID uuid.UUID, sku string) (*models.ProductDetail, error) {
	var p models.ProductDetail
	var mediaJSON, variantsJSON []byte

	err := r.dbService.QueryRow(
		ctx,
		`SELECT id, sku, name, description, base_price, has_variants, status, created_at, media, variants
		 FROM inventory.sp_get_product_with_variants(
		     $1,
		     (SELECT id FROM inventory.products WHERE tenant_id = $1 AND sku = $2 AND status = 'active' LIMIT 1)
		 )`,
		tenantID, sku,
	).Scan(&p.ID, &p.SKU, &p.Name, &p.Description, &p.BasePrice, &p.HasVariants, &p.Status, &p.CreatedAt, &mediaJSON, &variantsJSON)

	if err != nil {
		if r.logger != nil {
			r.logger.Printf("❌ Error getting product by SKU: %v", err)
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return nil, pgErr
		}
		return nil, err
	}

	p.Media = []models.ProductMedia{}
	if len(mediaJSON) > 0 {
		if err := json.Unmarshal(mediaJSON, &p.Media); err != nil {
			return nil, err
		}
	}

	p.Variants = []models.VariantGroup{}
	if len(variantsJSON) > 0 {
		if err := json.Unmarshal(variantsJSON, &p.Variants); err != nil {
			return nil, err
		}
	}

	return &p, nil
}

func (r *ProductRepository) AddProductMedia(ctx context.Context, tenantID uuid.UUID, productID string, dto models.AddProductMediaDto) (*models.AddProductMediaResponse, error) {
	var mediaID, message string

	err := r.dbService.QueryRow(
		ctx,
		`SELECT CAST(media_id AS VARCHAR), message
		 FROM inventory.sp_add_product_media($1, $2, $3, $4, $5, $6, $7, $8)`,
		tenantID, productID, dto.VariantOptionID, dto.MediaType, dto.URL, dto.AltText, dto.IsPrimary, dto.SortOrder,
	).Scan(&mediaID, &message)

	if err != nil {
		if r.logger != nil {
			r.logger.Printf("❌ Error adding product media: %v", err)
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return nil, pgErr
		}
		return nil, err
	}

	return &models.AddProductMediaResponse{MediaID: mediaID, Message: message}, nil
}

func (r *ProductRepository) RemoveProductMedia(ctx context.Context, tenantID uuid.UUID, mediaID string) (string, error) {
	var url string

	err := r.dbService.QueryRow(
		ctx,
		`SELECT media_url FROM inventory.sp_remove_product_media($1, $2)`,
		tenantID, mediaID,
	).Scan(&url)

	if err != nil {
		if r.logger != nil {
			r.logger.Printf("❌ Error removing product media: %v", err)
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return "", pgErr
		}
		return "", err
	}

	return url, nil
}

// ListProducts returns one page of active products plus the total the same filters match.
// total_count comes back on every row and stays 0 when the page is empty.
func (r *ProductRepository) ListProducts(
	ctx context.Context,
	tenantID uuid.UUID,
	query models.ListProductsQuery,
) ([]models.Product, models.ProductCounts, int64, error) {
	var counts models.ProductCounts
	rows, err := r.dbService.Query(
		ctx,
		`SELECT id, sku, name, description, base_price, has_variants, status, created_at, categories, total_count,
		        available_quantity, stock_status, variants_count, axes, min_price, max_price, primary_image_url, pos_visible, counts, units
		 FROM inventory.sp_list_products($1, $2, $3, $4::UUID, $5::VARCHAR, $6::VARCHAR, $7::VARCHAR)`,
		tenantID, query.Page, query.PageSize, query.CategoryID, query.Search, query.Status, query.Filter,
	)
	if err != nil {
		if r.logger != nil {
			r.logger.Printf("❌ Error listing products: %v", err)
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return nil, counts, 0, pgErr
		}
		return nil, counts, 0, err
	}
	defer rows.Close()

	products := []models.Product{}
	var totalCount int64
	for rows.Next() {
		p, categoriesJSON, axesJSON, countsJSON, unitsJSON, err := scanProductRow(rows, &totalCount)
		if err != nil {
			return nil, counts, 0, err
		}
		if err := json.Unmarshal(countsJSON, &counts); err != nil {
			return nil, counts, 0, err
		}
		// An empty page is one row with no product: it only carries the counts.
		if p == nil {
			continue
		}
		p.Categories = []models.ProductCategoryRef{}
		if err := json.Unmarshal(categoriesJSON, &p.Categories); err != nil {
			return nil, counts, 0, err
		}
		if err := json.Unmarshal(unitsJSON, &p.Units); err != nil {
			return nil, counts, 0, err
		}
		p.Axes = []models.ProductAxis{}
		if err := json.Unmarshal(axesJSON, &p.Axes); err != nil {
			return nil, counts, 0, err
		}
		products = append(products, *p)
	}
	if err = rows.Err(); err != nil {
		return nil, counts, 0, err
	}

	return products, counts, totalCount, nil
}

// scanProductRow reads one row of sp_list_products; the product is nil on an empty page's only row.
func scanProductRow(rows pgx.Rows, totalCount *int64) (*models.Product, []byte, []byte, []byte, []byte, error) {
	var (
		id, sku, name, status, stockStatus              *string
		basePrice, available, minPrice, maxPrice        *float64
		hasVariants, posVisible                         *bool
		createdAt                                       *time.Time
		variantsCount                                   *int
		categoriesJSON, axesJSON, countsJSON, unitsJSON []byte
		description, image                              *string
	)
	if err := rows.Scan(&id, &sku, &name, &description, &basePrice, &hasVariants, &status, &createdAt, &categoriesJSON, totalCount,
		&available, &stockStatus, &variantsCount, &axesJSON, &minPrice, &maxPrice, &image, &posVisible, &countsJSON, &unitsJSON); err != nil {
		return nil, nil, nil, nil, nil, err
	}
	if id == nil {
		return nil, nil, nil, countsJSON, nil, nil
	}
	return &models.Product{
		ID: *id, SKU: *sku, Name: *name, Description: description, BasePrice: *basePrice, HasVariants: *hasVariants,
		Status: *status, CreatedAt: *createdAt, AvailableQuantity: derefFloat(available), StockStatus: derefString(stockStatus),
		VariantsCount: *variantsCount, MinPrice: *minPrice, MaxPrice: *maxPrice, PrimaryImageURL: image, PosVisible: *posVisible,
	}, categoriesJSON, axesJSON, countsJSON, unitsJSON, nil
}

func derefFloat(v *float64) float64 {
	if v == nil {
		return 0
	}
	return *v
}

func derefString(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

// SetProductStatus discontinues or reactivates a product; the SP refuses another tenant's.
func (r *ProductRepository) SetProductStatus(ctx context.Context, tenantID uuid.UUID, productID, status string, actedBy *string) (*models.SetProductStatusResponse, error) {
	var out models.SetProductStatusResponse
	err := r.dbService.QueryRow(ctx,
		`SELECT CAST(id AS VARCHAR), status FROM inventory.sp_set_product_status($1, $2::UUID, $3, $4::UUID)`,
		tenantID, productID, status, actedBy,
	).Scan(&out.ID, &out.Status)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// BulkUpdateProducts applies one change to the listed products in one call.
func (r *ProductRepository) BulkUpdateProducts(ctx context.Context, tenantID uuid.UUID, dto models.BulkUpdateProductsDto, actedBy *string) (*models.BulkUpdateProductsResponse, error) {
	var out models.BulkUpdateProductsResponse
	err := r.dbService.QueryRow(ctx,
		`SELECT updated FROM inventory.sp_bulk_update_products($1, $2::UUID[], $3::VARCHAR, $4::NUMERIC, $5::NUMERIC, $6::UUID, $7::BOOLEAN, $8::UUID)`,
		tenantID, dto.IDs, dto.Set.Status, dto.Set.BasePrice, dto.Set.PriceChangePct, dto.Set.CategoryID, dto.Set.PosVisible, actedBy,
	).Scan(&out.Updated)
	if err != nil {
		return nil, err
	}
	return &out, nil
}
