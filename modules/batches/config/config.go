package config

import "josex/web/modules/core/utils"

type BatchesConfig struct {
	ExpiringWarningDays int
}

func LoadBatchesConfig() *BatchesConfig {
	return &BatchesConfig{
		ExpiringWarningDays: utils.GetEnvAsInt("BATCHES_EXPIRING_WARNING_DAYS", 7),
	}
}
