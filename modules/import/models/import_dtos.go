package models

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// Row status values for the validate (dry-run) phase.
const (
	RowStatusValid     = "valid"
	RowStatusInvalid   = "invalid"
	RowStatusDuplicate = "duplicate"
)

// Row status values for the process phase.
const (
	RowStatusCreated = "created"
	RowStatusFailed  = "failed"
	RowStatusSkipped = "skipped"
)

// ColumnType is the editing/validation type a client applies to one import
// column. It is deliberately small: the wizard renders one editor per type.
type ColumnType string

const (
	ColumnTypeText   ColumnType = "text"
	ColumnTypeEmail  ColumnType = "email"
	ColumnTypePhone  ColumnType = "phone"
	ColumnTypeDate   ColumnType = "date"
	ColumnTypeNumber ColumnType = "number"
	ColumnTypeYear   ColumnType = "year"
	ColumnTypeURL    ColumnType = "url"
	ColumnTypeImage  ColumnType = "image"
)

// ColumnSpec describes one import column: its CSV header key, the editor/type
// the client uses for it, whether the engine rejects a row without it, and an
// optional resource-supplied pattern for customized validation (D8). Format is
// a human hint (e.g. "YYYY-MM-DD") the client may show as a placeholder.
type ColumnSpec struct {
	Key      string     `json:"key"`
	Type     ColumnType `json:"type"`
	Required bool       `json:"required"`
	Pattern  string     `json:"pattern,omitempty"`
	Format   string     `json:"format,omitempty"`
}

// SchemaResponse is the typed column list of one resource, consumed by the
// wizard to render and validate its editable cells.
type SchemaResponse struct {
	Resource string       `json:"resource"`
	Columns  []ColumnSpec `json:"columns"`
}

// ImportOptions carries caller-supplied, resource-specific switches parsed from
// the request's optional `options` JSON field (e.g. Users' send_invitation).
type ImportOptions map[string]any

// BoolOr returns the boolean option at key, or def when it is absent or not a bool.
func (o ImportOptions) BoolOr(key string, def bool) bool {
	value := def
	if raw, ok := o[key]; ok {
		if b, ok := raw.(bool); ok {
			value = b
		}
	}
	return value
}

// ImportLimits are the configured upload constraints, surfaced to clients so the
// UI hint stays in sync with the server (no hardcoded literal).
type ImportLimits struct {
	MaxFileSizeMB int `json:"max_file_size_mb"`
	MaxRows       int `json:"max_rows"`
}

// ImportImages holds the inflated companion archive, keyed by the lowercased
// basename of each entry. A descriptor looks a CSV cell up here; a miss is the
// descriptor's business (a warning for Users), not the engine's.
type ImportImages map[string][]byte

// Lookup finds an image by the filename a CSV cell names, matched
// case-insensitively against the archive's basenames.
func (i ImportImages) Lookup(filename string) ([]byte, bool) {
	if len(i) == 0 {
		return nil, false
	}
	content, ok := i[strings.ToLower(strings.TrimSpace(filename))]
	return content, ok
}

// ImportContext carries the per-request scope a descriptor needs: the tenant it
// runs in, the acting user, the request language, the parsed options, and the
// images extracted from the optional companion archive.
type ImportContext struct {
	TenantID    uuid.UUID
	PerformedBy *uuid.UUID
	Lang        string
	Options     ImportOptions
	Images      ImportImages
}

// RowResult is the per-row outcome of a validate or process run, keyed by the
// CSV line number (header is line 1, so the first data row is line 2).
type RowResult struct {
	Line     int               `json:"line"`
	Status   string            `json:"status"`
	Errors   []string          `json:"errors,omitempty"`   // mapped frontend error codes
	Warnings []string          `json:"warnings,omitempty"` // non-fatal mapped codes
	Data     map[string]string `json:"data"`               // original row, for client-side re-download of failures
}

// RunMeta is the persisted summary of a single import run (D1): resource,
// counts, warning total, actor, and timestamp.
type RunMeta struct {
	Resource    string     `json:"resource"`
	Total       int        `json:"total"`
	Created     int        `json:"created"`
	Failed      int        `json:"failed"`
	Skipped     int        `json:"skipped"`
	Warnings    int        `json:"warnings"`
	PerformedBy *uuid.UUID `json:"performed_by,omitempty"`
	Timestamp   time.Time  `json:"timestamp"`
}

// ValidateResponse is the dry-run result: per-row feedback plus aggregate counts.
type ValidateResponse struct {
	Resource  string      `json:"resource"`
	Total     int         `json:"total"`
	Valid     int         `json:"valid"`
	Invalid   int         `json:"invalid"`
	Duplicate int         `json:"duplicate"`
	Rows      []RowResult `json:"rows"`
	Warnings  int         `json:"warnings"`
}

// ImportResponse is the process result: per-row outcome plus the persisted run summary.
type ImportResponse struct {
	Resource string      `json:"resource"`
	Run      RunMeta     `json:"run"`
	Rows     []RowResult `json:"rows"`
}
