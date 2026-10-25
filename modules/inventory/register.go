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
		// TENANCY-003 D1: what it needs and what is worth offering beside it.
		Complements: []string{"sales", "purchasing"},
		// TENANCY-008: the catalogue first; setup.go says which are done.
		Setup: []registry.SetupStep{
			{Code: "products", Minutes: 5, Route: "/inventory/products/new"},
			{Code: "categories", Minutes: 2, Route: "/inventory/categories?new=1"},
		},
	})
	// TENANCY-006 D3: what a business type switches on inside inventory.
	registry.RegisterCapability(registry.Capability{Code: "variants", Module: "inventory", Name: "Sizes, colours and models"})
	registry.RegisterCapability(registry.Capability{Code: "lots", Module: "inventory", Name: "Batches and expiry dates"})
}
