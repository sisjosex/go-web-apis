package utils

import (
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
)

// ExtractTrackingErrorCode extracts error code from PL/pgsql TRACKING_ERROR
// Expected format: "TRACKING_ERROR:error.code"
// Example: "TRACKING_ERROR:company.not-found"
func ExtractTrackingErrorCode(err error) string {
	if err == nil {
		return ""
	}

	pgErr, ok := err.(*pgconn.PgError)
	if !ok {
		return ""
	}

	// Message format: TRACKING_ERROR:error.code
	message := pgErr.Message
	if strings.HasPrefix(message, "TRACKING_ERROR:") {
		return strings.TrimPrefix(message, "TRACKING_ERROR:")
	}

	return ""
}
