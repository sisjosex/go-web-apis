package interfaces

import (
	"josex/web/models"
)

type UserService interface {
	InsertUser(userDTO models.CreateUserDto) (*models.User, error)
	UpdateUser(userDTO models.UpdateUserDto) (*models.User, error)
	LoginUser(userDTO models.LoginUserDto) (*models.SessionToken, error)
	LoginExternal(userDTO models.LoginExternalDto) (*models.SessionToken, error)
	LogoutUser(userDTO models.SessionUser) (*models.SessionUser, error)
}
