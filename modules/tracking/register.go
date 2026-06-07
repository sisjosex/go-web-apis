package tracking

import "josex/web/modules/core/registry"

func init() {
	registry.Register(registry.ModuleDescriptor{
		Code:        "tracking",
		Name:        "Fleet Tracking",
		Description: "Real-time GPS tracking, route management and driver assignments.",
		Icon:        "map-pin",
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
	})
}
