package config

import (
	"time"

	"josex/web/modules/core/utils"
)

// CoreConfig holds configuration for core module
type CoreConfig struct {
	// Database
	DatabaseURL      string
	DatabasePoolSize int32

	// Migrations
	EnabledModules        []string // Which modules to run migrations for
	ExcludedFromMigration []string // Modules to load but not migrate in Main DB (e.g., tracking for tenant-only mode)

	// Server
	AppMode string
	AppHost string
	AppPort string

	// Frontend
	AppName     string // Display name used in emails and UI
	FrontendURL string // Used for email links and CORS
	// MobileLandingURL is where an app-access notice sends people: the store links and news (TRACK-032 D2).
	MobileLandingURL string

	// Logging — LOG_LEVEL debug|info|warn|error, LOG_FORMAT text|json (INFRA-004)
	LogLevel  string
	LogFormat string

	// CORS
	AllowedOrigins []string

	// Rate limiting — requests per second per IP
	RateLimitPerSecond int

	// Runtime roles and Valkey (INFRA-001)
	AppRole            string        // all | api | realtime | worker | scheduler; the -role flag wins
	RedisURL           string        // empty = no Valkey: in-memory rate limit, no jobs
	JobsConcurrency    int           // asynq worker goroutines
	OutboxPollInterval time.Duration // relay fallback when a NOTIFY is missed

	// Stored files (INFRA-007): R2 in production, the dev compose's SeaweedFS otherwise (D3).
	StorageEndpoint        string // a URL: https://<account>.r2.cloudflarestorage.com
	StorageAccessKey       string
	StorageSecretKey       string
	StorageMediaBucket     string // public images, served from MediaPublicBaseURL (D4)
	StorageDocumentsBucket string // private files, reached only through signed links (D2)
}

// DefaultCoreConfig returns default configuration for core module
func DefaultCoreConfig() *CoreConfig {
	return &CoreConfig{
		DatabaseURL:      "postgres://postgres:postgres@localhost:5432/web?sslmode=disable",
		DatabasePoolSize: 10,
		EnabledModules:   []string{"core", "auth", "users", "tracking", "inventory"}, // All modules enabled by default
		AppMode:          "debug",
		AppHost:          "127.0.0.1",
		AppPort:          "8080",
		LogLevel:         "info",
		LogFormat:        "text",
		AllowedOrigins:   []string{"http://localhost:3000"},

		RateLimitPerSecond: 10,

		AppRole:            "all",
		JobsConcurrency:    10,
		OutboxPollInterval: 5 * time.Second,
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
		// Modules to load but exclude from migrations (e.g., "tracking" for tenant-only mode)
		ExcludedFromMigration: utils.GetEnvAsStringSlice("EXCLUDED_FROM_MIGRATION", []string{}),

		// Server
		AppMode: utils.GetEnv("APP_MODE", "debug"),
		AppHost: utils.GetEnv("APP_HOST", "127.0.0.1"),
		AppPort: utils.GetEnv("APP_PORT", "8080"),

		// App identity
		AppName: utils.GetEnv("APP_NAME", "Platform"),
		// Frontend URL (for email links and redirects)
		FrontendURL: utils.GetEnv("FRONTEND_URL", "http://localhost:3000"),
		// The page the app-access notice links to (TRACK-032 D2)
		MobileLandingURL: utils.GetEnv("MOBILE_LANDING_URL", "https://taypi24.com"),

		// Logging
		LogLevel:  utils.GetEnv("LOG_LEVEL", "info"),
		LogFormat: utils.GetEnv("LOG_FORMAT", "text"),

		// CORS - comma-separated list
		AllowedOrigins: utils.GetEnvAsStringSlice("ALLOWED_ORIGINS", []string{"http://localhost:3000"}),

		// Rate limiting - requests per second per IP
		RateLimitPerSecond: utils.GetEnvAsInt("RATE_LIMIT_PER_SECOND", 10),

		// Runtime roles and Valkey
		AppRole:            utils.GetEnv("APP_ROLE", "all"),
		RedisURL:           utils.GetEnv("REDIS_URL", ""),
		JobsConcurrency:    utils.GetEnvAsInt("JOBS_CONCURRENCY", 10),
		OutboxPollInterval: utils.GetEnvAsDuration("OUTBOX_POLL_INTERVAL", 5*time.Second),

		// Stored files — the defaults are the dev compose's local S3.
		StorageEndpoint:        utils.GetEnv("STORAGE_S3_ENDPOINT", "http://127.0.0.1:8333"),
		StorageAccessKey:       utils.GetEnv("STORAGE_S3_ACCESS_KEY", "dev-access-key"),
		StorageSecretKey:       utils.GetEnv("STORAGE_S3_SECRET_KEY", "dev-secret-key"),
		StorageMediaBucket:     utils.GetEnv("STORAGE_MEDIA_BUCKET", "taypi24-media"),
		StorageDocumentsBucket: utils.GetEnv("STORAGE_DOCUMENTS_BUCKET", "taypi24-documents"),
	}
}

// Settings addressed as core.media.* — see modules/core/utils/settings.go. They
// resolve per call (database → CORE_MEDIA_* env → default) because the overlay
// is loaded once the database is up, after this config is built.
const (
	settingsModule = "core"
	sectionMedia   = "media"
)

// MediaPublicBaseURL is the origin a stored image's key is appended to, making the URL persisted on
// the record (e.g. profile_picture_url): media.taypi24.com in production (D4), the dev bucket's
// path on the local S3 otherwise.
func MediaPublicBaseURL() string {
	return utils.GetSetting(settingsModule, sectionMedia, "public_base_url", "http://127.0.0.1:8333/taypi24-media")
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

// GetEnabledModules returns the list of enabled modules
func (c *CoreConfig) GetEnabledModules() []string {
	if len(c.EnabledModules) == 0 {
		return []string{"core"}
	}
	return c.EnabledModules
}

// IsModuleExcludedFromMigration checks if a module should not be migrated in this database
func (c *CoreConfig) IsModuleExcludedFromMigration(moduleName string) bool {
	for _, m := range c.ExcludedFromMigration {
		if m == moduleName {
			return true
		}
	}
	return false
}
