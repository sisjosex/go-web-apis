package users

import "josex/web/modules/core/registry"

func init() {
	registry.Register(registry.ModuleDescriptor{
		Code:        "users",
		Name:        "Users & Access",
		Description: "Manage team members, roles, and access permissions for your workspace.",
		Icon:        "people",
		Category:    "administration",
		Features: []string{
			"User management",
			"Role assignment",
			"Access permissions",
			"Audit log",
		},
		IsDefault: true,
	})
}
