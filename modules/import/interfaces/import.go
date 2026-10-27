package interfaces

import (
	"context"
	"time"

	importModels "josex/web/modules/import/models"

	"github.com/google/uuid"
)

// ImportDescriptor encapsulates everything resource-specific about importing one
// resource. The generic engine owns parse/detect/limits/aggregation/recording;
// all field knowledge, validation, and persistence live behind this interface so
// a second resource is added by writing a descriptor, not by touching the engine.
type ImportDescriptor interface {
	// Resource is the stable identifier used in the API (e.g. "users").
	Resource() string
	// Columns are the expected headers in canonical order, each with the type and
	// validation the client needs to edit it (templates, docs, and /import/schema).
	Columns() []importModels.ColumnSpec
	// Matches reports whether a CSV with these (normalized) headers belongs to
	// this resource. This is the pluggable, header-signature detection strategy.
	Matches(headers []string) bool
	// ValidateRow performs a dry-run check of one row and returns its outcome
	// with status valid|invalid|duplicate. It must not write anything.
	ValidateRow(ctx importModels.ImportContext, line int, row map[string]string) importModels.RowResult
	// ProcessRow performs the write for one row and returns its outcome with
	// status created|failed|skipped.
	ProcessRow(ctx importModels.ImportContext, line int, row map[string]string) importModels.RowResult
	// RecordRun persists the run summary (D1). It is best-effort — the engine
	// ignores its outcome so a bookkeeping failure never fails the import.
	RecordRun(ctx importModels.ImportContext, meta importModels.RunMeta)
}

// OptionsProvider is a descriptor with run-wide switches — "Enviar invitación",
// "Crear las categorías que falten" — that the page draws from the schema.
type OptionsProvider interface {
	Options() []importModels.OptionSpec
}

// Updater is a descriptor that can update a record that already exists instead
// of skipping it (IMPORT-002 D4). KeyColumns name the columns that recognize the
// record (the page's "Lo reconocemos por <campo>"). When the operator chooses
// "Actualizar", the engine hands UpdateRow every row ProcessRow skipped as
// existing; it answers updated, skipped or failed.
type Updater interface {
	KeyColumns() []string
	UpdateRow(ctx importModels.ImportContext, line int, row map[string]string) importModels.RowResult
}

// ImportRepository is what the engine keeps between files: the remembered
// mappings (D3) and the history of runs.
type ImportRepository interface {
	GetMapping(ctx context.Context, tenantID uuid.UUID, resource, signature string) (map[string]string, error)
	UpsertMapping(ctx context.Context, tenantID uuid.UUID, resource, signature string, mapping map[string]string) error
	RecordRun(ctx context.Context, tenantID uuid.UUID, meta importModels.RunMeta) (uuid.UUID, error)
	ListRuns(ctx context.Context, tenantID uuid.UUID, query importModels.RunsQuery) (*importModels.RunsResponse, error)
	LastRuns(ctx context.Context, tenantID uuid.UUID) (map[string]time.Time, error)
}

// ImportRegistry holds the registered descriptors, each under the module that
// owns it, and resolves a CSV to one — by explicit resource key or by
// header-signature detection inside a scope. A scope is a list of resource and
// module keys; an empty one reaches every descriptor (IMPORT-001 D1).
//
// Registration order is also import order (A1-D1): a module registers the
// resources other ones point at first — categories before products, products
// before their stock and lots — and the wizard runs a multi-file drop in it.
type ImportRegistry interface {
	Register(module string, descriptor ImportDescriptor)
	Get(resource string) (ImportDescriptor, bool)
	Detect(headers []string, scope []string) (ImportDescriptor, bool)
	InScope(resource string, scope []string) bool
	Resources(scope []string) []importModels.ImportResourceInfo
}

// ImportService is the generic engine: it parses the CSV, resolves the
// descriptor, enforces limits, and runs the validate or process pass. A request
// names a resource, a scope to detect within, or both.
type ImportService interface {
	Validate(ctx importModels.ImportContext, req importModels.ImportRequest) (*importModels.ValidateResponse, error)
	Import(ctx importModels.ImportContext, req importModels.ImportRequest) (*importModels.ImportResponse, error)
	// Resources lists what a scope reaches, so a client opened on a module can
	// offer its resources without knowing them (D2), named in lang, with when the
	// tenant last imported each.
	Resources(ctx context.Context, tenantID uuid.UUID, lang string, scope []string) []importModels.ImportResourceInfo
	// Mapping answers the mapping remembered for a file with these headings (D3).
	Mapping(ctx context.Context, tenantID uuid.UUID, resource string, headers []string) (*importModels.MappingResponse, error)
	// Runs pages the tenant's import history.
	Runs(ctx context.Context, tenantID uuid.UUID, query importModels.RunsQuery) (*importModels.RunsResponse, error)
	// Detect names the resource a file is from its header line alone, without
	// reading or validating a row (A1-D1).
	Detect(target importModels.ImportTarget, data []byte, fileName string) (*importModels.DetectResponse, error)
	// Limits returns the configured upload constraints for the UI hint.
	Limits() importModels.ImportLimits
	// Template returns the CSV header row for a resource, built from its
	// descriptor's Columns() so each resource is a single source of truth.
	Template(resource, format string) ([]byte, error)
	// Schema returns the typed column specs for a resource so a client can render
	// and validate an editable cell per column without knowing the resource,
	// labelled in lang.
	Schema(resource, lang string) (*importModels.SchemaResponse, error)
	// ExtractImages safely inflates the optional companion image archive, applying
	// the configured per-entry, count and total caps.
	ExtractImages(archive []byte) (importModels.ImportImages, error)
}
