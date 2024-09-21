package interfaces

import "josex/web/models"

type UserRepository interface {
	InsertUser(userDTO models.CreateUserDto) (*models.User, error)
}
