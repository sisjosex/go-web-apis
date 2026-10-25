package tenancy

import (
	"josex/web/modules/core/registry"
	"josex/web/modules/tenancy/models"
)

// Time zones and countries the workspace may pick (TENANCY-007): the region the product serves.
var (
	timezones = []string{
		"America/La_Paz", "America/Lima", "America/Santiago", "America/Argentina/Buenos_Aires",
		"America/Asuncion", "America/Sao_Paulo", "America/Bogota", "America/Guayaquil",
		"America/Mexico_City", "America/New_York", "Europe/Madrid", "UTC",
	}
	countries = []string{"BO", "PE", "CL", "AR", "PY", "BR", "CO", "EC", "MX", "US", "ES"}
)

func init() {
	registry.RegisterSetting(registry.SettingSpec{Key: models.SettingTimezone, Module: "tenancy", Type: registry.SettingEnum, Default: "America/La_Paz", Options: timezones})
	registry.RegisterSetting(registry.SettingSpec{Key: models.SettingLocale, Module: "tenancy", Type: registry.SettingEnum, Default: "es", Options: []string{"es", "en"}})
	registry.RegisterSetting(registry.SettingSpec{Key: models.SettingCountry, Module: "tenancy", Type: registry.SettingEnum, Default: "BO", Options: countries})
}
