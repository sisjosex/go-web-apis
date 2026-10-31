package repositories

import (
	"context"
	"encoding/json"

	coreServices "josex/web/modules/core/services"
	"josex/web/modules/inventory/models"

	"github.com/google/uuid"
)

// AttributeRepository calls the attribute, category-set and facet SPs.
type AttributeRepository struct {
	dbService coreServices.DatabaseService
}

// NewAttributeRepository builds the repository.
func NewAttributeRepository(dbService coreServices.DatabaseService) *AttributeRepository {
	return &AttributeRepository{dbService: dbService}
}

// ListAttributes answers all of them, or the one named.
func (r *AttributeRepository) ListAttributes(ctx context.Context, tenantID uuid.UUID, attributeID *string) ([]models.Attribute, error) {
	rows, err := r.dbService.Query(ctx,
		`SELECT CAST(id AS VARCHAR), code, name, type, unit, filterable, axis, options, product_count, category_names
		 FROM inventory.sp_list_attributes($1, $2::UUID)`, tenantID, attributeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.Attribute{}
	for rows.Next() {
		var row models.Attribute
		var options, categories []byte
		if err := rows.Scan(&row.ID, &row.Code, &row.Name, &row.Type, &row.Unit, &row.Filterable, &row.Axis, &options, &row.ProductCount, &categories); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(options, &row.Options); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(categories, &row.CategoryNames); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// UpsertAttribute writes the whole definition and answers its id.
func (r *AttributeRepository) UpsertAttribute(ctx context.Context, tenantID uuid.UUID, attributeID *string, dto models.UpsertAttributeDto) (string, error) {
	options, err := json.Marshal(dto.Options)
	if err != nil {
		return "", err
	}
	var id string
	err = r.dbService.QueryRow(ctx,
		`SELECT CAST(inventory.sp_upsert_attribute($1, $2::UUID, $3, $4, $5, $6, $7::JSONB, $8) AS VARCHAR)`,
		tenantID, attributeID, dto.Name, dto.Type, dto.Unit, dto.Filterable, string(options), dto.Axis,
	).Scan(&id)
	return id, err
}

// DeleteAttribute removes one no product uses.
func (r *AttributeRepository) DeleteAttribute(ctx context.Context, tenantID uuid.UUID, attributeID string) error {
	_, err := r.dbService.Execute(ctx, `SELECT inventory.sp_delete_attribute($1, $2::UUID)`, tenantID, attributeID)
	return err
}

// GetCategoryAttributes splits a category's attributes into its own and those inherited.
func (r *AttributeRepository) GetCategoryAttributes(ctx context.Context, tenantID uuid.UUID, categoryID string) (*models.CategoryAttributesResponse, error) {
	rows, err := r.dbService.Query(ctx,
		`SELECT CAST(id AS VARCHAR), code, name, type, unit, axis, options, inherited FROM inventory.sp_get_category_attributes($1, $2::UUID)`,
		tenantID, categoryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := &models.CategoryAttributesResponse{Own: []models.CategoryAttribute{}, Inherited: []models.CategoryAttribute{}}
	for rows.Next() {
		var row models.CategoryAttribute
		var options []byte
		var inherited bool
		if err := rows.Scan(&row.ID, &row.Code, &row.Name, &row.Type, &row.Unit, &row.Axis, &options, &inherited); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(options, &row.Options); err != nil {
			return nil, err
		}
		if inherited {
			out.Inherited = append(out.Inherited, row)
		} else {
			out.Own = append(out.Own, row)
		}
	}
	return out, rows.Err()
}

// SetCategoryAttributes replaces the category's own attributes.
func (r *AttributeRepository) SetCategoryAttributes(ctx context.Context, tenantID uuid.UUID, categoryID string, attributeIDs []string) error {
	_, err := r.dbService.Execute(ctx, `SELECT inventory.sp_set_category_attributes($1, $2::UUID, $3::UUID[])`, tenantID, categoryID, attributeIDs)
	return err
}

// ReorderCategories moves them in one statement.
func (r *AttributeRepository) ReorderCategories(ctx context.Context, tenantID uuid.UUID, items []models.CategoryOrderItem) error {
	body, err := json.Marshal(items)
	if err != nil {
		return err
	}
	_, err = r.dbService.Execute(ctx, `SELECT inventory.sp_reorder_categories($1, $2::JSONB)`, tenantID, string(body))
	return err
}

// ProductFacets answers what the list's filters offer.
func (r *AttributeRepository) ProductFacets(ctx context.Context, tenantID uuid.UUID, query models.ProductFacetsQuery, attrs *string) ([]models.ProductFacet, error) {
	rows, err := r.dbService.Query(ctx,
		`SELECT code, name, options FROM inventory.sp_product_facets($1, $2::UUID, $3, $4::JSONB)`,
		tenantID, query.CategoryID, query.Search, attrs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.ProductFacet{}
	for rows.Next() {
		var row models.ProductFacet
		var options []byte
		if err := rows.Scan(&row.Code, &row.Name, &options); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(options, &row.Options); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}
