package services

import (
	"context"
	"log"

	"josex/web/modules/inventory/interfaces"
	"josex/web/modules/inventory/models"
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
func (s *CategoryService) CreateCategory(ctx context.Context, dto *models.CreateCategoryDto) (*models.Category, error) {
	return s.repository.CreateCategory(ctx, dto)
}

// UpdateCategory updates an existing category
func (s *CategoryService) UpdateCategory(ctx context.Context, categoryID string, dto *models.UpdateCategoryDto) (*models.Category, error) {
	return s.repository.UpdateCategory(ctx, categoryID, dto)
}

// DeleteCategory soft-deletes a category
func (s *CategoryService) DeleteCategory(ctx context.Context, categoryID string) (bool, error) {
	return s.repository.DeleteCategory(ctx, categoryID)
}

// GetCategory retrieves a single category by ID
func (s *CategoryService) GetCategory(ctx context.Context, categoryID string) (*models.Category, error) {
	return s.repository.GetCategory(ctx, categoryID)
}

// ListCategories lists all categories (or filtered by parent)
func (s *CategoryService) ListCategories(ctx context.Context, parentID *string, isActive bool, limit, offset int) ([]models.Category, error) {
	return s.repository.ListCategories(ctx, parentID, isActive, limit, offset)
}

// SearchCategories searches categories by name, slug, or description
func (s *CategoryService) SearchCategories(ctx context.Context, searchTerm string, isActive *bool, limit int) ([]models.Category, error) {
	return s.repository.SearchCategories(ctx, searchTerm, isActive, limit)
}

// AssignProductToCategory assigns a product to a category
func (s *CategoryService) AssignProductToCategory(ctx context.Context, productID, categoryID string) error {
	return s.repository.AssignProductToCategory(ctx, productID, categoryID)
}

// RemoveProductFromCategory removes a product from a category
func (s *CategoryService) RemoveProductFromCategory(ctx context.Context, productID, categoryID string) error {
	return s.repository.RemoveProductFromCategory(ctx, productID, categoryID)
}

// GetProductsByCategory gets all products in a category
func (s *CategoryService) GetProductsByCategory(ctx context.Context, categoryID string, limit, offset int) ([]models.GetProductsByCategoryResponse, error) {
	return s.repository.GetProductsByCategory(ctx, categoryID, limit, offset)
}

// GetCategoryProductCount gets the count of products in a category
func (s *CategoryService) GetCategoryProductCount(ctx context.Context, categoryID string) (int64, error) {
	return s.repository.GetCategoryProductCount(ctx, categoryID)
}
