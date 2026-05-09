package purchasing

import "josex/web/modules/core/registry"

func init() {
	registry.Register(registry.ModuleDescriptor{
		Code:        "purchasing",
		Name:        "Purchasing",
		Description: "Purchase orders, quotes and customer management.",
		Icon:        "building-shop",
		Category:    "operations",
		Features: []string{
			"Purchasing",
			"Quotes",
			"Customers",
		},
		IsDefault: false,
	})
}
