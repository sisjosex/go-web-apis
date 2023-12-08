package utils

import (
	"josex/web/config"

	"github.com/buxizhizhoum/inflection"
	"github.com/go-playground/validator/v10"
)

var errors = make(map[string]string)

func FieldToColumn(fieldName string) string {
	return inflection.Underscore(fieldName)
}

func FormatValidationErrors(err error) map[string]string {
	validationErrors, ok := err.(validator.ValidationErrors)
	if !ok {
		return make(map[string]string)
	}

	for _, fieldErr := range validationErrors {
		field := fieldErr.Field()
		tag := fieldErr.Tag()
		customTag, exists := config.ErrorTagCatalog[tag]
		if exists {
			tag = customTag
		}
		errors[FieldToColumn(field)] = tag
	}

	return errors
}
