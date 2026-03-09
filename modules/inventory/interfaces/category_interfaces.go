package interfaces

import (
	"context"

	"josex/web/modules/inventory/models"
)

type CategoryRepository interface {
	CreateCategory(ctx context.Context, dto *models.CreateCategoryDto) (*models.Category, error)
	UpdateCategory(ctx context.Context, categoryID string, dto *models.UpdateCategoryDto) (*models.Category, error)
	DeleteCategory(ctx context.Context, categoryID string) (bool, error)
	GetCategory(ctx context.Context, categoryID string) (*models.Category, error)
	ListCategories(ctx context.Context, parentID *string, isActive bool, limit, offset int) ([]models.Category, error)
	SearchCategories(ctx context.Context, searchTerm string, isActive *bool, limit int) ([]models.Category, error)
	AssignProductToCategory(ctx context.Context, productID, categoryID string) error
	RemoveProductFromCategory(ctx context.Context, productID, categoryID string) error
	GetProductsByCategory(ctx context.Context, categoryID string, limit, offset int) ([]models.GetProductsByCategoryResponse, error)
	GetCategoryProductCount(ctx context.Context, categoryID string) (int64, error)
}

type CategoryService interface {
	CreateCategory(ctx context.Context, dto *models.CreateCategoryDto) (*models.Category, error)
	UpdateCategory(ctx context.Context, categoryID string, dto *models.UpdateCategoryDto) (*models.Category, error)
	DeleteCategory(ctx context.Context, categoryID string) (bool, error)
	GetCategory(ctx context.Context, categoryID string) (*models.Category, error)
	ListCategories(ctx context.Context, parentID *string, isActive bool, limit, offset int) ([]models.Category, error)
	SearchCategories(ctx context.Context, searchTerm string, isActive *bool, limit int) ([]models.Category, error)
	AssignProductToCategory(ctx context.Context, productID, categoryID string) error
	RemoveProductFromCategory(ctx context.Context, productID, categoryID string) error
	GetProductsByCategory(ctx context.Context, categoryID string, limit, offset int) ([]models.GetProductsByCategoryResponse, error)
	GetCategoryProductCount(ctx context.Context, categoryID string) (int64, error)
}
