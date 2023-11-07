package config

import (
	"fmt"
)

type ValidationError struct {
	Error  string      `json:"error"`
	Detail interface{} `json:"detail,omitempty"`
}

const (
	UserCreateFailed      = "user.create.failed"
	UserValidationFailed  = "user.create.validation-failed"
	UserEmailAlreadyInUse = "user.create.email-in-use"
	UserSearchFailed      = "user.search.failed"
	UserRegisterFailed    = "user.register.failed"
	UserGetByIdNotFound   = "user.get-by-id.not-found"
	UserGetByIdEmailFound = "user.get-by-email.not-found"
)

func BuildErrorSingle(Error string) *ValidationError {
	return &ValidationError{Error: Error}
}

func BuildErrorDetail(Error string, Detail interface{}) *ValidationError {
	fmt.Println("Detail", Detail)
	return &ValidationError{Error: Error, Detail: Detail}
}
