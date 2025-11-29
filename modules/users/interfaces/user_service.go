package interfaces

import (
	coreModels "josex/web/modules/core/models"
	userModels "josex/web/modules/users/models"

	"github.com/google/uuid"
)

// UserService handles user management business logic (admin)
type UserService interface {
	InsertUser(userDTO userModels.CreateUserDto) (*coreModels.User, error)
	UpdateUser(userDTO userModels.UpdateUserDto) (*coreModels.User, error)
	ListUsers() ([]coreModels.User, error)
	GetUserById(userID uuid.UUID) (*coreModels.User, error)
	SoftDeleteUser(userID uuid.UUID) error
}
