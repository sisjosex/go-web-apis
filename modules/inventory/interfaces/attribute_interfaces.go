package interfaces

import (
	"context"

	"josex/web/modules/inventory/models"

	"github.com/google/uuid"
)

// AttributeRepository reads and writes attributes, category sets and facets (INV-024).
type AttributeRepository interface {
	ListAttributes(ctx context.Context, tenantID uuid.UUID, attributeID *string) ([]models.Attribute, error)
	UpsertAttribute(ctx context.Context, tenantID uuid.UUID, attributeID *string, dto models.UpsertAttributeDto) (string, error)
	DeleteAttribute(ctx context.Context, tenantID uuid.UUID, attributeID string) error
	GetCategoryAttributes(ctx context.Context, tenantID uuid.UUID, categoryID string) (*models.CategoryAttributesResponse, error)
	SetCategoryAttributes(ctx context.Context, tenantID uuid.UUID, categoryID string, attributeIDs []string) error
	ReorderCategories(ctx context.Context, tenantID uuid.UUID, items []models.CategoryOrderItem) error
	ProductFacets(ctx context.Context, tenantID uuid.UUID, query models.ProductFacetsQuery, attrs *string) ([]models.ProductFacet, error)
}
