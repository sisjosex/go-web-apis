package services

import (
	"context"
	"log"

	coreModels "josex/web/modules/core/models"
	coreUtils "josex/web/modules/core/utils"
)

// LoadSettings reads every row of core.settings into the process-wide overlay so
// utils.GetSetting* resolves database values first and env vars as fallback.
//
// It is called once, after migrations have run. A failure is not fatal: the
// overlay simply stays empty and every setting falls back to its env var, which
// is the behaviour the API had before core.settings existed.
func LoadSettings(ctx context.Context, dbService DatabaseService) {
	settings, err := fetchSettings(ctx, dbService)
	if err != nil {
		log.Printf("⚠️  Could not load core.settings (%v) — falling back to environment", err)
		return
	}

	values := make(map[string]string, len(settings))
	for _, setting := range settings {
		values[coreUtils.SettingKey(setting.Module, setting.Section, setting.Key)] = setting.Value
	}
	coreUtils.SetSettingsOverlay(values)

	log.Printf("✅ Loaded %d configuration value(s) from core.settings", len(values))
}

// fetchSettings reads the settings rows through core.sp_get_settings.
func fetchSettings(ctx context.Context, dbService DatabaseService) ([]coreModels.Setting, error) {
	rows, err := dbService.Query(ctx, `SELECT * FROM core.sp_get_settings()`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var settings []coreModels.Setting
	for rows.Next() {
		var setting coreModels.Setting
		if err := rows.Scan(
			&setting.Module,
			&setting.Section,
			&setting.Key,
			&setting.Value,
		); err != nil {
			return nil, err
		}
		settings = append(settings, setting)
	}

	return settings, rows.Err()
}
