package services

import (
	"context"
	"log"

	"josex/web/modules/inventory/interfaces"
	"josex/web/modules/inventory/models"

	"github.com/google/uuid"
)

type CategoryService struct {
	repository interfaces.CategoryRepository
	logger     *log.Logger
}

func NewCategoryService(repository interfaces.CategoryRepository, logger *log.Logger) *CategoryService {
	return &CategoryService{
		repository: repository,
		logger:     logger,
	}
}

// CreateCategory creates a new product category
func (s *CategoryService) CreateCategory(ctx context.Context, tenantID uuid.UUID, dto *models.CreateCategoryDto) (*models.Category, error) {
	return s.repository.CreateCategory(ctx, tenantID, dto)
}

// UpdateCategory updates an existing category
func (s *CategoryService) UpdateCategory(ctx context.Context, tenantID uuid.UUID, categoryID string, dto *models.UpdateCategoryDto) (*models.Category, error) {
	return s.repository.UpdateCategory(ctx, tenantID, categoryID, dto)
}

// DeleteCategory soft-deletes a category
func (s *CategoryService) DeleteCategory(ctx context.Context, tenantID uuid.UUID, categoryID string) (bool, error) {
	return s.repository.DeleteCategory(ctx, tenantID, categoryID)
}

// GetCategory retrieves a single category by ID
func (s *CategoryService) GetCategory(ctx context.Context, tenantID uuid.UUID, categoryID string) (*models.Category, error) {
	return s.repository.GetCategory(ctx, tenantID, categoryID)
}

// ListCategories lists all categories (or filtered by parent)
func (s *CategoryService) ListCategories(ctx context.Context, tenantID uuid.UUID, parentID *string, isActive bool, limit, offset int) ([]models.Category, error) {
	return s.repository.ListCategories(ctx, tenantID, parentID, isActive, limit, offset)
}

// SearchCategories searches categories by name, slug, or description
func (s *CategoryService) SearchCategories(ctx context.Context, tenantID uuid.UUID, searchTerm string, isActive *bool, limit int) ([]models.Category, error) {
	return s.repository.SearchCategories(ctx, tenantID, searchTerm, isActive, limit)
}

// AssignProductToCategory assigns a product to a category
func (s *CategoryService) AssignProductToCategory(ctx context.Context, tenantID uuid.UUID, productID, categoryID string) error {
	return s.repository.AssignProductToCategory(ctx, tenantID, productID, categoryID)
}

// RemoveProductFromCategory removes a product from a category
func (s *CategoryService) RemoveProductFromCategory(ctx context.Context, tenantID uuid.UUID, productID, categoryID string) error {
	return s.repository.RemoveProductFromCategory(ctx, tenantID, productID, categoryID)
}

// GetProductsByCategory gets all products in a category
func (s *CategoryService) GetProductsByCategory(ctx context.Context, tenantID uuid.UUID, categoryID string, limit, offset int) ([]models.GetProductsByCategoryResponse, error) {
	return s.repository.GetProductsByCategory(ctx, tenantID, categoryID, limit, offset)
}

// GetCategoriesByProduct gets all categories a product is assigned to
func (s *CategoryService) GetCategoriesByProduct(ctx context.Context, tenantID uuid.UUID, productID string, limit, offset int) ([]models.Category, error) {
	return s.repository.GetCategoriesByProduct(ctx, tenantID, productID, limit, offset)
}

// GetCategoryProductCount gets the count of products in a category
func (s *CategoryService) GetCategoryProductCount(ctx context.Context, tenantID uuid.UUID, categoryID string) (int64, error) {
	return s.repository.GetCategoryProductCount(ctx, tenantID, categoryID)
}
