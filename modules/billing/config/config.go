package config

import (
	"josex/web/modules/core/utils"
)

// BillingConfig holds configuration for the billing module.
type BillingConfig struct {
	// DefaultPageSize is the number of payment records returned per page.
	DefaultPageSize int
	// MaxPageSize caps the limit parameter on list endpoints.
	MaxPageSize int
}

// LoadBillingConfig loads billing configuration from environment variables.
func LoadBillingConfig() *BillingConfig {
	return &BillingConfig{
		DefaultPageSize: utils.GetEnvAsInt("BILLING_DEFAULT_PAGE_SIZE", 10),
		MaxPageSize:     utils.GetEnvAsInt("BILLING_MAX_PAGE_SIZE", 100),
	}
}
