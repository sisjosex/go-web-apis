package validators

import (
	"regexp"

	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
)

func EmailValidation(fl validator.FieldLevel) bool {
	emailStr := fl.Field().String()

	// Si es nil, se considera como válido
	if emailStr == "" {
		return false
	}

	// Obtener el valor desreferenciado y validar si es una cadena válida
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	return emailRegex.MatchString(emailStr)
}

func validateUUIDv4(fl validator.FieldLevel) bool {
	id := fl.Field().String()
	_, err := uuid.Parse(id)
	return err != nil
}

func RegisterValidations() *validator.Validate {
	validate := binding.Validator.Engine().(*validator.Validate)
	validate.RegisterValidation("email-valid", EmailValidation)
	validate.RegisterValidation("uuidv4", validateUUIDv4)
	return validate
}
