package services

import (
	"context"

	"josex/web/modules/tenancy/interfaces"
	"josex/web/modules/tenancy/models"
	permRegistry "josex/web/modules/tenancy/permissions"

	"github.com/google/uuid"
)

type permissionService struct {
	repo interfaces.PermissionRepository
}

// NewPermissionService creates a new PermissionService
func NewPermissionService(repo interfaces.PermissionRepository) interfaces.PermissionService {
	return &permissionService{repo: repo}
}

func (s *permissionService) AssignPermission(ctx context.Context, tenantID, requesterID, targetUserID uuid.UUID, permCode string) error {
	return s.repo.AssignPermission(ctx, tenantID, requesterID, targetUserID, permCode)
}

func (s *permissionService) RevokePermission(ctx context.Context, tenantID, requesterID, targetUserID uuid.UUID, permCode string) error {
	return s.repo.RevokePermission(ctx, tenantID, requesterID, targetUserID, permCode)
}

func (s *permissionService) ListUserPermissions(ctx context.Context, userID, tenantID uuid.UUID) ([]*models.UserPermissionResponse, error) {
	perms, err := s.repo.ListUserPermissions(ctx, userID, tenantID)
	if err != nil {
		return nil, err
	}

	// Enrich with metadata from registry
	for _, p := range perms {
		available := permRegistry.List("")
		for _, a := range available {
			if a.Code == p.Code {
				p.Module = a.Module
				p.Name = a.Name
				break
			}
		}
	}

	return perms, nil
}

func (s *permissionService) ListAvailablePermissions(module string) []models.AvailablePermission {
	return permRegistry.List(module)
}
