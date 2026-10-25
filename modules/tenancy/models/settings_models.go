package models

// The workspace's general settings (TENANCY-007), declared in tenancy's register.go and read by the
// modules that default something from them.
const (
	SettingTimezone = "general.timezone"
	SettingLocale   = "general.locale"
	SettingCountry  = "general.country"
)

// TenantSettingsResponse is a workspace's settings: every key its modules declare, the default when
// it never set one, and what each key may hold.
type TenantSettingsResponse struct {
	Settings map[string]any      `json:"settings"`
	Specs    []TenantSettingSpec `json:"specs"`
}

// TenantSettingSpec is one key's declaration as the web reads it.
type TenantSettingSpec struct {
	Key     string   `json:"key"`
	Module  string   `json:"module"`
	Type    string   `json:"type"`
	Options []string `json:"options,omitempty"`
}
