package interfaces

import (
	"context"

	"josex/web/modules/tenancy/models"

	"github.com/google/uuid"
)

// PermissionRepository defines methods for permission data access
type PermissionRepository interface {
	// AssignPermission grants a permission to a user within a tenant.
	// Idempotent: succeeds silently if already assigned.
	AssignPermission(ctx context.Context, tenantID, requesterID, targetUserID uuid.UUID, permCode string) error

	// RevokePermission removes a permission from a user within a tenant.
	// Idempotent: succeeds silently if not assigned.
	RevokePermission(ctx context.Context, tenantID, requesterID, targetUserID uuid.UUID, permCode string) error

	// ListUserPermissions returns all permissions assigned to a user in a tenant.
	ListUserPermissions(ctx context.Context, userID, tenantID uuid.UUID) ([]*models.UserPermissionResponse, error)
}
