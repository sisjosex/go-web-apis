package validators

import (
	"regexp"

	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
)

var validate *validator.Validate

func GetValidator() *validator.Validate {
	return validate
}

func InitValidator() *validator.Validate {
	if validate == nil {
		validate = validator.New()
		RegisterCustomValidations()
	}
	return validate
}

func RegisterCustomValidations() {

	if validate, ok := binding.Validator.Engine().(*validator.Validate); ok {
		validate.RegisterValidation("emailregex", EmailValidation)
	}
}

func EmailValidation(fl validator.FieldLevel) bool {
	email := fl.Field().String()
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	return emailRegex.MatchString(email)
}
