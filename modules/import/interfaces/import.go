package interfaces

import (
	importModels "josex/web/modules/import/models"
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
	Validate(ctx importModels.ImportContext, target importModels.ImportTarget, data []byte) (*importModels.ValidateResponse, error)
	Import(ctx importModels.ImportContext, target importModels.ImportTarget, data []byte) (*importModels.ImportResponse, error)
	// Resources lists what a scope reaches, so a client opened on a module can
	// offer its resources without knowing them (D2).
	Resources(scope []string) []importModels.ImportResourceInfo
	// Detect names the resource a file is from its header line alone, without
	// reading or validating a row (A1-D1).
	Detect(target importModels.ImportTarget, data []byte) (*importModels.DetectResponse, error)
	// Limits returns the configured upload constraints for the UI hint.
	Limits() importModels.ImportLimits
	// Template returns the CSV header row for a resource, built from its
	// descriptor's Columns() so each resource is a single source of truth.
	Template(resource string) ([]byte, error)
	// Schema returns the typed column specs for a resource so a client can render
	// and validate an editable cell per column without knowing the resource.
	Schema(resource string) (*importModels.SchemaResponse, error)
	// ExtractImages safely inflates the optional companion image archive, applying
	// the configured per-entry, count and total caps.
	ExtractImages(archive []byte) (importModels.ImportImages, error)
}
