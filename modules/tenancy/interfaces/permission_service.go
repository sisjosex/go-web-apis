package interfaces

import (
	"context"

	"josex/web/modules/tenancy/models"

	"github.com/google/uuid"
)

// PermissionService defines business logic for permission management
type PermissionService interface {
	// AssignPermission grants a permission to a user within a tenant.
	AssignPermission(ctx context.Context, tenantID, requesterID, targetUserID uuid.UUID, permCode string) error

	// RevokePermission removes a permission from a user within a tenant.
	RevokePermission(ctx context.Context, tenantID, requesterID, targetUserID uuid.UUID, permCode string) error

	// ListUserPermissions returns all permissions assigned to a user in a tenant,
	// enriched with metadata from the global registry.
	ListUserPermissions(ctx context.Context, userID, tenantID uuid.UUID) ([]*models.UserPermissionResponse, error)

	// ListAvailablePermissions returns all registered permissions, optionally
	// filtered by module name. Pass empty string to return all.
	ListAvailablePermissions(module string) []models.AvailablePermission
}
