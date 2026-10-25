package inventory

import (
	"context"

	"josex/web/modules/core/registry"

	"github.com/google/uuid"
)

// TENANCY-008: whether the tenant has a product and a category yet, in one call.
func init() {
	registry.RegisterSetupChecker("inventory", func(ctx context.Context, q registry.RowQuerier, tenantID uuid.UUID) (map[string]bool, error) {
		var products, categories bool
		if err := q.QueryRow(ctx, `SELECT * FROM inventory.sp_inventory_setup($1)`, tenantID).Scan(&products, &categories); err != nil {
			return nil, err
		}
		return map[string]bool{"products": products, "categories": categories}, nil
	})
}
