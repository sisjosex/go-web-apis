package interfaces

import "josex/web/models"

type UserRepository interface {
	InsertUser(userDTO models.CreateUserDto) (*models.User, error)
	UpdateUser(userDTO models.UpdateUserDto) (*models.User, error)
	UpdateProfile(userDTO models.UpdateProfileDto) (*models.User, error)
	LoginUser(userDTO models.LoginUserDto) (*models.SessionUser, error)
	LoginExternal(userDTO models.LoginExternalDto) (*models.SessionUser, error)
	LogoutUser(userDTO models.SessionUser) (*models.SessionUser, error)
}
