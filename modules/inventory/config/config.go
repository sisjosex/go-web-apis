package config

import "josex/web/modules/core/utils"

// InventoryConfig holds configuration for inventory module
type InventoryConfig struct {
	EnableVariants      bool
	EnableAlerts        bool
	ExpiringWarningDays int
	// MaxAxes and MaxCombinations cap what sp_generate_product_skus will build
	// for one product (INV-008 D5). The SP enforces them; these are the values
	// it is called with.
	MaxAxes         int
	MaxCombinations int
}

// LoadInventoryConfig loads inventory configuration from environment
func LoadInventoryConfig() *InventoryConfig {
	return &InventoryConfig{
		EnableVariants:      utils.GetEnvAsBool("INVENTORY_ENABLE_VARIANTS", true),
		EnableAlerts:        utils.GetEnvAsBool("INVENTORY_ENABLE_ALERTS", true),
		ExpiringWarningDays: utils.GetEnvAsInt("BATCHES_EXPIRING_WARNING_DAYS", 7),
		MaxAxes:             utils.GetEnvAsInt("INVENTORY_MAX_AXES", 3),
		MaxCombinations:     utils.GetEnvAsInt("INVENTORY_MAX_COMBINATIONS", 100),
	}
}
