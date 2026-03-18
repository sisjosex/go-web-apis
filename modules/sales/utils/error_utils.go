package utils

import (
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// GetHTTPStatusFromError maps database/business errors to HTTP status codes
func GetHTTPStatusFromError(err error) int {
	if err == nil {
		return 500
	}

	// Handle pgx no rows
	if errors.Is(err, pgx.ErrNoRows) {
		return 404
	}

	// Handle PostgreSQL errors
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		// Custom error codes from our SPs
		if pgErr.Message != "" {
			switch pgErr.Message {
			case "customer.not-found", "sales-order.not-found", "order-item.not-found":
				return 404
			case "customer.already-exists", "sales-order.invalid-status", "sales-order.insufficient-inventory":
				return 409
			case "customer.invalid-email", "sales-order.empty-items", "order-item.invalid-qty":
				return 400
			}
		}
	}

	// Check error message for patterns
	errMsg := err.Error()
	if errMsg != "" {
		switch {
		case errMsg == "not found" || errMsg == "sql: no rows in result set":
			return 404
		case strContains(errMsg, "not-found"):
			return 404
		case strContains(errMsg, "already-exists") || strContains(errMsg, "duplicate"):
			return 409
		case strContains(errMsg, "insufficient-") || strContains(errMsg, "Conflict"):
			return 409
		case strContains(errMsg, "invalid-"):
			return 400
		}
	}

	return 500
}

// strContains checks if string contains substring
func strContains(s, substr string) bool {
	return strings.Contains(s, substr)
}
