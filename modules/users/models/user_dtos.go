package models

import (
	coreModels "josex/web/modules/core/models"

	"github.com/google/uuid"
)

// CreateUserDto for creating a new user (admin operation)
type CreateUserDto struct {
	FirstName         string               `json:"first_name" binding:"required"`
	LastName          string               `json:"last_name" binding:"required"`
	Email             string               `form:"email" binding:"required,email-valid" conform:"trim,lowercase"`
	Password          string               `json:"password" binding:"required"`
	Phone             string               `json:"phone"`
	Birthday          *coreModels.DateOnly `json:"birthday" time_format:"2006-01-02"`
	ProfilePictureUrl string               `json:"profile_picture_url"`
	Bio               string               `json:"bio"`
	WebsiteUrl        string               `json:"website_url"`
}

// UpdateUserDto for updating a user (admin operation)
type UpdateUserDto struct {
	ID                uuid.UUID            `json:"id" binding:"uuidv4"`
	FirstName         *string              `json:"first_name"`
	LastName          *string              `json:"last_name"`
	Phone             *string              `json:"phone"`
	Birthday          *coreModels.DateOnly `json:"birthday" time_format:"2006-01-02"`
	Email             *string              `form:"email" binding:"omitempty,email-valid" conform:"trim,lowercase"`
	PasswordCurrent   *string              `json:"password_current"`
	PasswordNew       *string              `json:"password_new"`
	IsActive          *bool                `json:"is_active"`
	ExpirationDate    *coreModels.DateOnly `json:"expiration_date" time_format:"2006-01-02"`
	ProfilePictureUrl *string              `json:"profile_picture_url"`
	Bio               *string              `json:"bio"`
	WebsiteUrl        *string              `json:"website_url"`
}

// UserListQuery represents query parameters for listing users
type UserListQuery struct {
	Page   int    `form:"page" json:"page" binding:"omitempty,min=1"`                                       // Page number (default: 1)
	Limit  int    `form:"limit" json:"limit" binding:"omitempty,min=1,max=100"`                             // Items per page (default: 10, max: 100)
	Search string `form:"search" json:"search"`                                                             // Search in name/email
	Status string `form:"status" json:"status" binding:"omitempty,oneof=active inactive expired"`           // Filter by status
	Sort   string `form:"sort" json:"sort" binding:"omitempty,oneof=created_at email first_name last_name"` // Sort field
	Order  string `form:"order" json:"order" binding:"omitempty,oneof=asc desc"`                            // Sort order (asc/desc)
}

// UserListResponse represents paginated user list response
type UserListResponse struct {
	Users      []coreModels.User `json:"users"`
	Total      int64             `json:"total"`       // Total count of users
	Page       int               `json:"page"`        // Current page
	Limit      int               `json:"limit"`       // Items per page
	TotalPages int               `json:"total_pages"` // Total pages
}

// SoftDeleteUserDto represents soft delete request
type SoftDeleteUserDto struct {
	ID     string `json:"id" binding:"required,uuidv4"`
	Reason string `json:"reason" binding:"omitempty,max=500"` // Optional deletion reason
}
