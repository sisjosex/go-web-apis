package models

import "github.com/google/uuid"

type UpdateUserDto struct {
	ID                uuid.UUID `json:"id" binding:"uuidv4"`
	FirstName         *string   `json:"first_name"`
	LastName          *string   `json:"last_name"`
	Phone             *string   `json:"phone"`
	Birthday          *DateOnly `json:"birthday" time_format:"2006-01-02"`
	Email             *string   `form:"email" binding:"omitempty,email-valid" conform:"trim,lowercase"`
	PasswordCurrent   *string   `json:"password_current"`
	PasswordNew       *string   `json:"password_new"`
	IsActive          *bool     `json:"is_active"`
	IsVerified        *bool     `json:"is_verified"`
	ExpirationDate    *DateOnly `json:"expiration_date" time_format:"2006-01-02"`
	ProfilePictureUrl *string   `json:"profile_picture_url"`
	Bio               *string   `json:"bio"`
	WebsiteUrl        *string   `json:"website_url"`
}
