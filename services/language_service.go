package services

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
)

// Exportar Translations para que pueda ser accedido desde otros paquetes
var Translations = make(map[string]map[string]string)

// LoadTranslations carga las traducciones en memoria
func LoadTranslations(lang string) error {
	filePath := fmt.Sprintf("lang/%s.json", lang)
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	var data map[string]string
	if err := json.NewDecoder(file).Decode(&data); err != nil {
		return err
	}

	Translations[lang] = data
	fmt.Println("Loaded lang", lang)
	return nil
}

// LoadAllTranslations carga todos los idiomas soportados
func LoadAllTranslations(languages []string) {
	for _, lang := range languages {
		if err := LoadTranslations(lang); err != nil {
			log.Printf("Error cargando idioma %s: %v", lang, err)
		}
	}
}

// Translate devuelve la traducción basada en una clave y un idioma
func Translate(lang, key string) string {
	if _, exists := Translations[lang]; !exists {
		lang = "en"
	}
	if val, exists := Translations[lang][key]; exists {
		return val
	}
	return key
}
