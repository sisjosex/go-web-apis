// Package permissions provides a global registry for all permission definitions
// across modules. Each module registers its permissions via Register() —
// typically called from an init() function in the module's permissions package.
package permissions

import "josex/web/modules/tenancy/models"

var registry []models.AvailablePermission

// Register adds a module's permission definitions to the global registry.
func Register(perms []models.AvailablePermission) {
	registry = append(registry, perms...)
}

// List returns all registered permissions, optionally filtered by module.
// Pass an empty string to return all.
func List(module string) []models.AvailablePermission {
	if module == "" {
		result := make([]models.AvailablePermission, len(registry))
		copy(result, registry)
		return result
	}
	var filtered []models.AvailablePermission
	for _, p := range registry {
		if p.Module == module {
			filtered = append(filtered, p)
		}
	}
	return filtered
}
