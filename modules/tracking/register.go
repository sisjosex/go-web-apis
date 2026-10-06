package tracking

import "josex/web/modules/core/registry"

func init() {
	registry.Register(registry.ModuleDescriptor{
		Code:        "tracking",
		Name:        "Fleet Tracking",
		Description: "Real-time GPS tracking, route management and driver assignments.",
		Icon:        "location",
		Route:       "/tracking",
		Category:    "operations",
		Features: []string{
			"Real-time GPS location",
			"Route history & playback",
			"Driver & vehicle management",
			"Event alerts & notifications",
			"Client portal access",
		},
		IsDefault: false,
		// TENANCY-003 D1: what it needs, what is worth offering beside it, which businesses it serves.
		Verticals: []string{"transport"},
	})
}
