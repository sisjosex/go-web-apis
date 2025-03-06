package models

import "github.com/google/uuid"

type SessionUser struct {
	UserId    uuid.UUID `json:"user_id" binding:"required,uuidv4"`
	SessionId uuid.UUID `json:"session_id" binding:"required,uuidv4"`
	IsActive  bool      `json:"is_active"`
}

type VerifyEmailRequest struct {
	UserId *uuid.UUID `json:"user_id,omitempty" binding:"omitempty,uuidv4"`
	Email  *string    `json:"email,omitempty" binding:"omitempty,email-valid"`
}

type VerifyEmailToken struct {
	Token uuid.UUID `json:"token" binding:"required,uuidv4"`
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
	Token       uuid.UUID `json:"token" binding:"required,uuidv4"`
	PasswordNew string    `json:"password_new" binding:"required"`
}
