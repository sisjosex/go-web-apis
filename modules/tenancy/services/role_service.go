package services

import (
	"context"

	"josex/web/modules/tenancy/interfaces"
	"josex/web/modules/tenancy/models"

	"github.com/google/uuid"
)

type roleService struct {
	repo interfaces.RoleRepository
}

// NewRoleService creates a new RoleService.
func NewRoleService(repo interfaces.RoleRepository) interfaces.RoleService {
	return &roleService{repo: repo}
}

func (s *roleService) CreateRole(ctx context.Context, tenantID, requesterID uuid.UUID, dto *models.CreateRoleDto) (*models.Role, error) {
	return s.repo.CreateRole(ctx, tenantID, requesterID, dto)
}

func (s *roleService) UpdateRole(ctx context.Context, tenantID, requesterID, roleID uuid.UUID, dto *models.UpdateRoleDto) (*models.Role, error) {
	return s.repo.UpdateRole(ctx, tenantID, requesterID, roleID, dto)
}

func (s *roleService) DeleteRole(ctx context.Context, tenantID, requesterID, roleID uuid.UUID) error {
	return s.repo.DeleteRole(ctx, tenantID, requesterID, roleID)
}

func (s *roleService) ListRoles(ctx context.Context, tenantID uuid.UUID) ([]*models.RoleListItem, error) {
	return s.repo.ListRoles(ctx, tenantID)
}

func (s *roleService) GetRole(ctx context.Context, tenantID, roleID uuid.UUID) (*models.RoleDetail, error) {
	return s.repo.GetRole(ctx, tenantID, roleID)
}

func (s *roleService) SetRolePermissions(ctx context.Context, tenantID, requesterID, roleID uuid.UUID, permCodes []string) error {
	return s.repo.SetRolePermissions(ctx, tenantID, requesterID, roleID, permCodes)
}

func (s *roleService) AssignUserRole(ctx context.Context, tenantID, requesterID, userID, roleID uuid.UUID) error {
	return s.repo.AssignUserRole(ctx, tenantID, requesterID, userID, roleID)
}

func (s *roleService) RevokeUserRole(ctx context.Context, tenantID, requesterID, userID, roleID uuid.UUID) error {
	return s.repo.RevokeUserRole(ctx, tenantID, requesterID, userID, roleID)
}

func (s *roleService) ListUserRoles(ctx context.Context, userID, tenantID uuid.UUID) ([]*models.UserRole, error) {
	return s.repo.ListUserRoles(ctx, userID, tenantID)
}

func (s *roleService) ListRoleUsers(ctx context.Context, tenantID, roleID uuid.UUID) ([]*models.RoleUser, error) {
	return s.repo.ListRoleUsers(ctx, tenantID, roleID)
}
