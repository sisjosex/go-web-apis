package models

import "github.com/google/uuid"

type SessionUser struct {
	UserId    uuid.UUID `json:"user_id" binding:"required,uuidv4"`
	SessionId uuid.UUID `json:"session_id" binding:"required,uuidv4"`
	IsActive  bool      `json:"is_active"`
}

type LogoutSessionDto struct {
	UserId    uuid.UUID `json:"user_id" binding:"required,uuidv4"`
	SessionId uuid.UUID `json:"session_id" binding:"required,uuidv4"`
}

type VerifyEmailRequestDto struct {
	// Send only for request edit primary email address
	Email *string `json:"email,omitempty" binding:"omitempty,email-valid"`
}

type VerifyEmailRequest struct {
	UserId *uuid.UUID `json:"user_id,omitempty" binding:"omitempty,uuidv4"`
	Email  *string    `json:"email,omitempty" binding:"omitempty,email-valid"`
}

type VerifyEmailToken struct {
	Token uuid.UUID `json:"token" binding:"required,uuidv4"`
}

type ChangePasswordRequestDto struct {
	PasswordCurrent string `json:"password_current" binding:"required"`
	PasswordNew     string `json:"password_new" binding:"required"`
}

type ChangePasswordDto struct {
	UserId          *uuid.UUID `json:"user_id,omitempty" binding:"omitempty,uuidv4"`
	PasswordCurrent string     `json:"password_current" binding:"required"`
	PasswordNew     string     `json:"password_new" binding:"required"`
}

type PasswordResetRequestDto struct {
	Email *string `json:"email" binding:"required,email-valid"`
}

type PasswordResetTokenRequestDto struct {
	Token uuid.UUID `json:"token" binding:"required,uuidv4"`
}

type PasswordResetWithTokenDto struct {
	// Password reset token, see /auth/request_password_reset
	Token uuid.UUID `json:"token" binding:"required,uuidv4"`
	// New password
	PasswordNew string `json:"password_new" binding:"required"`
}
