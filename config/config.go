package config

import (
	authConfig "josex/web/modules/auth/config"
	coreConfig "josex/web/modules/core/config"
	"josex/web/modules/core/utils"
	tenancyConfig "josex/web/modules/tenancy/config"
	usersConfig "josex/web/modules/users/config"
)

// ModularConfig holds all modular configuration
type ModularConfig struct {
	Core    *coreConfig.CoreConfig
	Auth    *authConfig.AuthConfig
	Users   *usersConfig.UsersConfig
	Tenancy *tenancyConfig.TenancyConfig
}

// Global modular configuration (loaded on init)
var ModularAppConfig *ModularConfig

func init() {
	// Ensure .env is loaded first
	utils.LoadEnv()

	// Load all module configurations
	ModularAppConfig = &ModularConfig{
		Core:    coreConfig.LoadCoreConfig(),
		Auth:    authConfig.LoadAuthConfig(),
		Users:   usersConfig.LoadUsersConfig(),
		Tenancy: tenancyConfig.LoadTenancyConfig(),
	}
}
