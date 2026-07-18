package services

import (
	"josex/web/modules/tenancy/interfaces"
	"josex/web/modules/tenancy/models"
	permRegistry "josex/web/modules/tenancy/permissions"
)

type permissionService struct{}

// NewPermissionService creates a new PermissionService
func NewPermissionService() interfaces.PermissionService {
	return &permissionService{}
}

func (s *permissionService) ListAvailablePermissions(module string) []models.AvailablePermission {
	return permRegistry.List(module)
}
