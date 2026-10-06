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
	// Where a business pays its plan (BILLING-001 D3): the bank QR image and the account it is for.
	PaymentQRImageURL string
	PaymentBank       string
	PaymentAccount    string
	PaymentHolder     string
}

// LoadBillingConfig loads billing configuration from environment variables.
func LoadBillingConfig() *BillingConfig {
	return &BillingConfig{
		DefaultPageSize: utils.GetEnvAsInt("BILLING_DEFAULT_PAGE_SIZE", 10),
		MaxPageSize:     utils.GetEnvAsInt("BILLING_MAX_PAGE_SIZE", 100),

		PaymentQRImageURL: utils.GetEnv("BILLING_PAYMENT_QR_URL", ""),
		PaymentBank:       utils.GetEnv("BILLING_PAYMENT_BANK", ""),
		PaymentAccount:    utils.GetEnv("BILLING_PAYMENT_ACCOUNT", ""),
		PaymentHolder:     utils.GetEnv("BILLING_PAYMENT_HOLDER", ""),
	}
}
