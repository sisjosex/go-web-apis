package config

import "josex/web/modules/core/utils"

// InventoryConfig holds configuration for inventory module
type InventoryConfig struct {
	EnableVariants bool
	EnableAlerts   bool
}

// LoadInventoryConfig loads inventory configuration from environment
func LoadInventoryConfig() *InventoryConfig {
	return &InventoryConfig{
		EnableVariants: utils.GetEnvAsBool("INVENTORY_ENABLE_VARIANTS", true),
		EnableAlerts:   utils.GetEnvAsBool("INVENTORY_ENABLE_ALERTS", true),
	}
}
