package config

import (
	"josex/web/modules/core/utils"
)

// Settings addressed as import.images.* — see modules/core/utils/settings.go.
// They resolve per call (database → IMPORT_IMAGES_* env → default) because the
// overlay is loaded once the database is up, after this config is built.
const (
	settingsModule = "import"
	sectionImages  = "images"
)

// ImportConfig holds limits for the generic CSV import framework.
type ImportConfig struct {
	MaxFileSizeMB int
	MaxRows       int
}

// LoadImportConfig loads import configuration from environment.
func LoadImportConfig() *ImportConfig {
	return &ImportConfig{
		MaxFileSizeMB: utils.GetEnvAsInt("IMPORT_MAX_FILE_SIZE_MB", 5),
		MaxRows:       utils.GetEnvAsInt("IMPORT_MAX_ROWS", 500),
	}
}

// MaxImagesZipMB caps the companion image archive, checked both on upload and
// against the inflated total.
func MaxImagesZipMB() int {
	return utils.GetSettingAsInt(settingsModule, sectionImages, "max_zip_mb", 20)
}

// MaxImageSizeKB caps a single extracted image. Defaults to the same 2 MB the
// users module allows for an avatar.
func MaxImageSizeKB() int {
	return utils.GetSettingAsInt(settingsModule, sectionImages, "max_image_kb", 2048)
}

// AllowedImageFormats are the extensions an archive entry may carry.
func AllowedImageFormats() []string {
	return utils.GetSettingAsStringSlice(settingsModule, sectionImages, "allowed_formats",
		[]string{"jpg", "jpeg", "png", "gif"})
}
