package config

import (
	"josex/web/modules/core/utils"
)

// UsersConfig holds configuration for users module (admin/management)
type UsersConfig struct {
	// Pagination
	DefaultPageSize int
	MaxPageSize     int

	// User management
	AllowUserDeletion bool
	EnableSoftDelete  bool

	// Avatar/Profile
	MaxAvatarSizeKB      int
	AllowedAvatarFormats []string
}

// LoadUsersConfig loads users configuration from environment
func LoadUsersConfig() *UsersConfig {
	return &UsersConfig{
		// Pagination
		DefaultPageSize: utils.GetEnvAsInt("USERS_DEFAULT_PAGE_SIZE", 20),
		MaxPageSize:     utils.GetEnvAsInt("USERS_MAX_PAGE_SIZE", 100),

		// User management flags
		AllowUserDeletion: utils.GetEnvAsBool("ALLOW_USER_DELETION", false),
		EnableSoftDelete:  utils.GetEnvAsBool("ENABLE_SOFT_DELETE", true),

		// Avatar/Profile settings
		MaxAvatarSizeKB:      utils.GetEnvAsInt("MAX_AVATAR_SIZE_KB", 2048), // 2MB
		AllowedAvatarFormats: utils.GetEnvAsStringSlice("ALLOWED_AVATAR_FORMATS", []string{"jpg", "jpeg", "png", "gif"}),
	}
}
