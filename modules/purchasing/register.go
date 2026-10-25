package purchasing

import "josex/web/modules/core/registry"

func init() {
	registry.Register(registry.ModuleDescriptor{
		Code:        "purchasing",
		Name:        "Purchasing",
		Description: "Purchase orders, quotes and customer management.",
		Icon:        "cart",
		Route:       "/purchasing",
		Category:    "operations",
		Features: []string{
			"Purchasing",
			"Quotes",
			"Customers",
		},
		IsDefault: false,
		// TENANCY-003 D1: what it needs and what is worth offering beside it.
		Requires:    []string{"inventory"},
		Complements: []string{"sales"},
	})
}
