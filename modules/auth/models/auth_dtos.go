package models

import (
	coreModels "josex/web/modules/core/models"

	"github.com/google/uuid"
)

// LoginUserRequestDto is a model for login user request
type LoginUserRequestDto struct {
	Email    string    `json:"email" binding:"required,email-valid" conform:"trim,lowercase"`
	Password string    `json:"password" binding:"required"`
	DeviceId uuid.UUID `json:"device_id,omitempty" binding:"omitempty,uuidv4"`
}

// LoginUserDto is a model for login user request (internal use with device info)
type LoginUserDto struct {
	Email      string    `json:"email" binding:"required,email-valid" conform:"trim,lowercase"`
	Password   string    `json:"password" binding:"required"`
	DeviceId   uuid.UUID `json:"device_id,omitempty" binding:"omitempty,uuidv4"`
	IpAddress  string    `json:"ip_address"`
	DeviceInfo string    `json:"device_info"`
	DeviceOs   string    `json:"device_os"`
	Browser    string    `json:"browser"`
	UserAgent  string    `json:"user_agent"`
}

// LoginExternalRequestDto for OAuth logins (Facebook, Google, etc.)
type LoginExternalRequestDto struct {
	AuthProviderId string               `json:"auth_provider_id" binding:"required" conform:"trim,lowercase"`
	DeviceId       uuid.UUID            `json:"device_id,omitempty" binding:"omitempty,uuidv4"`
	FirstName      *string              `json:"first_name"`
	LastName       *string              `json:"last_name"`
	Email          *string              `form:"email" binding:"omitempty,email-valid" conform:"trim,lowercase"`
	Phone          *string              `json:"phone"`
	Birthday       *coreModels.DateOnly `json:"birthday" time_format:"2006-01-02"`
}

// LoginExternalDto (internal use with device info)
type LoginExternalDto struct {
	AuthProviderName string               `json:"auth_provider_name" conform:"trim,lowercase"`
	AuthProviderId   string               `json:"auth_provider_id" binding:"required" conform:"trim,lowercase"`
	DeviceId         uuid.UUID            `json:"device_id,omitempty" binding:"omitempty,uuidv4"`
	FirstName        *string              `json:"first_name"`
	LastName         *string              `json:"last_name"`
	Email            *string              `form:"email" binding:"omitempty,email-valid" conform:"trim,lowercase"`
	Phone            *string              `json:"phone"`
	Birthday         *coreModels.DateOnly `json:"birthday" time_format:"2006-01-02"`
	IpAddress        string               `json:"ip_address"`
	DeviceInfo       string               `json:"device_info"`
	DeviceOs         string               `json:"device_os"`
	Browser          string               `json:"browser"`
	UserAgent        string               `json:"user_agent"`
}

// LoginSuccessResponse returned after successful login
type LoginSuccessResponse struct {
	AccessToken  *string `json:"access_token"`  // JWT token
	RefreshToken *string `json:"refresh_token"` // JWT token
}

// RefreshTokenRequestDto for token refresh
type RefreshTokenRequestDto struct {
	RefreshToken string `json:"refresh_token"` // JWT token
}

// RefreshTokenResponse after successful refresh
type RefreshTokenResponse struct {
	AccessToken *string `json:"access_token"` // JWT token
}

// LogoutSessionDto for logging out a specific session
type LogoutSessionDto struct {
	UserId    uuid.UUID `json:"user_id" binding:"required,uuidv4"`
	SessionId uuid.UUID `json:"session_id" binding:"required,uuidv4"`
}

// VerifyEmailRequestDto to request email verification
type VerifyEmailRequestDto struct {
	Email *string `json:"email,omitempty" binding:"omitempty,email-valid"`
}

// VerifyEmailRequest (internal use)
type VerifyEmailRequest struct {
	UserId *uuid.UUID `json:"user_id,omitempty" binding:"omitempty,uuidv4"`
	Email  *string    `json:"email,omitempty" binding:"omitempty,email-valid"`
}

// VerifyEmailToken for confirming email verification
type VerifyEmailToken struct {
	Token uuid.UUID `json:"token" binding:"required,uuidv4"`
}

// ChangePasswordRequestDto for password change
type ChangePasswordRequestDto struct {
	PasswordCurrent string `json:"password_current" binding:"required"`
	PasswordNew     string `json:"password_new" binding:"required"`
}

// ChangePasswordDto (internal use)
type ChangePasswordDto struct {
	UserId          *uuid.UUID `json:"user_id,omitempty" binding:"omitempty,uuidv4"`
	PasswordCurrent string     `json:"password_current" binding:"required"`
	PasswordNew     string     `json:"password_new" binding:"required"`
}

// PasswordResetRequestDto to request password reset
type PasswordResetRequestDto struct {
	Email *string `json:"email" binding:"required,email-valid"`
}

// PasswordResetTokenRequestDto for validating reset token
type PasswordResetTokenRequestDto struct {
	Token uuid.UUID `json:"token" binding:"required,uuidv4"`
}

// PasswordResetWithTokenDto to reset password with token
type PasswordResetWithTokenDto struct {
	Token       uuid.UUID `json:"token" binding:"required,uuidv4"`
	PasswordNew string    `json:"password_new" binding:"required"`
}

// GetProfileDto for profile retrieval
type GetProfileDto struct {
	ID uuid.UUID `json:"id" binding:"uuidv4"`
}

// UpdateProfileDto for profile updates
type UpdateProfileDto struct {
	ID                uuid.UUID            `json:"id" binding:"uuidv4"`
	FirstName         *string              `json:"first_name"`
	LastName          *string              `json:"last_name"`
	Phone             *string              `json:"phone"`
	Birthday          *coreModels.DateOnly `json:"birthday" time_format:"2006-01-02"`
	PasswordCurrent   *string              `json:"password_current"`
	PasswordNew       *string              `json:"password_new"`
	ProfilePictureUrl *string              `json:"profile_picture_url"`
	Bio               *string              `json:"bio"`
	WebsiteUrl        *string              `json:"website_url"`
}

// SessionToken for access token
type SessionToken struct {
	AccessToken string `json:"access_token" binding:"required"`
}

// SessionUser represents an authenticated user with session info
type SessionUser struct {
	UserId           uuid.UUID `json:"user_id" binding:"required,uuidv4"`
	SessionId        uuid.UUID `json:"session_id" binding:"required,uuidv4"`
	SystemRole       string    `json:"system_role"`       // super_admin, admin, user
	SubscriptionPlan string    `json:"subscription_plan"` // free, pro, enterprise
	IsActive         bool      `json:"is_active"`
}

// UserSession represents a user's session details
type UserSession struct {
	SessionID  uuid.UUID            `json:"session_id"`
	UserID     uuid.UUID            `json:"user_id"`
	DeviceID   *uuid.UUID           `json:"device_id,omitempty"`
	DeviceOS   string               `json:"device_os"`
	Browser    string               `json:"browser"`
	IpAddress  string               `json:"ip_address"`
	LoginTime  coreModels.DateTime  `json:"login_time"`
	LastActive coreModels.DateTime  `json:"last_active"`
	LogoutTime *coreModels.DateTime `json:"logout_time,omitempty"`
	IsActive   bool                 `json:"is_active"`
}
