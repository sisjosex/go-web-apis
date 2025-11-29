package services

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
)

// Translations stores all loaded translations
var Translations = make(map[string]map[string]string)

// LoadAllTranslations loads translation files for the given languages
func LoadAllTranslations(languages []string) {
	for _, lang := range languages {
		filePath := fmt.Sprintf("./lang/%s.json", lang)
		data, err := os.ReadFile(filePath)
		if err != nil {
			log.Fatalf("Error loading translation file %s: %v", filePath, err)
		}

		var translations map[string]string
		if err := json.Unmarshal(data, &translations); err != nil {
			log.Fatalf("Error parsing translation file %s: %v", filePath, err)
		}

		Translations[lang] = translations
		log.Printf("Loaded lang %s", lang)
	}
}

// GetTranslation returns the translation for a key in the given language
func GetTranslation(lang, key string) string {
	if translations, exists := Translations[lang]; exists {
		if translation, found := translations[key]; found {
			return translation
		}
	}
	// Fallback to English
	if translations, exists := Translations["en"]; exists {
		if translation, found := translations[key]; found {
			return translation
		}
	}
	return key
}
