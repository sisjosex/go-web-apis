package interfaces

import (
	coreModels "josex/web/modules/core/models"
	userModels "josex/web/modules/users/models"

	"github.com/google/uuid"
)

// UserRepository handles user CRUD operations (admin)
type UserRepository interface {
	InsertUser(userDTO userModels.CreateUserDto) (*coreModels.User, error)
	UpdateUser(userDTO userModels.UpdateUserDto) (*coreModels.User, error)
	ListUsers(query userModels.UserListQuery) (*userModels.UserListResponse, error)
	GetUserById(userID uuid.UUID) (*coreModels.User, error)
	GetStats(tenantID uuid.UUID, excludeUserID *uuid.UUID) (*userModels.UserStatsResponse, error)
	SoftDeleteUser(userID uuid.UUID) error
	AssignToTenant(tenantID, requesterID, userID uuid.UUID, role string) error
	ResetPasswordToken(userID uuid.UUID) (*userModels.ResetPasswordResult, error)
}
