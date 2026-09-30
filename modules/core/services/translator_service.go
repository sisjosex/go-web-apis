package services

import (
	"encoding/json"
	"fmt"
	"josex/web/modules/core/utils"
	"log"
	"log/slog"
	"os"
)

// LoadAllTranslations loads and merges translation files for the given languages
// from every module that ships a lang/ directory. Translations are loaded for all
// modules regardless of whether the module is enabled: cross-module data such as
// the permission catalog is exposed globally and must stay localizable even when
// the contributing module is not enabled on this server.
func LoadAllTranslations(languages []string) error {
	moduleEntries, err := os.ReadDir("modules")
	if err != nil {
		return fmt.Errorf("could not read modules directory: %w", err)
	}

	for _, lang := range languages {
		merged := make(map[string]string)

		for _, entry := range moduleEntries {
			if !entry.IsDir() {
				continue
			}

			filePath := fmt.Sprintf("modules/%s/lang/%s.json", entry.Name(), lang)
			if _, statErr := os.Stat(filePath); os.IsNotExist(statErr) {
				continue
			}

			data, readErr := os.ReadFile(filePath)
			if readErr != nil {
				log.Printf("Warning: Could not read translation file %s: %v", filePath, readErr)
				continue
			}

			var translations map[string]string
			if jsonErr := json.Unmarshal(data, &translations); jsonErr != nil {
				log.Printf("Warning: Could not parse translation file %s: %v", filePath, jsonErr)
				continue
			}

			for key, value := range translations {
				merged[key] = value
			}

			slog.Debug("translations file", "file", filePath, "count", len(translations))
		}

		utils.Translations[lang] = merged
		log.Printf("✅ Loaded %d translations for language: %s", len(merged), lang)
	}

	return nil
}
