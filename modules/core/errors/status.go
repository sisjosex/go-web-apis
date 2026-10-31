package errors

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	statusMu sync.RWMutex
	statuses = map[string]int{}
)

// RegisterStatus names the HTTP status of one error code the rules in StatusOf would get wrong. A module
// calls it from an init().
func RegisterStatus(code string, status int) {
	statusMu.Lock()
	defer statusMu.Unlock()
	statuses[code] = status
}

// StatusOf is the HTTP status of an SP's error: a registered code first, then the naming rules, and
// 500 for anything that names no rule.
func StatusOf(err error) int {
	if err == nil {
		return http.StatusInternalServerError
	}
	if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows) {
		return http.StatusNotFound
	}
	code := err.Error()
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		code = pgErr.Message
	}
	statusMu.RLock()
	status, ok := statuses[code]
	statusMu.RUnlock()
	if ok {
		return status
	}
	return statusByName(code)
}

func statusByName(code string) int {
	switch {
	case strings.Contains(code, "not-found"):
		return http.StatusNotFound
	case strings.HasPrefix(code, "validation."):
		return http.StatusBadRequest
	case strings.Contains(code, "exists"), strings.Contains(code, "duplicate"), strings.Contains(code, "insufficient"), strings.Contains(code, "invalid-status"):
		return http.StatusConflict
	case strings.Contains(code, "invalid"):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}
