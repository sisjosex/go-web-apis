package interfaces

import (
	"josex/web/modules/tenancy/models"
)

// PermissionService exposes the permission catalog used to build roles.
type PermissionService interface {
	// ListAvailablePermissions returns all registered permissions, optionally
	// filtered by module name. Pass empty string to return all.
	ListAvailablePermissions(module string) []models.AvailablePermission
}
