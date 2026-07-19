package interfaces

import (
	"context"

	"josex/web/modules/tenancy/models"

	"github.com/google/uuid"
)

// RoleRepository defines data-access methods for tenant roles.
type RoleRepository interface {
	// CreateRole inserts a new role scoped to a tenant.
	CreateRole(ctx context.Context, tenantID, requesterID uuid.UUID, dto *models.CreateRoleDto) (*models.Role, error)

	// UpdateRole updates the name and/or description of an existing role.
	UpdateRole(ctx context.Context, tenantID, requesterID, roleID uuid.UUID, dto *models.UpdateRoleDto) (*models.Role, error)

	// DeleteRole removes a role from a tenant.
	DeleteRole(ctx context.Context, tenantID, requesterID, roleID uuid.UUID) error

	// ListRoles returns all roles for a tenant with their permission count.
	ListRoles(ctx context.Context, tenantID uuid.UUID) ([]*models.RoleListItem, error)

	// GetRole returns a single role with its full permission code array.
	GetRole(ctx context.Context, tenantID, roleID uuid.UUID) (*models.RoleDetail, error)

	// SetRolePermissions replaces all permissions for a role atomically.
	SetRolePermissions(ctx context.Context, tenantID, requesterID, roleID uuid.UUID, permCodes []string) error

	// AssignUserRole assigns a role to a user within a tenant.
	AssignUserRole(ctx context.Context, tenantID, requesterID, userID, roleID uuid.UUID) error

	// RevokeUserRole removes a role from a user within a tenant.
	RevokeUserRole(ctx context.Context, tenantID, requesterID, userID, roleID uuid.UUID) error

	// ListUserRoles returns all roles assigned to a user within a tenant.
	ListUserRoles(ctx context.Context, userID, tenantID uuid.UUID) ([]*models.UserRole, error)

	// ListRoleUsers returns the active tenant users assigned to a role.
	ListRoleUsers(ctx context.Context, tenantID, roleID uuid.UUID) ([]*models.RoleUser, error)
}
