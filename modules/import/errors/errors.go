package errors

import "errors"

// Import module error codes. String values follow domain.action.error-type.
const (
	// File-level failures (multipart / CSV parsing). Empty and NoRows are distinct:
	// the first is a file with nothing in it, the second a header with no data below it.
	ImportFileRequired = "import.file.required"
	ImportFileTooLarge = "import.file.too-large"
	ImportFileEmpty    = "import.file.empty"
	ImportFileNoRows   = "import.file.no-rows"
	ImportFileInvalid  = "import.file.invalid"

	// Resource detection.
	ImportDetectUnknownFormat = "import.detect.unknown-format"

	// Limits.
	ImportTooManyRows = "import.too-many-rows"

	// Options parsing and generic validation.
	ImportOptionsInvalid   = "import.options.invalid"
	ImportValidationFailed = "import.validation-failed"

	// Companion image archive.
	ImportImagesInvalid       = "import.images.invalid"
	ImportImagesTooLarge      = "import.images.too-large"
	ImportImagesTooManyFiles  = "import.images.too-many-files"
	ImportImagesEntryTooLarge = "import.images.entry-too-large"
	ImportImagesEntryFormat   = "import.images.entry-format"
)

// Sentinel errors returned by the service so the controller can map them to the
// right HTTP status and error code without string matching.
var (
	ErrFileEmpty     = errors.New(ImportFileEmpty)
	ErrFileNoRows    = errors.New(ImportFileNoRows)
	ErrFileInvalid   = errors.New(ImportFileInvalid)
	ErrFileTooLarge  = errors.New(ImportFileTooLarge)
	ErrUnknownFormat = errors.New(ImportDetectUnknownFormat)
	ErrTooManyRows   = errors.New(ImportTooManyRows)

	ErrImagesInvalid       = errors.New(ImportImagesInvalid)
	ErrImagesTooLarge      = errors.New(ImportImagesTooLarge)
	ErrImagesTooManyFiles  = errors.New(ImportImagesTooManyFiles)
	ErrImagesEntryTooLarge = errors.New(ImportImagesEntryTooLarge)
	ErrImagesEntryFormat   = errors.New(ImportImagesEntryFormat)
)
