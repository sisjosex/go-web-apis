package utils

import (
	"github.com/buxizhizhoum/inflection"
	"github.com/go-playground/validator/v10"
)

func FieldToColumn(fieldName string) string {
	return inflection.Underscore(fieldName)
}

func FormatValidationErrors(err error) map[string]string {
	validationErrors, ok := err.(validator.ValidationErrors)
	if !ok {
		return make(map[string]string)
	}

	errors := make(map[string]string)

	for _, fieldErr := range validationErrors {
		errors[FieldToColumn(fieldErr.Field())] = fieldErr.Tag()
	}

	return errors
}
