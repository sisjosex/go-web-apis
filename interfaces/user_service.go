package interfaces

import (
	"josex/web/models"
)

type UserService interface {
	InsertUser(userDTO models.CreateUserDto) (*models.User, error)
	UpdateUser(userDTO models.UpdateUserDto) (*models.User, error)
	UpdateProfile(userDTO models.UpdateProfileDto) (*models.User, error)
	LoginUser(userDTO models.LoginUserDto) (*models.SessionUser, error)
	LoginExternal(userDTO models.LoginExternalDto) (*models.SessionUser, error)
	LogoutUser(userDTO models.SessionUser) (*models.SessionUser, error)
	GetProfile(userDTO models.GetProfileDto) (*models.User, error)
	GenerateEmailVerificationToken(verifyEmailRequest models.VerifyEmailRequest) (*models.VerifyEmailToken, error)
	ConfirmEmailAddress(verifyEmailRequest models.VerifyEmailToken) (*bool, error)
	ChangePassword(changePasswordDto models.ChangePasswordDto) (*bool, error)
	GeneratePasswordResetToken(passwordResetRequestDto models.PasswordResetRequestDto) (*models.PasswordResetTokenRequestDto, error)
	ResetPasswordWithToken(passwordResetWithTokenDto models.PasswordResetWithTokenDto) (*bool, error)
}
