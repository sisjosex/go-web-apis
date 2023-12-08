package validators

import (
	"josex/web/services"
	"regexp"

	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
)

func EmailValidation(fl validator.FieldLevel) bool {
	email := fl.Field().String()
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	return emailRegex.MatchString(email)
}

func EmailExistValidator(fl validator.FieldLevel) bool {
	var userService = services.NewUserService(services.DB)
	email := fl.Field().String()

	user, _ := userService.GetUserByEmail(email, nil)
	return user == nil
}

func RegisterValidations() *validator.Validate {
	validate := binding.Validator.Engine().(*validator.Validate)
	validate.RegisterValidation("email-valid", EmailValidation)
	validate.RegisterValidation("email-exists", EmailExistValidator)
	return validate
}
