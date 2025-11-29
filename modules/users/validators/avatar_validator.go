package validators

import (
	"fmt"
	"josex/web/config"
	"mime/multipart"
	"path/filepath"
	"strings"
)

// ValidateAvatar validates avatar file size and format based on configuration
func ValidateAvatar(file *multipart.FileHeader) error {
	if file == nil {
		return nil // No file uploaded, skip validation
	}

	usersConfig := config.ModularAppConfig.Users

	// Validate file size (convert KB to bytes)
	maxSizeBytes := int64(usersConfig.MaxAvatarSizeKB) * 1024
	if file.Size > maxSizeBytes {
		return fmt.Errorf("avatar file size exceeds maximum allowed size of %d KB", usersConfig.MaxAvatarSizeKB)
	}

	// Validate file format
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(file.Filename), "."))

	allowed := false
	for _, format := range usersConfig.AllowedAvatarFormats {
		if ext == strings.ToLower(format) {
			allowed = true
			break
		}
	}

	if !allowed {
		return fmt.Errorf("avatar format '%s' not allowed. Allowed formats: %s",
			ext,
			strings.Join(usersConfig.AllowedAvatarFormats, ", "))
	}

	return nil
}

// GetAllowedAvatarFormats returns the list of allowed avatar formats from config
func GetAllowedAvatarFormats() []string {
	return config.ModularAppConfig.Users.AllowedAvatarFormats
}

// GetMaxAvatarSizeKB returns the maximum avatar size in KB from config
func GetMaxAvatarSizeKB() int {
	return config.ModularAppConfig.Users.MaxAvatarSizeKB
}
