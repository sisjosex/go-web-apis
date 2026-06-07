// Package registry holds descriptors for all installable tenant modules.
// Each module calls Register() in its init() so the registry always reflects
// what is actually compiled into the binary.
package registry

import "sync"

// ModuleDescriptor contains static metadata about a module shown in the UI.
type ModuleDescriptor struct {
	Code        string   `json:"code"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Icon        string   `json:"icon"`
	Route       string   `json:"route"`      // frontend route path, e.g. "/tracking"
	Category    string   `json:"category"`   // "operations", "commerce", "analytics"
	Features    []string `json:"features"`
	IsDefault   bool     `json:"is_default"` // auto-enabled on new tenants
	AdminOnly   bool     `json:"admin_only"` // only visible to admin/owner roles
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

// Get returns the descriptor for a specific module code.
func Get(code string) (ModuleDescriptor, bool) {
	mu.RLock()
	defer mu.RUnlock()
	m, ok := modules[code]
	return m, ok
}
