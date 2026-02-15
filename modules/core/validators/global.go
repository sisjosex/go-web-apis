package validators

import (
	"reflect"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
)

func EmailValidation(fl validator.FieldLevel) bool {
	emailStr := fl.Field().String()

	// Si es vacío, se considera como inválido
	if emailStr == "" {
		return true
	}

	// Obtener el valor desreferenciado y validar si es una cadena válida
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	return emailRegex.MatchString(emailStr)
}

func validateUUIDv4(fl validator.FieldLevel) bool {
	field := fl.Field()

	// Handle pointer types (*uuid.UUID)
	if field.Kind().String() == "ptr" {
		if field.IsNil() {
			return true // nil is valid for omitempty
		}
		field = field.Elem()
	}

	// Handle uuid.UUID type directly - convert to string for validation
	if field.Type() == reflect.TypeOf(uuid.UUID{}) {
		u := field.Interface().(uuid.UUID)
		// uuid.UUID zero value is invalid, but we let omitempty handle that
		return u != uuid.UUID{}
	}

	// Fallback: try as string
	id := field.String()
	if id == "" {
		return true
	}

	_, err := uuid.Parse(id)
	return err == nil
}

// validateOtpChannel validates OTP channel is one of allowed values
func validateOtpChannel(fl validator.FieldLevel) bool {
	channel := strings.ToLower(strings.TrimSpace(fl.Field().String()))
	allowedChannels := []string{"whatsapp", "sms", "email"}

	for _, allowed := range allowedChannels {
		if channel == allowed {
			return true
		}
	}
	return false
}

func RegisterValidations() *validator.Validate {
	validate := binding.Validator.Engine().(*validator.Validate)
	validate.RegisterValidation("email-valid", EmailValidation)
	validate.RegisterValidation("uuidv4", validateUUIDv4)
	validate.RegisterValidation("otp-channel", validateOtpChannel)
	return validate
}
