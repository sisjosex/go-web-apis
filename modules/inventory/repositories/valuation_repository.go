package repositories

import (
	"context"

	coreServices "josex/web/modules/core/services"
	"josex/web/modules/inventory/models"

	"github.com/google/uuid"
)

// ValuationRepository calls the valuation SP.
type ValuationRepository struct {
	dbService coreServices.DatabaseService
}

// NewValuationRepository builds the repository.
func NewValuationRepository(dbService coreServices.DatabaseService) *ValuationRepository {
	return &ValuationRepository{dbService: dbService}
}

// Valuation answers a page of products at cost with the totals of all of them.
func (r *ValuationRepository) Valuation(ctx context.Context, tenantID uuid.UUID, query models.ValuationQuery) (*models.ValuationResponse, error) {
	rows, err := r.dbService.Query(ctx,
		`SELECT CAST(product_id AS VARCHAR), sku, name, quantity, unit_cost, value, total_value, uncosted_count, total_count
		 FROM inventory.sp_inventory_valuation($1, $2::UUID, $3, $4)`,
		tenantID, query.CategoryID, query.Page, query.PageSize)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := &models.ValuationResponse{Items: []models.ValuationItem{}, Page: query.Page, PageSize: query.PageSize}
	for rows.Next() {
		var id, sku, name *string
		var quantity *float64
		var item models.ValuationItem
		if err := rows.Scan(&id, &sku, &name, &quantity, &item.UnitCost, &item.Value, &out.TotalValue, &out.UncostedCount, &out.TotalCount); err != nil {
			return nil, err
		}
		if id == nil {
			continue
		}
		item.ProductID, item.SKU, item.Name, item.Quantity = *id, *sku, *name, *quantity
		out.Items = append(out.Items, item)
	}
	return out, rows.Err()
}
