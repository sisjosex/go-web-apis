package common

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

type ErrorResponse struct {
	// Error code
	// example: 400
	Code string `json:"code,omitempty"`
	// Error message
	// example: Bad Request
	Error interface{} `json:"error,omitempty"`
	// Translated error message
	Message interface{} `json:"message,omitempty"`
	// Error details
	// example: {"email": "Invalid email format"}
	Detail interface{} `json:"detail,omitempty"`
}

const (
	UserCreateFailed                    = "user.create.failed"
	UserUpdateFailed                    = "user.update.failed"
	UserLoginFailed                     = "user.login.invalid-credentials2"
	UserLoginValidationFailed           = "user.login.validation-failed"
	UserLoginNotFound                   = "user.login.not-found"
	UserLogoutValidationFailed          = "user.logout.validation-failed"
	UserValidationFailed                = "user.create.validation-failed"
	UserEmailAlreadyInUse               = "user.create.email-in-use"
	UserSearchFailed                    = "user.search.failed"
	UserRegisterFailed                  = "user.register.failed"
	UserGetByIdNotFound                 = "user.get-by-id.not-found"
	UserGetByIdEmailFound               = "user.get-by-email.not-found"
	UserRequestEmailError               = "user.email.request.failed"
	UserEmailVerification               = "user.email.verification.failed"
	UserChangePasswordError             = "user.change-password.failed"
	UserPasswordResetError              = "user.password-reset.failed"
	UserChangeEmailSendingError         = "user.change-email.sending-email-failed"
	UserForgorPasswordEmailSendingError = "user.forgot-password.sending-email-failed"
)

type ErrorTag map[string]string

var ErrorTagCatalog = ErrorTag{
	"email-valid": "email-invalid",
	//"email-exists": "user.email.exists",
}

func BuildErrorSingle(Error string) *ErrorResponse {
	return &ErrorResponse{Error: Error}
}

func BuildErrorDetail(Error string, Detail interface{}) *ErrorResponse {
	return &ErrorResponse{Error: Error, Detail: Detail}
}

func BuildError(err error) *ErrorResponse {

	if err == nil {
		return nil
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return &ErrorResponse{
			Error: pgErr.Message,
			Code:  pgErr.Code,
		}
	}

	return &ErrorResponse{
		Error: err.Error(),
	}
}
