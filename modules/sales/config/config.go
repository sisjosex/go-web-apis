package config

import (
	"josex/web/modules/core/utils"
)

// SalesConfig holds sales module configuration
type SalesConfig struct {
	OrderInactivityTimeoutMinutes int
	MaxOrderItems                 int
	EnableAutoInvoicing           bool
	TaxRate                       float64
}

// LoadSalesConfig loads sales module configuration from environment
func LoadSalesConfig() *SalesConfig {
	return &SalesConfig{
		OrderInactivityTimeoutMinutes: utils.GetEnvAsInt("SALES_ORDER_INACTIVITY_TIMEOUT_MINUTES", 60),
		MaxOrderItems:                 utils.GetEnvAsInt("SALES_MAX_ORDER_ITEMS", 100),
		EnableAutoInvoicing:           utils.GetEnvAsBool("SALES_ENABLE_AUTO_INVOICING", true),
		TaxRate:                       0.16, // 16% default tax rate
	}
}
