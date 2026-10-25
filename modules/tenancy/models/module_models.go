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

// SetVerticalsDto is PUT /tenants/:slug/vertical (TENANCY-006): the business types picked, the suggested
// modules switched on, whether to seed the starter categories (the sign-up only) and the main module.
// Codes may be empty when only PrimaryModule changes.
type SetVerticalsDto struct {
	Codes         []string `json:"codes"`
	ExtraModules  []string `json:"extra_modules"`
	Starter       bool     `json:"starter"`
	PrimaryModule string   `json:"primary_module,omitempty" conform:"trim,lowercase"`
}

// VerticalsResponse is what the preset left on: the stored types and main module, every enabled
// module, the capabilities switched on and how many starter categories were created.
type VerticalsResponse struct {
	Verticals        []string `json:"verticals"`
	PrimaryModule    *string  `json:"primary_module"`
	Enabled          []string `json:"enabled"`
	Capabilities     []string `json:"capabilities"`
	SeededCategories int      `json:"seeded_categories"`
}

// SetupStepStatus is one checklist step and whether the tenant has done it (TENANCY-008).
type SetupStepStatus struct {
	Code    string `json:"code"`
	Module  string `json:"module"`
	Done    bool   `json:"done"`
	Minutes int    `json:"minutes"`
	Route   string `json:"route"`
}

// TenantSetupResponse is GET /tenants/:slug/setup: the steps in order, how many are done and the minutes left.
type TenantSetupResponse struct {
	Steps       []SetupStepStatus `json:"steps"`
	Done        int               `json:"done"`
	Total       int               `json:"total"`
	MinutesLeft int               `json:"minutes_left"`
}

// VerticalsWrite is the platform half of a preset, run in one transaction.
type VerticalsWrite struct {
	Missing       []string // modules to turn on, requirements first
	Verticals     []string
	PrimaryModule string
	Fallback      string
	Capabilities  []string
	Restrict      bool
}
