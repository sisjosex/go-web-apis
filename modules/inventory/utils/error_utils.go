package utils

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// GetHTTPStatusFromError maps PostgreSQL error messages and database errors to HTTP status codes
// Uses pattern matching to handle common inventory error types:
// - "not-found" → 404
// - pgx.ErrNoRows → 404 (no rows found in query result)
// - "already-exists" → 409
// - "duplicate" → 409
// - "insufficient-" → 409
// - "invalid-" → 400
// Returns 500 for unmapped errors
func GetHTTPStatusFromError(err error) int {
	if err == nil {
		return http.StatusOK
	}

	// Handle pgx.ErrNoRows (no rows in result set from QueryRow)
	if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows) {
		return http.StatusNotFound
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		errorMsg := strings.ToLower(pgErr.Message)

		// Map common error patterns to HTTP status codes
		if strings.Contains(errorMsg, "not-found") {
			return http.StatusNotFound
		}
		if strings.Contains(errorMsg, "already-exists") || strings.Contains(errorMsg, "duplicate") {
			return http.StatusConflict
		}
		if strings.Contains(errorMsg, "insufficient") {
			return http.StatusConflict
		}
		if strings.Contains(errorMsg, "invalid") {
			return http.StatusBadRequest
		}
	}

	// Default to 500 for unknown errors
	return http.StatusInternalServerError
}
