package errors

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

// ErrorResponse represents a standardized error response
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

// Common validation errors (used across modules)
const (
	InvalidUUID = "validation.invalid-uuid"
)

type ErrorTag map[string]string

var ErrorTagCatalog = ErrorTag{
	"email-valid": "email-invalid",
}

// BuildErrorSingle creates an error response with just an error code
func BuildErrorSingle(Error string) *ErrorResponse {
	return &ErrorResponse{Error: Error}
}

// BuildErrorDetail creates an error response with error code and detail
func BuildErrorDetail(Error string, Detail interface{}) *ErrorResponse {
	return &ErrorResponse{Error: Error, Detail: Detail}
}

// BuildError creates an error response from a Go error
// Handles PostgreSQL errors with special formatting
func BuildError(err error) *ErrorResponse {
	if err == nil {
		return nil
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return &ErrorResponse{
			Error:  pgErr.Message,
			Code:   pgErr.Code,
			Detail: pgErr.Detail,
		}
	}

	return &ErrorResponse{
		Error: err.Error(),
	}
}
