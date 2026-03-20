package repositories

import (
	"context"
	"log"

	"josex/web/modules/core/services"
	"josex/web/modules/inventory/models"

	"github.com/google/uuid"
)

type CategoryRepository struct {
	dbService services.DatabaseService
	logger    *log.Logger
}

func NewCategoryRepository(dbService services.DatabaseService, logger *log.Logger) *CategoryRepository {
	return &CategoryRepository{
		dbService: dbService,
		logger:    logger,
	}
}

// CreateCategory creates a new product category
func (r *CategoryRepository) CreateCategory(ctx context.Context, tenantID uuid.UUID, dto *models.CreateCategoryDto) (*models.Category, error) {
	var category models.Category
	var parentIDVal *uuid.UUID

	if dto.ParentID != nil {
		parentID, err := uuid.Parse(*dto.ParentID)
		if err != nil {
			return nil, err
		}
		parentIDVal = &parentID
	}

	displayOrder := 0
	if dto.DisplayOrder != nil {
		displayOrder = *dto.DisplayOrder
	}

	err := r.dbService.QueryRow(ctx,
		`SELECT * FROM inventory.sp_create_category($1, $2, $3, $4, $5, $6, $7)`,
		tenantID, dto.Name, dto.Slug, parentIDVal, dto.Description, dto.IconURL, displayOrder,
	).Scan(
		&category.ID, &category.ParentID, &category.Name, &category.Slug, &category.Description,
		&category.IconURL, &category.DisplayOrder, &category.IsActive, &category.ProductCount,
		&category.CreatedAt, &category.UpdatedAt,
	)

	if err != nil {
		if r.logger != nil {
			r.logger.Printf("❌ Error creating category: %v", err)
		}
		return nil, err
	}

	if r.logger != nil {
		r.logger.Printf("✅ Category created: ID=%s, Name=%s, Slug=%s", category.ID.String(), category.Name, category.Slug)
	}
	return &category, nil
}

// UpdateCategory updates an existing category
func (r *CategoryRepository) UpdateCategory(ctx context.Context, tenantID uuid.UUID, categoryID string, dto *models.UpdateCategoryDto) (*models.Category, error) {
	var category models.Category
	catID, err := uuid.Parse(categoryID)
	if err != nil {
		return nil, err
	}

	var parentIDVal *uuid.UUID
	if dto.ParentID != nil {
		parentID, err := uuid.Parse(*dto.ParentID)
		if err != nil {
			return nil, err
		}
		parentIDVal = &parentID
	}

	err = r.dbService.QueryRow(ctx,
		`SELECT * FROM inventory.sp_update_category($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		tenantID, catID, dto.Name, dto.Slug, parentIDVal, dto.Description, dto.IconURL, dto.DisplayOrder, dto.IsActive,
	).Scan(
		&category.ID, &category.ParentID, &category.Name, &category.Slug, &category.Description,
		&category.IconURL, &category.DisplayOrder, &category.IsActive, &category.ProductCount,
		&category.CreatedAt, &category.UpdatedAt,
	)

	if err != nil {
		if r.logger != nil {
			r.logger.Printf("❌ Error updating category: %v", err)
		}
		return nil, err
	}

	if r.logger != nil {
		r.logger.Printf("✅ Category updated: ID=%s, Name=%s", category.ID.String(), category.Name)
	}
	return &category, nil
}

// DeleteCategory soft-deletes a category
func (r *CategoryRepository) DeleteCategory(ctx context.Context, tenantID uuid.UUID, categoryID string) (bool, error) {
	catID, err := uuid.Parse(categoryID)
	if err != nil {
		return false, err
	}

	var id uuid.UUID
	var name string
	var deleted bool

	err = r.dbService.QueryRow(ctx,
		`SELECT id, name, deleted FROM inventory.sp_delete_category($1, $2)`,
		tenantID,
		catID,
	).Scan(&id, &name, &deleted)

	if err != nil {
		if r.logger != nil {
			r.logger.Printf("❌ Error deleting category: %v", err)
		}
		return false, err
	}

	if r.logger != nil {
		r.logger.Printf("✅ Category deleted: ID=%s, Name=%s", id.String(), name)
	}
	return deleted, nil
}

// GetCategory retrieves a single category by ID
func (r *CategoryRepository) GetCategory(ctx context.Context, tenantID uuid.UUID, categoryID string) (*models.Category, error) {
	var category models.Category
	catID, err := uuid.Parse(categoryID)
	if err != nil {
		return nil, err
	}

	err = r.dbService.QueryRow(ctx,
		`SELECT id, parent_id, name, slug, description, icon_url, display_order, is_active, product_count, created_at, updated_at
		 FROM inventory.sp_get_category($1, $2)`,
		tenantID,
		catID,
	).Scan(
		&category.ID, &category.ParentID, &category.Name, &category.Slug, &category.Description,
		&category.IconURL, &category.DisplayOrder, &category.IsActive, &category.ProductCount,
		&category.CreatedAt, &category.UpdatedAt,
	)

	if err != nil {
		if r.logger != nil {
			r.logger.Printf("❌ Error getting category: %v", err)
		}
		return nil, err
	}

	return &category, nil
}

// ListCategories lists all categories (or filtered by parent)
func (r *CategoryRepository) ListCategories(ctx context.Context, tenantID uuid.UUID, parentID *string, isActive bool, limit, offset int) ([]models.Category, error) {
	var categories []models.Category
	var parentIDVal *uuid.UUID

	if parentID != nil {
		parsedID, err := uuid.Parse(*parentID)
		if err != nil {
			return categories, err
		}
		parentIDVal = &parsedID
	}

	rows, err := r.dbService.Query(ctx,
		`SELECT id, parent_id, name, slug, description, icon_url, display_order, is_active, product_count, created_at
		 FROM inventory.sp_list_categories($1, $2, $3, 'display_order', $4, $5)`,
		tenantID, parentIDVal, isActive, limit, offset,
	)
	if err != nil {
		if r.logger != nil {
			r.logger.Printf("❌ Error listing categories: %v", err)
		}
		return categories, err
	}
	defer rows.Close()

	for rows.Next() {
		var category models.Category
		err := rows.Scan(
			&category.ID, &category.ParentID, &category.Name, &category.Slug, &category.Description,
			&category.IconURL, &category.DisplayOrder, &category.IsActive, &category.ProductCount, &category.CreatedAt,
		)
		if err != nil {
			continue
		}
		categories = append(categories, category)
	}

	return categories, nil
}

// SearchCategories searches categories by name, slug, or description
func (r *CategoryRepository) SearchCategories(ctx context.Context, tenantID uuid.UUID, searchTerm string, isActive *bool, limit int) ([]models.Category, error) {
	var categories []models.Category

	rows, err := r.dbService.Query(ctx,
		`SELECT id, parent_id, name, slug, description, icon_url, display_order, is_active, product_count
		 FROM inventory.sp_search_categories($1, $2, $3, $4)`,
		tenantID, searchTerm, isActive, limit,
	)
	if err != nil {
		if r.logger != nil {
			r.logger.Printf("❌ Error searching categories: %v", err)
		}
		return categories, err
	}
	defer rows.Close()

	for rows.Next() {
		var category models.Category
		err := rows.Scan(
			&category.ID, &category.ParentID, &category.Name, &category.Slug, &category.Description,
			&category.IconURL, &category.DisplayOrder, &category.IsActive, &category.ProductCount,
		)
		if err != nil {
			continue
		}
		categories = append(categories, category)
	}

	return categories, nil
}

// AssignProductToCategory assigns a product to a category
func (r *CategoryRepository) AssignProductToCategory(ctx context.Context, tenantID uuid.UUID, productID, categoryID string) error {
	prodID, err := uuid.Parse(productID)
	if err != nil {
		return err
	}

	catID, err := uuid.Parse(categoryID)
	if err != nil {
		return err
	}

	var id uuid.UUID
	var prodIDResp uuid.UUID
	var catIDResp uuid.UUID
	var message string

	err = r.dbService.QueryRow(ctx,
		`SELECT id, product_id, category_id, message FROM inventory.sp_assign_product_to_category($1, $2, $3)`,
		tenantID, prodID, catID,
	).Scan(&id, &prodIDResp, &catIDResp, &message)

	if err != nil {
		if r.logger != nil {
			r.logger.Printf("❌ Error assigning product to category: %v", err)
		}
		return err
	}

	if r.logger != nil {
		r.logger.Printf("✅ Product assigned to category: ProductID=%s, CategoryID=%s", prodIDResp.String(), catIDResp.String())
	}
	return nil
}

// RemoveProductFromCategory removes a product from a category
func (r *CategoryRepository) RemoveProductFromCategory(ctx context.Context, tenantID uuid.UUID, productID, categoryID string) error {
	prodID, err := uuid.Parse(productID)
	if err != nil {
		return err
	}

	catID, err := uuid.Parse(categoryID)
	if err != nil {
		return err
	}

	var prodIDResp uuid.UUID
	var catIDResp uuid.UUID
	var removed bool

	err = r.dbService.QueryRow(ctx,
		`SELECT product_id, category_id, removed FROM inventory.sp_remove_product_from_category($1, $2, $3)`,
		tenantID, prodID, catID,
	).Scan(&prodIDResp, &catIDResp, &removed)

	if err != nil {
		if r.logger != nil {
			r.logger.Printf("❌ Error removing product from category: %v", err)
		}
		return err
	}

	if r.logger != nil {
		r.logger.Printf("✅ Product removed from category: ProductID=%s, CategoryID=%s", prodIDResp.String(), catIDResp.String())
	}
	return nil
}

// GetProductsByCategory gets all products in a category
func (r *CategoryRepository) GetProductsByCategory(ctx context.Context, tenantID uuid.UUID, categoryID string, limit, offset int) ([]models.GetProductsByCategoryResponse, error) {
	var products []models.GetProductsByCategoryResponse
	catID, err := uuid.Parse(categoryID)
	if err != nil {
		return products, err
	}

	rows, err := r.dbService.Query(ctx,
		`SELECT id, name, sku, description, base_price, status
		 FROM inventory.sp_get_products_by_category($1, $2, $3, $4)`,
		tenantID, catID, limit, offset,
	)
	if err != nil {
		if r.logger != nil {
			r.logger.Printf("❌ Error getting products by category: %v", err)
		}
		return products, err
	}
	defer rows.Close()

	for rows.Next() {
		var product models.GetProductsByCategoryResponse
		err := rows.Scan(
			&product.ID, &product.Name, &product.SKU, &product.Description, &product.BasePrice, &product.Status,
		)
		if err != nil {
			if r.logger != nil {
				r.logger.Printf("❌ Error scanning product: %v", err)
			}
			continue
		}
		products = append(products, product)
	}

	if r.logger != nil {
		r.logger.Printf("✓ GetProductsByCategory: categoryID=%s, found=%d products", categoryID, len(products))
	}
	return products, nil
}

// GetCategoryProductCount gets the count of products in a category
func (r *CategoryRepository) GetCategoryProductCount(ctx context.Context, tenantID uuid.UUID, categoryID string) (int64, error) {
	catID, err := uuid.Parse(categoryID)
	if err != nil {
		return 0, err
	}

	var catIDResp uuid.UUID
	var productCount int64

	err = r.dbService.QueryRow(ctx,
		`SELECT category_id, product_count FROM inventory.sp_get_category_product_count($1, $2)`,
		tenantID,
		catID,
	).Scan(&catIDResp, &productCount)

	if err != nil {
		if r.logger != nil {
			r.logger.Printf("❌ Error getting category product count: %v", err)
		}
		return 0, err
	}

	return productCount, nil
}
