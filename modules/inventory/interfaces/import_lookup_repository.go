package interfaces

import (
	"context"

	"josex/web/modules/inventory/models"

	"github.com/google/uuid"
)

// ImportLookupRepository is the read-only SP calls the CSV import descriptors
// make to preview and resolve rows (INV-017 D1). Every one is tenant-scoped in
// the SP; a miss is a nil result, never an error.
type ImportLookupRepository interface {
	ResolveSkuByAxes(
		ctx context.Context,
		tenantID uuid.UUID,
		sku string,
		axes, options []string,
	) (*models.ImportSkuResolution, error)

	FindCategoryID(ctx context.Context, tenantID uuid.UUID, name string, parentID *string) (*string, error)

	FindVariantOptionID(ctx context.Context, tenantID uuid.UUID, productID, axis, option string) (*string, error)

	SkuExists(ctx context.Context, tenantID uuid.UUID, sku string) (*models.ImportSkuExistence, error)

	FreeCategorySlug(ctx context.Context, tenantID uuid.UUID, base string, limit int) (string, error)
}
