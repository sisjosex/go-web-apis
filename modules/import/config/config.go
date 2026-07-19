package config

import (
	"josex/web/modules/core/utils"
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
