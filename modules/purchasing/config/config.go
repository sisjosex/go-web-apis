package config

import (
	"josex/web/modules/core/utils"
)

// PurchasingConfig holds purchasing module configuration
type PurchasingConfig struct {
	EnableAutoReceive bool
	DefaultPageSize   int
}

// LoadPurchasingConfig loads purchasing configuration from environment
func LoadPurchasingConfig() *PurchasingConfig {
	return &PurchasingConfig{
		EnableAutoReceive: utils.GetEnvAsBool("PURCHASING_ENABLE_AUTO_RECEIVE", false),
		DefaultPageSize:   utils.GetEnvAsInt("PURCHASING_DEFAULT_PAGE_SIZE", 20),
	}
}
