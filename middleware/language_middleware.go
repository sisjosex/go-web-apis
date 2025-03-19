package middleware

import (
	"josex/web/services"

	"github.com/gin-gonic/gin"
)

// Middleware para detectar y almacenar el idioma en el contexto
func LanguageMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Obtener el idioma de "Accept-Language" o del query param `?lang=es`
		lang := c.Query("lang")
		if lang == "" {
			lang = c.GetHeader("Accept-Language")
		}

		// Limpiar el idioma (ejemplo: "es-ES" → "es")
		if len(lang) > 2 {
			lang = lang[:2]
		}

		// Si el idioma no está soportado, usa inglés por defecto
		if _, exists := services.Translations[lang]; !exists {
			lang = "en"
		}

		// Guardar idioma en el contexto de Gin
		c.Set("lang", lang)
		c.Next()
	}
}
