package config

import (
	"josex/web/modules/core/utils"
)

// TenancyConfig holds configuration for tenancy module
type TenancyConfig struct {
	// Enable/disable multitenancy
	Enabled bool

	// Database pool configuration for tenant databases
	TenantDatabasePoolSize int32

	// Allow tenants to use their own database URLs
	AllowCustomDatabaseURLs bool

	// Default schema name for schema-based tenancy
	DefaultSchemaName string

	// Self-service tenant creation
	AllowSelfService bool

	// Subscription plan limits
	FreePlanLimit       int
	ProPlanLimit        int
	EnterprisePlanLimit int // 0 = unlimited
}

// LoadTenancyConfig loads tenancy module configuration from environment
func LoadTenancyConfig() *TenancyConfig {
	return &TenancyConfig{
		Enabled:                 utils.GetEnvAsBool("TENANCY_ENABLED", false),
		TenantDatabasePoolSize:  utils.GetEnvAsInt32("TENANCY_DATABASE_POOL_SIZE", 5),
		AllowCustomDatabaseURLs: utils.GetEnvAsBool("TENANCY_ALLOW_CUSTOM_DATABASE_URLS", true),
		DefaultSchemaName:       utils.GetEnv("TENANCY_DEFAULT_SCHEMA", "public"),
		AllowSelfService:        utils.GetEnvAsBool("TENANCY_ALLOW_SELF_SERVICE", true),
		FreePlanLimit:           utils.GetEnvAsInt("TENANCY_FREE_PLAN_LIMIT", 1),
		ProPlanLimit:            utils.GetEnvAsInt("TENANCY_PRO_PLAN_LIMIT", 5),
		EnterprisePlanLimit:     utils.GetEnvAsInt("TENANCY_ENTERPRISE_PLAN_LIMIT", 0), // 0 = unlimited
	}
}
