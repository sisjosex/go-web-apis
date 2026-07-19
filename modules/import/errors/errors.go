package errors

import "errors"

// Import module error codes. String values follow domain.action.error-type.
const (
	// File-level failures (multipart / CSV parsing).
	ImportFileRequired = "import.file.required"
	ImportFileTooLarge = "import.file.too-large"
	ImportFileEmpty    = "import.file.empty"
	ImportFileInvalid  = "import.file.invalid"

	// Resource detection.
	ImportDetectUnknownFormat = "import.detect.unknown-format"

	// Limits.
	ImportTooManyRows = "import.too-many-rows"

	// Options parsing and generic validation.
	ImportOptionsInvalid   = "import.options.invalid"
	ImportValidationFailed = "import.validation-failed"
)

// Sentinel errors returned by the service so the controller can map them to the
// right HTTP status and error code without string matching.
var (
	ErrFileEmpty     = errors.New(ImportFileEmpty)
	ErrFileInvalid   = errors.New(ImportFileInvalid)
	ErrUnknownFormat = errors.New(ImportDetectUnknownFormat)
	ErrTooManyRows   = errors.New(ImportTooManyRows)
)
