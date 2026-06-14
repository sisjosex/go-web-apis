package models

import (
	"time"

	"github.com/google/uuid"
)

// Role represents a custom tenant role.
type Role struct {
	ID          uuid.UUID `json:"id"`
	TenantID    uuid.UUID `json:"tenant_id"`
	Name        string    `json:"name"`
	Description *string   `json:"description,omitempty"`
	IsSystem    bool      `json:"is_system"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// RoleListItem is a role with permission count (for list endpoints).
type RoleListItem struct {
	ID              uuid.UUID `json:"id"`
	TenantID        uuid.UUID `json:"tenant_id"`
	Name            string    `json:"name"`
	Description     *string   `json:"description,omitempty"`
	IsSystem        bool      `json:"is_system"`
	PermissionCount int64     `json:"permission_count"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// RoleDetail is a role with its permission codes (for single-role retrieval).
type RoleDetail struct {
	ID          uuid.UUID `json:"id"`
	TenantID    uuid.UUID `json:"tenant_id"`
	Name        string    `json:"name"`
	Description *string   `json:"description,omitempty"`
	IsSystem    bool      `json:"is_system"`
	Permissions []string  `json:"permissions"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// UserRole represents a role assigned to a user within a tenant.
type UserRole struct {
	RoleID      uuid.UUID `json:"role_id"`
	RoleName    string    `json:"role_name"`
	Description *string   `json:"description,omitempty"`
	IsSystem    bool      `json:"is_system"`
	AssignedBy  uuid.UUID `json:"assigned_by"`
	AssignedAt  time.Time `json:"assigned_at"`
}

// CreateRoleDto for creating a custom role.
type CreateRoleDto struct {
	Name        string    `json:"name"        binding:"required" conform:"trim"`
	Description *string   `json:"description" binding:"omitempty" conform:"trim"`
	TenantID    uuid.UUID `json:"-"` // set server-side from middleware
}

// UpdateRoleDto for updating a custom role.
type UpdateRoleDto struct {
	Name        *string   `json:"name"        binding:"omitempty" conform:"trim"`
	Description *string   `json:"description" binding:"omitempty" conform:"trim"`
	TenantID    uuid.UUID `json:"-"` // set server-side from middleware
}

// SetRolePermissionsDto for replacing all permissions of a role.
type SetRolePermissionsDto struct {
	Permissions []string  `json:"permissions" binding:"required"`
	TenantID    uuid.UUID `json:"-"` // set server-side from middleware
}
