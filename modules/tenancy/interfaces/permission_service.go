package interfaces

import (
	"josex/web/modules/tenancy/models"
)

// PermissionService exposes the permission catalog used to build roles.
type PermissionService interface {
	// ListSections returns the roles screen's rows with their codes, optionally filtered by module
	// name. Pass empty string to return all.
	ListSections(module string) []models.PermissionSection
}
