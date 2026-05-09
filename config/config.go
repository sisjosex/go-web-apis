package config

import (
	"sync"

	authConfig "josex/web/modules/auth/config"
	billingConfig "josex/web/modules/billing/config"
	coreConfig "josex/web/modules/core/config"
	"josex/web/modules/core/utils"
	inventoryConfig "josex/web/modules/inventory/config"
	purchasingConfig "josex/web/modules/purchasing/config"
	salesConfig "josex/web/modules/sales/config"
	tenancyConfig "josex/web/modules/tenancy/config"
	trackingConfig "josex/web/modules/tracking/config"
	usersConfig "josex/web/modules/users/config"
)

// ModularConfig holds all modular configuration
type ModularConfig struct {
	Core       *coreConfig.CoreConfig
	Auth       *authConfig.AuthConfig
	Users      *usersConfig.UsersConfig
	Billing    *billingConfig.BillingConfig
	Tenancy    *tenancyConfig.TenancyConfig
	Tracking   *trackingConfig.TrackingConfig
	Inventory  *inventoryConfig.InventoryConfig
	Sales      *salesConfig.SalesConfig
	Purchasing *purchasingConfig.PurchasingConfig
}

// Global modular configuration (lazy-loaded on first access)
var (
	ModularAppConfig *ModularConfig
	configOnce       sync.Once
)

// GetConfig returns the global config, loading it on first call
func GetConfig() *ModularConfig {
	configOnce.Do(func() {
		// Ensure .env is loaded first
		utils.LoadEnv()

		// Load all module configurations
		ModularAppConfig = &ModularConfig{
			Core:       coreConfig.LoadCoreConfig(),
			Auth:       authConfig.LoadAuthConfig(),
			Users:      usersConfig.LoadUsersConfig(),
			Billing:    billingConfig.LoadBillingConfig(),
			Tenancy:    tenancyConfig.LoadTenancyConfig(),
			Tracking:   trackingConfig.LoadTrackingConfig(),
			Inventory:  inventoryConfig.LoadInventoryConfig(),
			Sales:      salesConfig.LoadSalesConfig(),
			Purchasing: purchasingConfig.LoadPurchasingConfig(),
		}
	})
	return ModularAppConfig
}

func init() {
	// Empty - config is now lazy-loaded
}
