package utils

import (
	"github.com/jackc/pgx/v5/pgconn"
)

// ExtractTrackingErrorCode extracts error code from PL/pgsql exceptions
// Expected format: "error.code" (e.g., "company.not-found")
// The error message from PostgreSQL is used directly as the error code
func ExtractTrackingErrorCode(err error) string {
	if err == nil {
		return ""
	}

	pgErr, ok := err.(*pgconn.PgError)
	if !ok {
		return ""
	}

	// Return message directly (contains error code like "company.not-found")
	return pgErr.Message
}
