package utils

// Translations stores all loaded translations (populated by translator_service)
var Translations = make(map[string]map[string]string)

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
