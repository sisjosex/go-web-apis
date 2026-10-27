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
		// TENANCY-003 D1: what it needs and what is worth offering beside it.
		// TENANCY-008: each piece needs the one before it (TRACK-050 D1); setup.go says which are done.
		Setup: []registry.SetupStep{
			{Code: "companies", Minutes: 2, Route: "/tracking/companies?new=1"},
			// TRACK-073: the client a route carries riders to, before its drivers and routes.
			{Code: "organizations", Minutes: 2, Route: "/tracking/organizations?new=1"},
			{Code: "drivers", Minutes: 3, Route: "/tracking/drivers?new=1"},
			{Code: "vehicles", Minutes: 2, Route: "/tracking/vehicles?new=1"},
			{Code: "routes", Minutes: 5, Route: "/tracking/routes?new=1"},
			{Code: "riders", Minutes: 5, Route: "/tracking/riders?new=1"},
		},
	})
}
