package models

import (
	coreModels "josex/web/modules/core/models"
	"time"

	"github.com/google/uuid"
)

// LoginUserRequestDto is a model for login user request
type LoginUserRequestDto struct {
	Email    string     `json:"email" binding:"required,email-valid" conform:"trim,lowercase"`
	Password string     `json:"password" binding:"required"`
	DeviceId *uuid.UUID `json:"device_id,omitempty" binding:"omitempty,uuidv4"`
}

// LoginUserDto is a model for login user request (internal use with device info)
type LoginUserDto struct {
	Email      string     `json:"email" binding:"required,email-valid" conform:"trim,lowercase"`
	Password   string     `json:"password" binding:"required"`
	DeviceId   *uuid.UUID `json:"device_id,omitempty" binding:"omitempty,uuidv4"`
	IpAddress  string     `json:"ip_address"`
	DeviceInfo string     `json:"device_info"`
	DeviceOs   string     `json:"device_os"`
	Browser    string     `json:"browser"`
	UserAgent  string     `json:"user_agent"`
	ClientType string     `json:"client_type"` // web | mobile, from X-Client-Type (TRACK-015 D2)
}

// LoginExternalRequestDto for OAuth logins (Facebook, Google, etc.)
type LoginExternalRequestDto struct {
	AuthProviderId string               `json:"auth_provider_id" binding:"required" conform:"trim,lowercase"`
	DeviceId       *uuid.UUID           `json:"device_id,omitempty" binding:"omitempty,uuidv4"`
	FirstName      *string              `json:"first_name"`
	LastName       *string              `json:"last_name"`
	Email          *string              `json:"email" binding:"required,email-valid" conform:"trim,lowercase"`
	Phone          *string              `json:"phone"`
	Birthday       *coreModels.DateOnly `json:"birthday" time_format:"2006-01-02"`
}

// LoginExternalDto (internal use with device info)
type LoginExternalDto struct {
	AuthProviderName string               `json:"auth_provider_name" conform:"trim,lowercase"`
	AuthProviderId   string               `json:"auth_provider_id" binding:"required" conform:"trim,lowercase"`
	DeviceId         *uuid.UUID           `json:"device_id,omitempty" binding:"omitempty,uuidv4"`
	FirstName        *string              `json:"first_name"`
	LastName         *string              `json:"last_name"`
	Email            *string              `json:"email" binding:"required,email-valid" conform:"trim,lowercase"`
	Phone            *string              `json:"phone"`
	Birthday         *coreModels.DateOnly `json:"birthday" time_format:"2006-01-02"`
	IpAddress        string               `json:"ip_address"`
	DeviceInfo       string               `json:"device_info"`
	DeviceOs         string               `json:"device_os"`
	Browser          string               `json:"browser"`
	UserAgent        string               `json:"user_agent"`
	ClientType       string               `json:"client_type"` // web | mobile, from X-Client-Type (TRACK-015 D2)
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
	// RefreshToken replaces the one sent: each refresh restarts its lifetime, so a client in use
	// never has to sign in again (MOBILE-011 D4). The old one stays valid until it expires.
	RefreshToken *string `json:"refresh_token"`
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
	// The caller's session: it survives the change, every other session of the user ends.
	CurrentSessionId *uuid.UUID `json:"-"`
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

// ValidateResetTokenDto to validate a password reset token (query param)
type ValidateResetTokenDto struct {
	Token string `form:"token" binding:"required"`
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
	// IsCurrent marks the session the request was made with (MOBILE-011: "This device").
	IsCurrent bool `json:"is_current"`
}

// ============================================================================
// OTP DTOs - Multi-channel authentication (WhatsApp, SMS, Email)
// ============================================================================

// RequestOtpRequestDto is the request DTO for requesting an OTP code
// destination can be a phone number (e.g., "+1234567890") or email (e.g., "user@example.com")
// channel specifies the delivery method: "whatsapp", "sms", "email"
type RequestOtpRequestDto struct {
	Destination string     `json:"destination" binding:"required" conform:"trim"`                   // phone or email
	Channel     string     `json:"channel" binding:"required,otp-channel" conform:"trim,lowercase"` // whatsapp, sms, email
	DeviceId    *uuid.UUID `json:"device_id,omitempty" binding:"omitempty,uuidv4"`
}

// RequestOtpDto is the internal DTO with device information
type RequestOtpDto struct {
	Destination string     `json:"destination"`
	Channel     string     `json:"channel"`
	DeviceId    *uuid.UUID `json:"device_id,omitempty"`
	IpAddress   string     `json:"ip_address"`
	DeviceInfo  string     `json:"device_info"`
	DeviceOs    string     `json:"device_os"`
	Browser     string     `json:"browser"`
	UserAgent   string     `json:"user_agent"`
}

// RequestOtpResponse is the response after successfully requesting an OTP
type RequestOtpResponse struct {
	OtpId        string    `json:"otp_id"`
	Destination  string    `json:"destination"` // masked for security (e.g., "+12345****90")
	Channel      string    `json:"channel"`
	DeliveredVia string    `json:"delivered_via"` // channel that carried the code; "email" when a phone login is relayed
	ExpiresAt    time.Time `json:"expires_at"`
	Message      string    `json:"message"`
}

// VerifyOtpRequestDto is the request DTO for verifying an OTP code
type VerifyOtpRequestDto struct {
	Destination string     `json:"destination" binding:"required" conform:"trim"`
	OtpCode     string     `json:"otp_code" binding:"required,len=6,numeric" conform:"trim"` // Exactly 6 digits
	Channel     string     `json:"channel" binding:"required,otp-channel" conform:"trim,lowercase"`
	DeviceId    *uuid.UUID `json:"device_id,omitempty" binding:"omitempty,uuidv4"`
}

// VerifyOtpDto is the internal DTO with device information
type VerifyOtpDto struct {
	Destination string
	OtpCode     string
	Channel     string
	DeviceId    *uuid.UUID
	IpAddress   string
	DeviceInfo  string
	DeviceOs    string
	Browser     string
	UserAgent   string
	ClientType  string // web | mobile, from X-Client-Type (TRACK-015 D2)
}

// VerifyOtpResponse is the response after successfully verifying an OTP
// Only returns JWT tokens - JWT contains all necessary info (user_id, session_id, claims, etc.)
// If OTP is invalid/expired, API returns error instead
type VerifyOtpResponse struct {
	AccessToken  *string   `json:"access_token"`  // JWT access token
	RefreshToken *string   `json:"refresh_token"` // JWT refresh token
	SystemRole   string    `json:"-"`             // Internal field for JWT generation (not exposed in JSON)
	SessionId    uuid.UUID `json:"-"`             // Internal field for JWT generation (not exposed in JSON)
	UserId       uuid.UUID `json:"-"`             // Internal field for JWT generation (not exposed in JSON)
}

// OtpRequest represents an OTP record from the database
type OtpRequest struct {
	Id          uuid.UUID  `db:"id"`
	Destination string     `db:"destination"`
	OtpChannel  string     `db:"otp_channel"`
	OtpCode     string     `db:"otp_code"`
	RelayEmail  *string    `db:"relay_email"` // set only when a phone channel is relayed by email
	UserId      *uuid.UUID `db:"user_id"`
	IsVerified  bool       `db:"is_verified"`
	VerifiedAt  *time.Time `db:"verified_at"`
	Attempts    int        `db:"attempts"`
	MaxAttempts int        `db:"max_attempts"`
	CreatedAt   time.Time  `db:"created_at"`
	ExpiresAt   time.Time  `db:"expires_at"`
}
