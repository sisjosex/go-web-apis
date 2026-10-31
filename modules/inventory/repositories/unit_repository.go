package repositories

import (
	"context"
	"encoding/json"

	coreServices "josex/web/modules/core/services"
	"josex/web/modules/inventory/models"

	"github.com/google/uuid"
)

// UnitRepository calls the units and codes SPs.
type UnitRepository struct {
	dbService coreServices.DatabaseService
}

// NewUnitRepository builds the repository.
func NewUnitRepository(dbService coreServices.DatabaseService) *UnitRepository {
	return &UnitRepository{dbService: dbService}
}

// ListUnits answers the standard list and the tenant's own.
func (r *UnitRepository) ListUnits(ctx context.Context, tenantID uuid.UUID) ([]models.Unit, error) {
	rows, err := r.dbService.Query(ctx, `SELECT id, code, name_es, name_en, symbol, siat_code, allow_fraction FROM inventory.sp_list_units($1)`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	units := []models.Unit{}
	for rows.Next() {
		var unit models.Unit
		if err := rows.Scan(&unit.ID, &unit.Code, &unit.NameEs, &unit.NameEn, &unit.Symbol, &unit.SiatCode, &unit.AllowFraction); err != nil {
			return nil, err
		}
		units = append(units, unit)
	}
	return units, rows.Err()
}

// ReplaceProductUnits hands the whole list to the SP, which diffs it by id.
func (r *UnitRepository) ReplaceProductUnits(ctx context.Context, tenantID uuid.UUID, productID string, dto models.ReplaceProductUnitsDto) error {
	body, err := json.Marshal(dto.Units)
	if err != nil {
		return err
	}
	_, err = r.dbService.Execute(ctx, `SELECT inventory.sp_replace_product_units($1, $2::UUID, $3::JSONB)`, tenantID, productID, string(body))
	return err
}

// FindByCode resolves a scanned barcode.
func (r *UnitRepository) FindByCode(ctx context.Context, tenantID uuid.UUID, code string) (*models.CodeLookup, error) {
	var out models.CodeLookup
	err := r.dbService.QueryRow(ctx,
		`SELECT CAST(product_id AS VARCHAR), product_name, CAST(sku_id AS VARCHAR), sku, CAST(product_unit_id AS VARCHAR), unit_name_es, unit_name_en, factor, price
		 FROM inventory.sp_find_by_code($1, $2)`, tenantID, code,
	).Scan(&out.Product.ID, &out.Product.Name, &out.Sku.ID, &out.Sku.SKU, &out.Unit.ID, &out.Unit.NameEs, &out.Unit.NameEn, &out.Factor, &out.Price)
	if err != nil {
		return nil, err
	}
	return &out, nil
}
