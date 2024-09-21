package interfaces

import (
	"josex/web/models"
)

type UserService interface {
	InsertUser(userDTO models.CreateUserDto) (*models.User, error)
}
