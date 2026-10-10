package controllers

import (
	"testing"

	"josex/web/modules/core/registry"
	"josex/web/modules/core/utils"

	"github.com/stretchr/testify/assert"
)

// TENANCY-004 D2: the catalog answers in the request's language from the module's lang file, and keeps
// the registry's English where a key is missing.
func TestLocalized_TranslatesWithEnglishFallback(t *testing.T) {
	saved := utils.Translations
	t.Cleanup(func() { utils.Translations = saved })
	utils.Translations = map[string]map[string]string{
		"en": {},
		"es": {
			"module.tracking.description": "Ubicación GPS en tiempo real",
			"module.tracking.feature.0":   "Ubicación en vivo",
		},
	}
	desc := registry.ModuleDescriptor{
		Code:        "tracking",
		Description: "Real-time GPS tracking",
		Features:    []string{"Live location", "Route history"},
	}

	es := localized("es", desc)
	en := localized("en", desc)

	assert.Equal(t, "Ubicación GPS en tiempo real", es.Description)
	assert.Equal(t, []string{"Ubicación en vivo", "Route history"}, es.Features)
	assert.Equal(t, desc.Description, en.Description)
	assert.Equal(t, []string{"Live location", "Route history"}, desc.Features, "the registry's slice is not mutated")
}
