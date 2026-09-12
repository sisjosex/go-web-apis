package models

import (
	"context"
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

// ImportTarget is what a request asks the engine to resolve: an explicit
// resource, a scope of resource and module keys to detect within, or both — an
// explicit resource outside its own scope is refused (IMPORT-001 D1).
type ImportTarget struct {
	Resource string
	Scope    []string
}

// ImportResourceInfo is one resource a scope reaches and the module that owns
// it, as GET /import/resources lists them (D2).
type ImportResourceInfo struct {
	Resource string `json:"resource"`
	Module   string `json:"module"`
}

// DetectResponse is the resource a file's header line resolves to and the
// module that owns it, as POST /import/detect answers (A1-D1).
type DetectResponse struct {
	Resource string `json:"resource"`
	Module   string `json:"module"`
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
// runs in, the acting user, the request language, the parsed options, the images
// extracted from the optional companion archive, and the file's columns in the
// order they were written.
type ImportContext struct {
	// Ctx is the request's context, so a cancelled upload stops the run's
	// queries instead of finishing them for nobody. Read it through Context().
	Ctx         context.Context
	TenantID    uuid.UUID
	PerformedBy *uuid.UUID
	Lang        string
	Options     ImportOptions
	Images      ImportImages
	// Headers are the file's normalized headers in column order, as parseCSV
	// read them. A row reaches a descriptor as a map, which has no order, so a
	// descriptor whose columns are not fixed in advance — the products file and
	// its variant[<axis>] markers (INV-016 D1) — has no other way to tell which
	// of them the operator wrote first. Set by the engine on every run; a
	// descriptor with a fixed column list ignores it.
	Headers []string
	// Scratch is per-run state a descriptor keeps across the rows of one file:
	// the engine creates it empty at the start of every validate or process run
	// and never reads it. A descriptor is registered once and shared by every
	// request, so this — not a field on the descriptor — is where a ledger that
	// must not leak between concurrent runs belongs. Rows of one run are
	// processed in order on one goroutine, so it needs no lock.
	Scratch map[string]any
}

// Context returns the request's context, or Background for a run built without
// one — a unit test driving a descriptor directly.
func (c ImportContext) Context() context.Context {
	if c.Ctx == nil {
		return context.Background()
	}
	return c.Ctx
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
// Headers is the file's column order — a row's data is a map, so it is the only
// way a client can write the file back in the order the operator wrote it.
type ValidateResponse struct {
	Resource  string      `json:"resource"`
	Headers   []string    `json:"headers"`
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
	Headers  []string    `json:"headers"`
	Run      RunMeta     `json:"run"`
	Rows     []RowResult `json:"rows"`
}
