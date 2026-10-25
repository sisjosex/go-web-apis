package registry

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
)

func offered(installed ...string) map[string]Vertical {
	out := map[string]Vertical{}
	for _, v := range OfferedVerticals(func(code string) bool { return slices.Contains(installed, code) }) {
		out[v.Code] = v
	}
	return out
}

// TENANCY-006 D1: a type is offered when one of its business modules is installed and its Defining one is;
// it keeps only the installed modules.
func TestOfferedVerticals_HidesTypesWithoutTheirModules(t *testing.T) {
	RegisterCapability(Capability{Code: "variants", Module: "inventory"})

	got := offered("inventory", "sales", "users", "tracking")

	assert.NotContains(t, got, "barber", "booking is not installed")
	assert.NotContains(t, got, "intercity_bus", "ticketing is not installed")
	assert.NotContains(t, got, "radio_taxi", "dispatch is not installed")
	assert.NotContains(t, got, "delivery", "shop is not installed")
	assert.Equal(t, []string{"inventory", "sales", "users"}, got["hardware"].Modules)
	assert.Equal(t, []string{"variants"}, got["clothing"].Capabilities, "layaway has no module yet")
	assert.Contains(t, got, "staff_transport")
	assert.Len(t, verticals, 33)

	assert.Contains(t, offered("inventory", "sales", "users", "booking"), "barber")
}

func TestVertical_PrimaryIsDefiningElseFirstBusinessModule(t *testing.T) {
	got := offered("inventory", "sales", "users", "tracking")

	assert.Equal(t, "inventory", got["bakery"].Primary(), "pos is not installed")
	assert.Equal(t, "tracking", got["school_transport"].Primary())
}

// D3: a tenant that never stored the list has every capability on; a stored list restricts.
func TestCapabilityOn_AllUntilAListIsStored(t *testing.T) {
	RegisterCapability(Capability{Code: "variants", Module: "inventory"})
	RegisterCapability(Capability{Code: "lots", Module: "inventory"})

	assert.True(t, CapabilityOn(map[string]any{}, "lots"))
	assert.True(t, CapabilityOn(map[string]any{CapabilitiesSetting: []any{"variants"}}, "variants"))
	assert.False(t, CapabilityOn(map[string]any{CapabilitiesSetting: []any{"variants"}}, "lots"))

	spec, _ := SettingFor(CapabilitiesSetting)
	assert.False(t, spec.Accepts([]any{"variants", "variants"}), "duplicates")
	assert.False(t, spec.Accepts([]any{"tables"}), "not registered")
	assert.True(t, spec.Accepts([]any{}))
}

func TestVerticals_EveryStarterHasAName(t *testing.T) {
	for _, v := range verticals {
		for _, code := range v.Starter {
			assert.NotEmpty(t, StarterNames[code], "%s starter %s", v.Code, code)
		}
	}
}
