package config

import (
	"josex/web/modules/core/utils"
)

// CoreConfig holds configuration for core module
type CoreConfig struct {
	// Database
	DatabaseURL      string
	DatabasePoolSize int32

	// Migrations
	EnabledModules []string // Which modules to run migrations for

	// Server
	AppMode string
	AppHost string
	AppPort string

	// Frontend
	FrontendURL string // Used for email links and CORS

	// Logging
	LogLevel string

	// CORS
	AllowedOrigins []string
}

// DefaultCoreConfig returns default configuration for core module
func DefaultCoreConfig() *CoreConfig {
	return &CoreConfig{
		DatabaseURL:      "postgres://postgres:postgres@localhost:5432/web?sslmode=disable",
		DatabasePoolSize: 10,
		EnabledModules:   []string{"core", "auth", "users"}, // All modules enabled by default
		AppMode:          "debug",
		AppHost:          "127.0.0.1",
		AppPort:          "8080",
		LogLevel:         "info",
		AllowedOrigins:   []string{"http://localhost:3000"},
	}
}

// LoadCoreConfig loads core configuration from environment
func LoadCoreConfig() *CoreConfig {
	return &CoreConfig{
		// Database
		DatabaseURL:      utils.GetEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/web?sslmode=disable"),
		DatabasePoolSize: utils.GetEnvAsInt32("DATABASE_POOL_SIZE", 10),

		// Migrations - comma-separated list: "core,auth,users"
		EnabledModules: utils.GetEnvAsStringSlice("ENABLED_MODULES", []string{"core", "auth", "users"}),

		// Server
		AppMode: utils.GetEnv("APP_MODE", "debug"),
		AppHost: utils.GetEnv("APP_HOST", "127.0.0.1"),
		AppPort: utils.GetEnv("APP_PORT", "8080"),

		// Frontend URL (for email links and redirects)
		FrontendURL: utils.GetEnv("FRONTEND_URL", "http://localhost:3000"),

		// Logging
		LogLevel: utils.GetEnv("LOG_LEVEL", "info"),

		// CORS - comma-separated list
		AllowedOrigins: utils.GetEnvAsStringSlice("ALLOWED_ORIGINS", []string{"http://localhost:3000"}),
	}
}

// IsModuleEnabled checks if a module is enabled
func (c *CoreConfig) IsModuleEnabled(moduleName string) bool {
	for _, m := range c.EnabledModules {
		if m == moduleName {
			return true
		}
	}
	return false
}
