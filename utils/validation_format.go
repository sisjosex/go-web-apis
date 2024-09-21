package utils

import (
	"josex/web/common"

	"github.com/buxizhizhoum/inflection"
	"github.com/go-playground/validator/v10"
)

func FieldToColumn(fieldName string) string {
	return inflection.Underscore(fieldName)
}

func FormatValidationErrors(err error) map[string]string {
	errors := make(map[string]string)

	if validationErrors, ok := err.(validator.ValidationErrors); ok {
		for _, fieldErr := range validationErrors {
			field := fieldErr.Field()
			tag := fieldErr.Tag()
			customTag, exists := common.ErrorTagCatalog[tag]
			if exists {
				tag = customTag
			}
			errors[FieldToColumn(field)] = tag
		}
	} else {
		errors["global"] = err.Error()
	}

	return errors
}

func ExtractValidationError(err error) interface{} {
	if validationErrors, ok := err.(validator.ValidationErrors); ok {
		return FormatValidationErrors(validationErrors)
	} else {
		return err.Error()
	}
}
