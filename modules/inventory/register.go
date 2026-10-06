package inventory

import "josex/web/modules/core/registry"

func init() {
	registry.Register(registry.ModuleDescriptor{
		Code:        "inventory",
		Name:        "Inventory",
		Description: "Stock management, warehouses and product catalog.",
		Icon:        "cube",
		Route:       "/inventory",
		Category:    "operations",
		Features: []string{
			"Product catalog",
			"Warehouse management",
			"Stock movements",
			"Low-stock alerts",
		},
		IsDefault: false,
		// TENANCY-003 D1: what it needs, what is worth offering beside it, which businesses it serves.
		Complements: []string{"sales", "purchasing"},
		Verticals:   []string{"retail"},
	})
}
