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
	Password          string               `json:"password"`
	Phone             string               `json:"phone"`
	Birthday          *coreModels.DateOnly `json:"birthday" time_format:"2006-01-02"`
	ProfilePictureUrl string               `json:"profile_picture_url"`
	Bio               string               `json:"bio"`
	WebsiteUrl        string               `json:"website_url"`
	// TenantRole is the access level the user joins the tenant with; empty means member (USERS-011).
	// Owner is a promotion with its own confirmation and portal comes from the guardian flow, so neither is accepted here.
	TenantRole string `json:"tenant_role" binding:"omitempty,oneof=admin member organization driver"`
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
	TenantID          uuid.UUID            `json:"-"` // set server-side from tenancy middleware
	PerformedBy       *uuid.UUID           `json:"-"` // set server-side from JWT middleware
}

// UserListQuery represents query parameters for listing users
type UserListQuery struct {
	Page          int        `form:"page" json:"page" binding:"omitempty,min=1"`
	Limit         int        `form:"limit" json:"limit" binding:"omitempty,min=1,max=100"`
	Search        string     `form:"search" json:"search"`
	Status        string     `form:"status" json:"status" binding:"omitempty,oneof=active inactive expired"`
	Sort          string     `form:"sort" json:"sort" binding:"omitempty,oneof=created_at email first_name last_name"`
	Order         string     `form:"order" json:"order" binding:"omitempty,oneof=asc desc"`
	TenantID      uuid.UUID  `json:"-"` // Set server-side from tenancy middleware; not exposed to clients
	ExcludeUserID *uuid.UUID `json:"-"` // Set server-side from JWT; not exposed to clients
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

// UserStatsResponse represents user count statistics for a tenant
type UserStatsResponse struct {
	Total    int64 `json:"total"`
	Active   int64 `json:"active"`
	Inactive int64 `json:"inactive"`
	Expired  int64 `json:"expired"`
}

// ResetPasswordResult carries the data needed to send a password reset email
type ResetPasswordResult struct {
	Email string
	Token string
}
