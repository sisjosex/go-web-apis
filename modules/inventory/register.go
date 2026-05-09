package inventory

import "josex/web/modules/core/registry"

func init() {
	registry.Register(registry.ModuleDescriptor{
		Code:        "inventory",
		Name:        "Inventory",
		Description: "Stock management, warehouses and product catalog.",
		Icon:        "archive",
		Category:    "operations",
		Features: []string{
			"Product catalog",
			"Warehouse management",
			"Stock movements",
			"Low-stock alerts",
		},
		IsDefault: false,
	})
}
