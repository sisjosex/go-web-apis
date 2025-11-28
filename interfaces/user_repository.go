package interfaces

import (
	"josex/web/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type UserRepository interface {
	InsertUser(userDTO models.CreateUserDto) (*models.User, error)
	UpdateUser(userDTO models.UpdateUserDto) (*models.User, error)
	UpdateProfile(userDTO models.UpdateProfileDto) (*models.User, error)
	LoginUser(userDTO models.LoginUserDto) (*models.SessionUser, error)
	LoginExternal(userDTO models.LoginExternalDto) (*models.SessionUser, error)
	LogoutUser(userDTO models.LogoutSessionDto) (*bool, error)
	GetProfile(userDTO models.GetProfileDto) (*models.User, error)
	ValidateSession(userID uuid.UUID, sessionID uuid.UUID) error

	// Session management
	GetUserSessions(userID uuid.UUID) ([]models.UserSession, error)
	LogoutSession(userID uuid.UUID, sessionID uuid.UUID) error
	LogoutAllSessions(userID uuid.UUID, currentSessionID *uuid.UUID) (int, error)

	// User management
	ListUsers(query models.UserListQuery) (*models.UserListResponse, error)
	GetUserById(userID uuid.UUID) (*models.User, error)
	SoftDeleteUser(userID uuid.UUID, reason string) error

	GenerateEmailVerificationToken(verifyEmailRequest models.VerifyEmailRequest, tx pgx.Tx) (*models.VerifyEmailToken, error)
	ConfirmEmailAddress(verifyEmailRequest models.VerifyEmailToken) (*bool, error)
	ChangePassword(changePasswordDto models.ChangePasswordDto) (*bool, error)
	GeneratePasswordResetToken(passwordResetRequestDto models.PasswordResetRequestDto, tx pgx.Tx) (*models.PasswordResetTokenRequestDto, error)
	ResetPasswordWithToken(passwordResetWithTokenDto models.PasswordResetWithTokenDto) (*bool, error)
}
