package models

import "time"

type User struct {
	ID                string     `json:"id" binding:"required"`
	FirstName         *string    `json:"first_name" binding:"required"`
	LastName          *string    `json:"last_name" binding:"required"`
	Email             *string    `json:"email" binding:"required,email-valid" conform:"trim,lowercase"`
	Phone             *string    `json:"phone"`
	Birthday          *DateOnly  `json:"birthday"`
	ProfilePictureUrl *string    `json:"profile_picture_url"`
	Bio               *string    `json:"bio"`
	WebsiteUrl        *string    `json:"website_url"`
	IsActive          *bool      `json:"is_active"`
	CreatedAt         *time.Time `json:"created_at"`
	ExpirationDate    *DateOnly  `json:"expiration_date"`
	TenantRole        *string    `json:"tenant_role,omitempty"`
	// SystemRole is the platform role the token carries (super_admin, admin, user): the app shows the
	// platform's own views by it (BILLING-002). Set from the token, not read from the database.
	SystemRole *string `json:"system_role,omitempty"`
	// Locale is the language the account reads the app in; only the profile read fills it (APP-012).
	Locale *string `json:"locale,omitempty"`
}
