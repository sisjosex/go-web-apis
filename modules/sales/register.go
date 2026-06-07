package sales

import "josex/web/modules/core/registry"

func init() {
	registry.Register(registry.ModuleDescriptor{
		Code:        "sales",
		Name:        "Sales",
		Description: "Orders, quotes and customer management.",
		Icon:        "chart-bar",
		Route:       "/sales",
		Category:    "commerce",
		Features: []string{
			"Order management",
			"Quote generation",
			"Customer database",
			"Revenue reports",
		},
		IsDefault: false,
	})
}
