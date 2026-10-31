// Package permissions provides a global registry for all permission definitions
// across modules. Each module registers its permissions via Register() —
// typically called from an init() function in the module's permissions package.
package permissions

import (
	"slices"

	"josex/web/modules/tenancy/models"
)

var (
	registry []models.AvailablePermission
	sections []models.PermissionSection
)

// RegisterSections declares a module's rows of the roles screen, in the order they show.
func RegisterSections(module string, codes ...string) {
	for _, code := range codes {
		sections = append(sections, models.PermissionSection{Code: code, Module: module})
	}
}

// Sections answers the declared sections with their codes, optionally of one module. A section no
// registered code belongs to is left out: there is nothing to grant in it.
func Sections(module string) []models.PermissionSection {
	result := make([]models.PermissionSection, 0, len(sections))
	for _, section := range sections {
		if module != "" && section.Module != module {
			continue
		}
		section.Permissions = sectionPermissions(section.Code)
		if len(section.Permissions) > 0 {
			result = append(result, section)
		}
	}
	return result
}

func sectionPermissions(section string) []models.SectionPermission {
	var out []models.SectionPermission
	for _, p := range registry {
		if p.Section == section || slices.Contains(p.AlsoIn, section) {
			out = append(out, models.SectionPermission{Code: p.Code, Access: p.Access})
		}
	}
	return out
}

// OperatorCodes are the permissions a business's seeded "Operador" role starts with: the day to day of
// taypi go, every section at "Editar" (USERS-015 D1).
func OperatorCodes() []string {
	var codes []string
	for _, p := range registry {
		if p.Module == "tracking" {
			codes = append(codes, p.Code)
		}
	}
	return codes
}

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
