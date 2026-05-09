package models

import (
	"time"

	"github.com/google/uuid"
)

// AssignPermissionDto for assigning a permission to a user
type AssignPermissionDto struct {
	PermissionCode string `json:"permission_code" binding:"required" conform:"trim"`
}

// RevokePermissionDto for revoking a permission from a user
type RevokePermissionDto struct {
	PermissionCode string `json:"permission_code" binding:"required" conform:"trim"`
}

// UserPermissionResponse represents a permission assigned to a user
type UserPermissionResponse struct {
	Code      string    `json:"code"`
	Module    string    `json:"module"`
	Name      string    `json:"name"`
	GrantedBy uuid.UUID `json:"granted_by"`
	GrantedAt time.Time `json:"granted_at"`
}

// AvailablePermission represents a permission available to assign
type AvailablePermission struct {
	Code        string `json:"code"`
	Module      string `json:"module"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}
