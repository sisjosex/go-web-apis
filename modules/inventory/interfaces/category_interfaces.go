package interfaces

import (
	"context"

	"josex/web/modules/inventory/models"

	"github.com/google/uuid"
)

type CategoryRepository interface {
	CreateCategory(ctx context.Context, tenantID uuid.UUID, dto *models.CreateCategoryDto) (*models.Category, error)
	UpdateCategory(ctx context.Context, tenantID uuid.UUID, categoryID string, dto *models.UpdateCategoryDto) (*models.Category, error)
	DeleteCategory(ctx context.Context, tenantID uuid.UUID, categoryID string) (bool, error)
	GetCategory(ctx context.Context, tenantID uuid.UUID, categoryID string) (*models.Category, error)
	ListCategories(ctx context.Context, tenantID uuid.UUID, parentID *string, isActive bool, limit, offset int) ([]models.Category, error)
	SearchCategories(ctx context.Context, tenantID uuid.UUID, searchTerm string, isActive *bool, limit int) ([]models.Category, error)
	AssignProductToCategory(ctx context.Context, tenantID uuid.UUID, productID, categoryID string) error
	RemoveProductFromCategory(ctx context.Context, tenantID uuid.UUID, productID, categoryID string) error
	GetProductsByCategory(ctx context.Context, tenantID uuid.UUID, categoryID string, limit, offset int) ([]models.GetProductsByCategoryResponse, error)
	GetCategoriesByProduct(ctx context.Context, tenantID uuid.UUID, productID string, limit, offset int) ([]models.Category, error)
	GetCategoryProductCount(ctx context.Context, tenantID uuid.UUID, categoryID string) (int64, error)
}

type CategoryService interface {
	CreateCategory(ctx context.Context, tenantID uuid.UUID, dto *models.CreateCategoryDto) (*models.Category, error)
	UpdateCategory(ctx context.Context, tenantID uuid.UUID, categoryID string, dto *models.UpdateCategoryDto) (*models.Category, error)
	DeleteCategory(ctx context.Context, tenantID uuid.UUID, categoryID string) (bool, error)
	GetCategory(ctx context.Context, tenantID uuid.UUID, categoryID string) (*models.Category, error)
	ListCategories(ctx context.Context, tenantID uuid.UUID, parentID *string, isActive bool, limit, offset int) ([]models.Category, error)
	SearchCategories(ctx context.Context, tenantID uuid.UUID, searchTerm string, isActive *bool, limit int) ([]models.Category, error)
	AssignProductToCategory(ctx context.Context, tenantID uuid.UUID, productID, categoryID string) error
	RemoveProductFromCategory(ctx context.Context, tenantID uuid.UUID, productID, categoryID string) error
	GetProductsByCategory(ctx context.Context, tenantID uuid.UUID, categoryID string, limit, offset int) ([]models.GetProductsByCategoryResponse, error)
	GetCategoriesByProduct(ctx context.Context, tenantID uuid.UUID, productID string, limit, offset int) ([]models.Category, error)
	GetCategoryProductCount(ctx context.Context, tenantID uuid.UUID, categoryID string) (int64, error)
}
