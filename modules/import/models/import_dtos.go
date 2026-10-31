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
	RowStatusUpdated = "updated"
	RowStatusFailed  = "failed"
	RowStatusSkipped = "skipped"
)

// What to do with a row whose record already exists (IMPORT-002 D4): skip it, as every resource
// always has, or update it — only for a descriptor that implements Updater.
const (
	OnExistingSkip   = "skip"
	OnExistingUpdate = "update"
)

// Where a heading's field came from, as the column step shows it (IMPORT-002).
const (
	HeaderSourceSaved    = "saved"
	HeaderSourceDetected = "detected"
	HeaderSourceChosen   = "chosen"
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
//
// Label is filled by the engine from `import.<resource>.<key>` in the request
// language (the key when untranslated). Aliases are the other headings a file
// may give the column — "Cód. Art." for sku — compared folded (lower case, no
// accents or punctuation); they stay on the server, which does the matching.
type ColumnSpec struct {
	Key      string     `json:"key"`
	Label    string     `json:"label"`
	Type     ColumnType `json:"type"`
	Required bool       `json:"required"`
	Pattern  string     `json:"pattern,omitempty"`
	Format   string     `json:"format,omitempty"`
	Aliases  []string   `json:"-"`
}

// OptionType is how the page draws a descriptor's extra switch.
type OptionType string

const (
	OptionTypeBool   OptionType = "bool"
	OptionTypeSelect OptionType = "select"
	OptionTypeUUID   OptionType = "uuid"
)

// OptionSpec is one run-wide switch a descriptor reads from ImportContext.Options
// ("Enviar invitación", "Crear las categorías que falten"). Label is filled by the
// engine from `import.<resource>.option.<key>`.
type OptionSpec struct {
	Key     string     `json:"key"`
	Label   string     `json:"label"`
	Type    OptionType `json:"type"`
	Default any        `json:"default"`
}

// SchemaResponse is the typed column list of one resource, consumed by the
// page to render and validate its editable cells, with the run's switches and
// whether an existing record can be updated, recognized by KeyLabel.
type SchemaResponse struct {
	Resource  string       `json:"resource"`
	Columns   []ColumnSpec `json:"columns"`
	Options   []OptionSpec `json:"options"`
	Updatable bool         `json:"updatable"`
	KeyLabel  string       `json:"key_label,omitempty"`
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

// ImportRequest is everything one validate or import upload carries beyond the
// file (IMPORT-002): the headings the operator mapped by hand, what to do with
// existing records, whether to remember the mapping, and the cells fixed or the
// rows dropped on the review step.
type ImportRequest struct {
	Target   ImportTarget
	Data     []byte
	FileName string
	// Mapping is {heading as written: field key}, a null key ignoring the column.
	Mapping    map[string]*string
	OnExisting string
	Remember   bool
	// Edits is {line: {field key: value}}, a null entry dropping the row.
	Edits map[int]map[string]string
}

// ImportResourceInfo is one resource a scope reaches and the module that owns
// it, as GET /import/resources lists them (D2), named in the request language,
// with when the tenant last imported it.
type ImportResourceInfo struct {
	Resource    string     `json:"resource"`
	Module      string     `json:"module"`
	Label       string     `json:"label"`
	Description string     `json:"description"`
	LastRunAt   *time.Time `json:"last_run_at"`
}

// MappingResponse is the remembered mapping of a file shape, or null.
type MappingResponse struct {
	Mapping map[string]string `json:"mapping"`
}

// HeaderMatch is one heading of the file and the field it fills: Key null is a
// column the run ignores. Example is its first non-empty cell.
type HeaderMatch struct {
	Header  string  `json:"header"`
	Key     *string `json:"key"`
	Source  string  `json:"source,omitempty"`
	Example string  `json:"example"`
}

// HeaderSuggestion is an unmatched heading that looks like a field no heading
// filled — "U. medida" for unit — offered as "Usar «U. medida»".
type HeaderSuggestion struct {
	Key    string `json:"key"`
	Header string `json:"header"`
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
	TenantSlug  string
	PerformedBy *uuid.UUID
	Lang        string
	Options     ImportOptions
	Images      ImportImages
	// OnExisting is OnExistingSkip or OnExistingUpdate (D4), set by the engine.
	OnExisting string
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
	ID          *uuid.UUID `json:"id"`
	Resource    string     `json:"resource"`
	FileName    string     `json:"file_name,omitempty"`
	Total       int        `json:"total"`
	Created     int        `json:"created"`
	Updated     int        `json:"updated"`
	Failed      int        `json:"failed"`
	Skipped     int        `json:"skipped"`
	Warnings    int        `json:"warnings"`
	PerformedBy *uuid.UUID `json:"performed_by,omitempty"`
	Timestamp   time.Time  `json:"timestamp"`
}

// PreviewRowCount is how many rows the file step shows (D2).
const PreviewRowCount = 5

// ValidateResponse is the dry-run result (D2): counts, how each heading maps,
// the first rows as written for the file step, and only the rows the review
// step must show, those with an error or a warning — not every row of a
// 5 000-row file.
//
// Preview cells follow Headers; an issue's Data is keyed by field, the cells
// the review step edits.
type ValidateResponse struct {
	Resource    string             `json:"resource"`
	Sheet       string             `json:"sheet,omitempty"`
	Total       int                `json:"total"`
	Valid       int                `json:"valid"`
	Warnings    int                `json:"warnings"`
	Invalid     int                `json:"invalid"`
	Existing    int                `json:"existing"`
	Headers     []HeaderMatch      `json:"headers"`
	Suggestions []HeaderSuggestion `json:"suggestions"`
	Preview     [][]string         `json:"preview"`
	Issues      []RowResult        `json:"issues"`
}

// ImportResponse is the process result: the recorded run, the rows that were
// not written — failed or skipped — for "Descargar filas omitidas", and the
// written ones that carry a warning (D2: rows with an error or a warning, never
// every row).
type ImportResponse struct {
	Resource   string      `json:"resource"`
	Run        RunMeta     `json:"run"`
	FailedRows []RowResult `json:"failed_rows"`
	WarnedRows []RowResult `json:"warned_rows"`
}

// RunRow is one past import, as the history lists it.
type RunRow struct {
	ID              uuid.UUID `json:"id"`
	Resource        string    `json:"resource"`
	FileName        *string   `json:"file_name"`
	Total           int       `json:"total"`
	Created         int       `json:"created"`
	Updated         int       `json:"updated"`
	Skipped         int       `json:"skipped"`
	Failed          int       `json:"failed"`
	PerformedByName *string   `json:"performed_by_name"`
	CreatedAt       time.Time `json:"created_at"`
}

// RunsQuery pages a tenant's history, one resource when set.
type RunsQuery struct {
	Resource string `query:"resource"`
	Page     int    `query:"page" default:"1" minimum:"1"`
	PageSize int    `query:"page_size" default:"20" minimum:"1" maximum:"100"`
}

// RunsResponse is one page of the history.
type RunsResponse struct {
	Runs       []RunRow `json:"runs"`
	TotalCount int64    `json:"total_count"`
}
