package models

import (
	"time"

	"github.com/google/uuid"
)

// Tenant represents a tenant in the system
type Tenant struct {
	ID              uuid.UUID `json:"id" db:"id"`
	Slug            string    `json:"slug" db:"slug"`
	Name            string    `json:"name" db:"name"`
	DatabaseURL     *string   `json:"database_url,omitempty" db:"database_url"`
	SchemaName      string    `json:"schema_name" db:"schema_name"`
	IsActive        bool      `json:"is_active" db:"is_active"`
	IsSuspended     bool      `json:"is_suspended" db:"is_suspended"`
	SuspendedReason *string   `json:"suspended_reason,omitempty" db:"suspended_reason"`
	Settings        *string   `json:"settings,omitempty" db:"settings"` // JSONB as string
	CreatedAt       time.Time `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time `json:"updated_at" db:"updated_at"`
}

// TenantUser represents the relationship between users and tenants
type TenantUser struct {
	ID       uuid.UUID `json:"id" db:"id"`
	TenantID uuid.UUID `json:"tenant_id" db:"tenant_id"`
	UserID   uuid.UUID `json:"user_id" db:"user_id"`
	Role     string    `json:"role" db:"role"`
	IsActive bool      `json:"is_active" db:"is_active"`
	JoinedAt time.Time `json:"joined_at" db:"joined_at"`
}

// CreateTenantDto for creating a new tenant
type CreateTenantDto struct {
	Slug        string  `json:"slug" binding:"required" conform:"trim,lowercase"`
	Name        string  `json:"name" binding:"required" conform:"trim"`
	DatabaseURL *string `json:"database_url,omitempty" conform:"trim"`
	SchemaName  *string `json:"schema_name,omitempty" conform:"trim,lowercase"`
	Settings    *string `json:"settings,omitempty"` // JSONB as string
}

// UpdateTenantDto for updating tenant information
type UpdateTenantDto struct {
	Name        *string `json:"name,omitempty" conform:"trim"`
	DatabaseURL *string `json:"database_url,omitempty" conform:"trim"`
	SchemaName  *string `json:"schema_name,omitempty" conform:"trim,lowercase"`
	Settings    *string `json:"settings,omitempty"` // JSONB as string
}

// TenantResponse - safe response without sensitive data
type TenantResponse struct {
	ID          uuid.UUID `json:"id"`
	Slug        string    `json:"slug"`
	Name        string    `json:"name"`
	IsActive    bool      `json:"is_active"`
	IsSuspended bool      `json:"is_suspended"`
	CreatedAt   time.Time `json:"created_at"`
}

// TenantDetailResponse - for authenticated tenant members
type TenantDetailResponse struct {
	ID          uuid.UUID `json:"id"`
	Slug        string    `json:"slug"`
	Name        string    `json:"name"`
	SchemaName  string    `json:"schema_name"`
	IsActive    bool      `json:"is_active"`
	IsSuspended bool      `json:"is_suspended"`
	Settings    *string   `json:"settings,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// AddUserToTenantDto for adding a user to a tenant
type AddUserToTenantDto struct {
	UserID uuid.UUID `json:"user_id" binding:"required,uuidv4"`
	Role   string    `json:"role" binding:"required" conform:"trim,lowercase"`
}

// RemoveUserFromTenantDto for removing a user from a tenant
type RemoveUserFromTenantDto struct {
	UserID uuid.UUID `json:"user_id" binding:"required,uuidv4"`
}

// UserTenantResponse - tenant info for user's accessible tenants
type UserTenantResponse struct {
	TenantID    uuid.UUID `json:"tenant_id"`
	Slug        string    `json:"slug"`
	Name        string    `json:"name"`
	IsActive    bool      `json:"is_active"`
	IsSuspended bool      `json:"is_suspended"`
	UserRole    string    `json:"user_role"`
	JoinedAt    time.Time `json:"joined_at"`
}

// TenantAccessInfo - stored in context by middleware
type TenantAccessInfo struct {
	TenantID     uuid.UUID `json:"tenant_id"`
	Slug         string    `json:"slug"`
	Name         string    `json:"name"`
	DatabaseURL  *string   `json:"database_url,omitempty"`
	SchemaName   string    `json:"schema_name"`
	IsActive     bool      `json:"is_active"`
	IsSuspended  bool      `json:"is_suspended"`
	UserRole     string    `json:"user_role"`
	UserIsActive bool      `json:"user_is_active"`
}
