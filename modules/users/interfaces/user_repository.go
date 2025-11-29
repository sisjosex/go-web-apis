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
	ListUsers() ([]coreModels.User, error)
	GetUserById(userID uuid.UUID) (*coreModels.User, error)
	SoftDeleteUser(userID uuid.UUID) error
}
