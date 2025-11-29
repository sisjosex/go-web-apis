package utils

import (
	"github.com/buxizhizhoum/inflection"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

// ErrorTagCatalog maps validation tags to error codes for translation
var ErrorTagCatalog = map[string]string{
	"required":    "validation.required",
	"email":       "validation.email",
	"email-valid": "validation.email-invalid",
	"min":         "validation.min",
	"max":         "validation.max",
	"gte":         "validation.gte",
	"lte":         "validation.lte",
	"len":         "validation.len",
	"oneof":       "validation.oneof",
	"url":         "validation.url",
	"datetime":    "validation.datetime",
	"uuid":        "validation.uuid",
	"uuidv4":      "validation.uuid",
}

func FieldToColumn(fieldName string) string {
	return inflection.Underscore(fieldName)
}

// FormatValidationErrors formats validation errors with translated messages
func FormatValidationErrors(c *gin.Context, err error) map[string]string {
	validationErrors := make(map[string]string)

	if valErrors, ok := err.(validator.ValidationErrors); ok {
		lang := getLangFromContext(c)

		for _, fieldErr := range valErrors {
			field := fieldErr.Field()
			tag := fieldErr.Tag()

			// Map tag to error code
			errorCode, exists := ErrorTagCatalog[tag]
			if !exists {
				errorCode = "validation." + tag
			}

			// Translate the error message
			translatedMessage := GetTranslation(lang, errorCode)

			validationErrors[FieldToColumn(field)] = translatedMessage
		}
	} else {
		validationErrors["global"] = err.Error()
	}

	return validationErrors
}

// ExtractValidationError extracts and translates validation errors
func ExtractValidationError(c *gin.Context, err error) interface{} {
	if validationErrors, ok := err.(validator.ValidationErrors); ok {
		return FormatValidationErrors(c, validationErrors)
	} else {
		return err.Error()
	}
}

// getLangFromContext extracts language from Gin context, defaults to "en"
func getLangFromContext(c *gin.Context) string {
	if lang, exists := c.Get("lang"); exists {
		if langStr, ok := lang.(string); ok {
			return langStr
		}
	}
	return "en"
}
