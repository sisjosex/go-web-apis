package common

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

type ValidationError struct {
	Code   string      `json:"code,omitempty"`
	Error  interface{} `json:"error,omitempty"`
	Detail interface{} `json:"detail,omitempty"`
}

const (
	UserCreateFailed          = "user.create.failed"
	UserUpdateFailed          = "user.update.failed"
	UserLoginFailed           = "user.login.invalid-credentials2"
	UserLoginValidationFailed = "user.login.validation-failed"
	UserLoginNotFound         = "user.login.not-found"
	UserValidationFailed      = "user.create.validation-failed"
	UserEmailAlreadyInUse     = "user.create.email-in-use"
	UserSearchFailed          = "user.search.failed"
	UserRegisterFailed        = "user.register.failed"
	UserGetByIdNotFound       = "user.get-by-id.not-found"
	UserGetByIdEmailFound     = "user.get-by-email.not-found"
	UserRequestEmailError     = "user.email.request.failed"
	UserEmailVerification     = "user.email.verification.failed"
	UserChangePasswordError   = "user.change-password.failed"
	UserPasswordResetError    = "user.password-reset.failed"
)

type ErrorTag map[string]string

var ErrorTagCatalog = ErrorTag{
	"email-valid": "email-invalid",
	//"email-exists": "user.email.exists",
}

func BuildErrorSingle(Error string) *ValidationError {
	return &ValidationError{Error: Error}
}

func BuildErrorDetail(Error string, Detail interface{}) *ValidationError {
	return &ValidationError{Error: Error, Detail: Detail}
}

func BuildError(err error) *ValidationError {

	if err == nil {
		return nil
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return &ValidationError{
			Error: pgErr.Message,
			Code:  pgErr.Code,
		}
	}

	return &ValidationError{
		Error: err.Error(),
	}
}
