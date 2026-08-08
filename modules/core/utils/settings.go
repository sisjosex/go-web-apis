package utils

import (
	"log"
	"strconv"
	"strings"
	"sync"
)

// Settings are configuration values addressed as <module>.<section>.<key> —
// one naming rule for both sources. A value is resolved in this order:
//
//  1. the boot-time overlay, loaded from core.settings by the settings service;
//  2. the environment variable <MODULE>_<SECTION>_<KEY> (e.g. IMPORT_IMAGES_MAX_ZIP_MB);
//  3. the default passed by the caller.
//
// The overlay is written once at boot and only read afterwards, so call sites
// resolve lazily and stay correct whether or not the database is reachable.
var (
	settingsOverlay   = map[string]string{}
	settingsOverlayMu sync.RWMutex
)

// SetSettingsOverlay installs the values loaded from the database, replacing any
// previous overlay. Keys are normalized to lowercase <module>.<section>.<key>.
func SetSettingsOverlay(values map[string]string) {
	normalized := make(map[string]string, len(values))
	for key, value := range values {
		normalized[strings.ToLower(key)] = value
	}

	settingsOverlayMu.Lock()
	settingsOverlay = normalized
	settingsOverlayMu.Unlock()
}

// SettingKey builds the canonical <module>.<section>.<key> identifier.
func SettingKey(module, section, key string) string {
	return strings.ToLower(module + "." + section + "." + key)
}

// settingEnvKey is the env fallback name for a setting: IMPORT_IMAGES_MAX_ZIP_MB.
func settingEnvKey(module, section, key string) string {
	return strings.ToUpper(module + "_" + section + "_" + key)
}

// lookupSetting returns the overlay value for a setting, if one is present.
func lookupSetting(module, section, key string) (string, bool) {
	settingsOverlayMu.RLock()
	value, ok := settingsOverlay[SettingKey(module, section, key)]
	settingsOverlayMu.RUnlock()

	if !ok || strings.TrimSpace(value) == "" {
		return "", false
	}
	return value, true
}

// GetSetting resolves a string setting: database, then env, then default.
func GetSetting(module, section, key, defaultValue string) string {
	if value, ok := lookupSetting(module, section, key); ok {
		return value
	}
	return GetEnv(settingEnvKey(module, section, key), defaultValue)
}

// GetSettingAsInt resolves an int setting: database, then env, then default.
func GetSettingAsInt(module, section, key string, defaultValue int) int {
	envKey := settingEnvKey(module, section, key)
	if value, ok := lookupSetting(module, section, key); ok {
		return parseIntOr(value, GetEnvAsInt(envKey, defaultValue))
	}
	return GetEnvAsInt(envKey, defaultValue)
}

// GetSettingAsBool resolves a bool setting: database, then env, then default.
func GetSettingAsBool(module, section, key string, defaultValue bool) bool {
	envKey := settingEnvKey(module, section, key)
	if value, ok := lookupSetting(module, section, key); ok {
		return parseBoolOr(value, GetEnvAsBool(envKey, defaultValue))
	}
	return GetEnvAsBool(envKey, defaultValue)
}

// GetSettingAsStringSlice resolves a comma-separated setting: database, then env,
// then default.
func GetSettingAsStringSlice(module, section, key string, defaultValue []string) []string {
	envKey := settingEnvKey(module, section, key)
	if value, ok := lookupSetting(module, section, key); ok {
		if parts := splitAndTrim(value); len(parts) > 0 {
			return parts
		}
	}
	return GetEnvAsStringSlice(envKey, defaultValue)
}

// parseIntOr converts a stored setting to int, falling back when it is malformed
// so one bad row never takes the server down.
func parseIntOr(value string, fallback int) int {
	if parsed, err := strconv.Atoi(strings.TrimSpace(value)); err == nil {
		return parsed
	}
	log.Printf("Warning: setting value '%s' is not an int, using fallback: %d", value, fallback)
	return fallback
}

// parseBoolOr converts a stored setting to bool, accepting the same spellings as
// GetEnvAsBool and falling back when it is malformed.
func parseBoolOr(value string, fallback bool) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "1", "yes", "on":
		return true
	case "false", "0", "no", "off":
		return false
	}
	log.Printf("Warning: setting value '%s' is not a bool, using fallback: %v", value, fallback)
	return fallback
}

// splitAndTrim splits a comma-separated setting into its non-empty parts.
func splitAndTrim(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
