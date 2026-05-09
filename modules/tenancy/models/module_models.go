package models

import (
	"time"

	"github.com/google/uuid"
)

// TenantModule represents a module enabled for a tenant.
type TenantModule struct {
	TenantID   uuid.UUID  `json:"tenant_id"`
	ModuleCode string     `json:"module_code"`
	IsEnabled  bool       `json:"is_enabled"`
	Config     string     `json:"config"` // JSONB stored as string
	EnabledAt  time.Time  `json:"enabled_at"`
	EnabledBy  *uuid.UUID `json:"enabled_by,omitempty"`
}

// UpsertTenantModuleDto is the request body for enabling/updating a module.
type UpsertTenantModuleDto struct {
	IsEnabled bool    `json:"is_enabled"`
	Config    *string `json:"config,omitempty"`
}

// TenantModuleResponse is the API response for a tenant module.
type TenantModuleResponse struct {
	ModuleCode string    `json:"module_code"`
	IsEnabled  bool      `json:"is_enabled"`
	Config     string    `json:"config"`
	EnabledAt  time.Time `json:"enabled_at"`
}
