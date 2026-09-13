package repositories

import (
	"context"
	"errors"

	coreServices "josex/web/modules/core/services"
	"josex/web/modules/inventory/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ImportLookupRepository calls the read-only SPs of 20260911000001.
type ImportLookupRepository struct {
	dbService coreServices.DatabaseService
}

func NewImportLookupRepository(dbService coreServices.DatabaseService) *ImportLookupRepository {
	return &ImportLookupRepository{dbService: dbService}
}

func (r *ImportLookupRepository) ResolveSkuByAxes(
	ctx context.Context,
	tenantID uuid.UUID,
	sku string,
	axes, options []string,
) (*models.ImportSkuResolution, error) {
	resolution := &models.ImportSkuResolution{}

	err := r.dbService.QueryRow(ctx, `
        SELECT product_id::TEXT, axis_count, matched_sku_id::TEXT, matched_sku,
               default_sku_id::TEXT, default_sku
        FROM inventory.sp_import_resolve_sku_by_axes(
            p_tenant_id := $1,
            p_sku       := $2,
            p_axes      := $3,
            p_options   := $4
        )
    `, tenantID, sku, axes, options).Scan(
		&resolution.ProductID, &resolution.AxisCount, &resolution.MatchedSkuID,
		&resolution.MatchedSku, &resolution.DefaultSkuID, &resolution.DefaultSku,
	)
	if err != nil {
		return nil, err
	}
	return resolution, nil
}

func (r *ImportLookupRepository) FindCategoryID(
	ctx context.Context,
	tenantID uuid.UUID,
	name string,
	parentID *string,
) (*string, error) {
	return r.optionalID(ctx, `
        SELECT category_id::TEXT
        FROM inventory.sp_import_find_category(
            p_tenant_id := $1,
            p_name      := $2,
            p_parent_id := $3::UUID
        )
    `, tenantID, name, parentID)
}

func (r *ImportLookupRepository) FindVariantOptionID(
	ctx context.Context,
	tenantID uuid.UUID,
	productID, axis, option string,
) (*string, error) {
	return r.optionalID(ctx, `
        SELECT option_id::TEXT
        FROM inventory.sp_import_find_variant_option(
            p_tenant_id  := $1,
            p_product_id := $2::UUID,
            p_axis       := $3,
            p_option     := $4
        )
    `, tenantID, productID, axis, option)
}

func (r *ImportLookupRepository) SkuExists(ctx context.Context, tenantID uuid.UUID, sku string) (*models.ImportSkuExistence, error) {
	existence := &models.ImportSkuExistence{}

	err := r.dbService.QueryRow(ctx, `
        SELECT product_sku_taken, variant_sku_taken
        FROM inventory.sp_import_sku_exists(
            p_tenant_id := $1,
            p_sku       := $2
        )
    `, tenantID, sku).Scan(&existence.ProductSkuTaken, &existence.VariantSkuTaken)
	if err != nil {
		return nil, err
	}
	return existence, nil
}

func (r *ImportLookupRepository) FreeCategorySlug(ctx context.Context, tenantID uuid.UUID, base string, limit int) (string, error) {
	slug := ""

	err := r.dbService.QueryRow(ctx, `
        SELECT slug
        FROM inventory.sp_import_free_category_slug(
            p_tenant_id := $1,
            p_base      := $2,
            p_limit     := $3
        )
    `, tenantID, base, limit).Scan(&slug)
	if err != nil {
		return "", err
	}
	return slug, nil
}

// optionalID runs a lookup SP that returns one id or no row at all; no row is a
// miss, reported as nil rather than as an error.
func (r *ImportLookupRepository) optionalID(ctx context.Context, query string, args ...interface{}) (*string, error) {
	id := ""
	if err := r.dbService.QueryRow(ctx, query, args...).Scan(&id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &id, nil
}
