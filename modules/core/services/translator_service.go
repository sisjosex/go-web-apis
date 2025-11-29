package services

import (
	"encoding/json"
	"fmt"
	"josex/web/modules/core/utils"
	"log"
	"os"
)

// LoadAllTranslations loads translation files for the given languages from all enabled modules
// It merges translations from modules/core/lang, modules/auth/lang, modules/users/lang, etc.
func LoadAllTranslations(languages []string, enabledModules []string) error {
	for _, lang := range languages {
		merged := make(map[string]string)
		translationCount := 0

		// Load translations from each enabled module
		for _, module := range enabledModules {
			filePath := fmt.Sprintf("modules/%s/lang/%s.json", module, lang)

			// Check if file exists
			if _, err := os.Stat(filePath); os.IsNotExist(err) {
				// Module might not have translations, skip silently
				continue
			}

			data, err := os.ReadFile(filePath)
			if err != nil {
				log.Printf("Warning: Could not read translation file %s: %v", filePath, err)
				continue
			}

			var translations map[string]string
			if err := json.Unmarshal(data, &translations); err != nil {
				log.Printf("Warning: Could not parse translation file %s: %v", filePath, err)
				continue
			}

			// Merge into main translations map
			for key, value := range translations {
				merged[key] = value
				translationCount++
			}

			log.Printf("✅ Loaded %d translations from %s", len(translations), filePath)
		}

		utils.Translations[lang] = merged
		log.Printf("✅ Loaded %d translations for language: %s", translationCount, lang)
	}

	return nil
}
