package interfaces

import (
	"context"

	coreModels "josex/web/modules/core/models"
	userModels "josex/web/modules/users/models"

	"github.com/google/uuid"
)

// UserRepository handles user CRUD operations (admin)
type UserRepository interface {
	InsertUser(userDTO userModels.CreateUserDto) (*coreModels.User, error)
	UpdateUser(userDTO userModels.UpdateUserDto) (*coreModels.User, error)
	ListUsers(query userModels.UserListQuery) (*userModels.UserListResponse, error)
	GetUserById(tenantID, userID uuid.UUID) (*coreModels.User, error)
	GetStats(tenantID uuid.UUID, excludeUserID *uuid.UUID) (*userModels.UserStatsResponse, error)
	SoftDeleteUser(tenantID, userID uuid.UUID) error
	AssignToTenant(tenantID, requesterID, userID uuid.UUID, role string) error
	EnsureTenantSeat(ctx context.Context, tenantID uuid.UUID, limit int) error
	ResetPasswordToken(tenantID, userID uuid.UUID) (*userModels.ResetPasswordResult, error)
}
