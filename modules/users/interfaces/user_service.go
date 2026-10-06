package interfaces

import (
	"context"

	coreModels "josex/web/modules/core/models"
	userModels "josex/web/modules/users/models"

	"github.com/google/uuid"
)

// UserService handles user management business logic (admin)
type UserService interface {
	InsertUser(userDTO userModels.CreateUserDto) (*coreModels.User, error)
	UpdateUser(userDTO userModels.UpdateUserDto) (*coreModels.User, error)
	ListUsers(query userModels.UserListQuery) (*userModels.UserListResponse, error)
	GetUserById(userID uuid.UUID) (*coreModels.User, error)
	GetStats(tenantID uuid.UUID, excludeUserID *uuid.UUID) (*userModels.UserStatsResponse, error)
	SoftDeleteUser(userID uuid.UUID) error
	AssignToTenant(tenantID, requesterID, userID uuid.UUID, role string) error
	// EnsureTenantSeat refuses when the workspace's plan has no member seat left (BILLING-001 D2).
	EnsureTenantSeat(ctx context.Context, tenantID uuid.UUID) error
	ResetPassword(userID uuid.UUID) (*userModels.ResetPasswordResult, error)
}
