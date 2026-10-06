// Package registry holds descriptors for all installable tenant modules.
// Each module calls Register() in its init() so the registry always reflects
// what is actually compiled into the binary.
package registry

import (
	"sort"
	"sync"
)

// ModuleDescriptor contains static metadata about a module shown in the UI.
type ModuleDescriptor struct {
	Code        string   `json:"code"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Icon        string   `json:"icon"`
	Route       string   `json:"route"`    // frontend route path, e.g. "/tracking"
	Category    string   `json:"category"` // "operations", "commerce", "analytics"
	Features    []string `json:"features"`
	IsDefault   bool     `json:"is_default"` // auto-enabled on new tenants
	AdminOnly   bool     `json:"admin_only"` // only visible to admin/owner roles
	// Requires are the modules this one cannot run without; enabling it enables them (TENANCY-003 D1).
	Requires []string `json:"requires"`
	// Complements are the modules worth offering beside it, in settings.
	Complements []string `json:"complements"`
	// Verticals are the business types it belongs to; a module offered to every business has none.
	Verticals []string `json:"verticals"`
}

// Vertical is a business type the onboarding offers (TENANCY-003 D1): picking it enables Modules.
type Vertical struct {
	Code    string   `json:"code"`
	Name    string   `json:"name"`
	Modules []string `json:"modules"`
}

// verticals are declared here, in code: 0 tables, 0 queries. A module joins one by naming it in its
// descriptor's Verticals; the list's Modules are what the onboarding turns on.
var verticals = []Vertical{
	{Code: "transport", Name: "Transporte de personal o escolar", Modules: []string{"tracking", "users"}},
	{Code: "retail", Name: "Tienda o comercio", Modules: []string{"inventory", "sales", "users"}},
}

var (
	mu      sync.RWMutex
	modules = map[string]ModuleDescriptor{}
)

// Register adds or replaces a module descriptor. Called from each module's init().
func Register(m ModuleDescriptor) {
	mu.Lock()
	defer mu.Unlock()
	modules[m.Code] = m
}

// GetAll returns all registered module descriptors.
func GetAll() []ModuleDescriptor {
	mu.RLock()
	defer mu.RUnlock()
	result := make([]ModuleDescriptor, 0, len(modules))
	for _, m := range modules {
		result = append(result, m)
	}
	return result
}

// Verticals returns the business types, each with the modules it enables.
func Verticals() []Vertical {
	return verticals
}

// GetVertical returns one business type by code.
func GetVertical(code string) (Vertical, bool) {
	for _, v := range verticals {
		if v.Code == code {
			return v, true
		}
	}
	return Vertical{}, false
}

// WithRequirements answers code and every module it needs, transitively, requirements first.
func WithRequirements(code string) []string {
	mu.RLock()
	defer mu.RUnlock()
	seen := map[string]bool{}
	var out []string
	var walk func(string)
	walk = func(c string) {
		if seen[c] {
			return
		}
		seen[c] = true
		for _, req := range modules[c].Requires {
			walk(req)
		}
		out = append(out, c)
	}
	walk(code)
	return out
}

// RequiredBy answers, of the enabled codes, those that need code, sorted.
func RequiredBy(code string, enabled map[string]bool) []string {
	mu.RLock()
	defer mu.RUnlock()
	var out []string
	for c := range enabled {
		if c == code || !enabled[c] {
			continue
		}
		for _, req := range modules[c].Requires {
			if req == code {
				out = append(out, c)
				break
			}
		}
	}
	sort.Strings(out)
	return out
}

// Get returns the descriptor for a specific module code.
func Get(code string) (ModuleDescriptor, bool) {
	mu.RLock()
	defer mu.RUnlock()
	m, ok := modules[code]
	return m, ok
}
