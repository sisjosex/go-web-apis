package errors

import (
	"errors"
	"josex/web/modules/core/utils"

	"github.com/gin-gonic/gin"
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

// Authorization errors
const (
	UserUnauthorized            = "user.unauthorized"
	UserInsufficientPermissions = "user.insufficient-permissions"
)

// Request errors
const (
	RequestRateLimited = "request.rate-limited"
)

// BuildErrorSingle creates an error response with just an error code and translates it
func BuildErrorSingle(c *gin.Context, errorCode string) *ErrorResponse {
	lang := getLangFromContext(c)
	return &ErrorResponse{
		Error:   errorCode,
		Message: utils.GetTranslation(lang, errorCode),
	}
}

// BuildErrorDetail creates an error response with error code, translation, and detail
func BuildErrorDetail(c *gin.Context, errorCode string, detail interface{}) *ErrorResponse {
	lang := getLangFromContext(c)
	return &ErrorResponse{
		Error:   errorCode,
		Message: utils.GetTranslation(lang, errorCode),
		Detail:  detail,
	}
}

// BuildError creates an error response from a Go error
// Handles PostgreSQL errors with special formatting
func BuildError(c *gin.Context, err error) *ErrorResponse {
	if err == nil {
		return nil
	}

	lang := getLangFromContext(c)

	// Try to unwrap and find PostgreSQL error
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return &ErrorResponse{
			Error:   pgErr.Message,
			Message: utils.GetTranslation(lang, pgErr.Message),
			Code:    pgErr.Code,
			Detail:  pgErr.Detail,
		}
	}

	return &ErrorResponse{
		Error: err.Error(),
	}
}

// getLangFromContext extracts language from Gin context, defaults to "en"
func getLangFromContext(c *gin.Context) string {
	if lang, exists := c.Get("lang"); exists {
		if langStr, ok := lang.(string); ok {
			return langStr
		}
	}
	return "en"
}
