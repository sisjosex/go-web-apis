package sales

import "josex/web/modules/core/registry"

func init() {
	registry.Register(registry.ModuleDescriptor{
		Code:        "sales",
		Name:        "Sales",
		Description: "Orders, quotes and customer management.",
		Icon:        "data-bar-vertical",
		Route:       "/sales",
		Category:    "commerce",
		Features: []string{
			"Order management",
			"Quote generation",
			"Customer database",
			"Revenue reports",
		},
		IsDefault: false,
		// TENANCY-003 D1: what it needs, what is worth offering beside it, which businesses it serves.
		Requires:    []string{"inventory"},
		Complements: []string{"purchasing"},
		Verticals:   []string{"retail"},
	})
}
