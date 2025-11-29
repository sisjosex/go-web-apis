package utils

import (
	"josex/web/modules/core/errors"

	"github.com/buxizhizhoum/inflection"
	"github.com/go-playground/validator/v10"
)

func FieldToColumn(fieldName string) string {
	return inflection.Underscore(fieldName)
}

func FormatValidationErrors(err error) map[string]string {
	validationErrors := make(map[string]string)

	if valErrors, ok := err.(validator.ValidationErrors); ok {
		for _, fieldErr := range valErrors {
			field := fieldErr.Field()
			tag := fieldErr.Tag()
			customTag, exists := errors.ErrorTagCatalog[tag]
			if exists {
				tag = customTag
			}
			validationErrors[FieldToColumn(field)] = tag
		}
	} else {
		validationErrors["global"] = err.Error()
	}

	return validationErrors
}

func ExtractValidationError(err error) interface{} {
	if validationErrors, ok := err.(validator.ValidationErrors); ok {
		return FormatValidationErrors(validationErrors)
	} else {
		return err.Error()
	}
}
