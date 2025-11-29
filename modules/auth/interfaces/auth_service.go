package interfaces

import (
	authModels "josex/web/modules/auth/models"
	coreModels "josex/web/modules/core/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// AuthService handles authentication business logic
type AuthService interface {
	// User registration
	InsertUser(userDTO authModels.CreateUserDto) (*coreModels.User, error)

	// Authentication
	LoginUser(userDTO authModels.LoginUserDto) (*authModels.SessionUser, error)
	LoginExternal(userDTO authModels.LoginExternalDto) (*authModels.SessionUser, error)
	LogoutUser(userDTO authModels.LogoutSessionDto) (*bool, error)

	// Profile management
	GetProfile(userDTO authModels.GetProfileDto) (*coreModels.User, error)
	UpdateProfile(userDTO authModels.UpdateProfileDto) (*coreModels.User, error)

	// Session management
	ValidateSession(userID uuid.UUID, sessionID uuid.UUID) error
	GetUserSessions(userID uuid.UUID) ([]authModels.UserSession, error)
	LogoutSession(userID uuid.UUID, sessionID uuid.UUID) error
	LogoutAllSessions(userID uuid.UUID, currentSessionID *uuid.UUID) (int, error)

	// Email verification
	GenerateEmailVerificationToken(verifyEmailRequest authModels.VerifyEmailRequest, tx pgx.Tx) (*authModels.VerifyEmailToken, error)
	ConfirmEmailAddress(verifyEmailRequest authModels.VerifyEmailToken) (*bool, error)

	// Password management
	ChangePassword(changePasswordDto authModels.ChangePasswordDto) (*bool, error)
	GeneratePasswordResetToken(passwordResetRequestDto authModels.PasswordResetRequestDto, tx pgx.Tx) (*authModels.PasswordResetTokenRequestDto, error)
	ResetPasswordWithToken(passwordResetWithTokenDto authModels.PasswordResetWithTokenDto) (*bool, error)
}
