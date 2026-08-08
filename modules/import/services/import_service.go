package services

import (
	"bytes"
	"encoding/csv"
	"io"
	"strings"
	"time"

	"josex/web/config"
	importErrors "josex/web/modules/import/errors"
	importInterfaces "josex/web/modules/import/interfaces"
	importModels "josex/web/modules/import/models"
)

// Fallback limits used only when config has not been loaded (e.g. isolated tests).
const (
	defaultMaxFileSizeMB = 5
	defaultMaxRows       = 500
)

type importService struct {
	registry importInterfaces.ImportRegistry
}

// readLimits resolves the configured import limits, falling back to defaults.
func readLimits() importModels.ImportLimits {
	limits := importModels.ImportLimits{MaxFileSizeMB: defaultMaxFileSizeMB, MaxRows: defaultMaxRows}
	if config.ModularAppConfig != nil && config.ModularAppConfig.Import != nil {
		limits.MaxFileSizeMB = config.ModularAppConfig.Import.MaxFileSizeMB
		limits.MaxRows = config.ModularAppConfig.Import.MaxRows
	}
	return limits
}

// NewImportService creates the generic import engine backed by the given registry.
func NewImportService(registry importInterfaces.ImportRegistry) importInterfaces.ImportService {
	return &importService{registry: registry}
}

// Limits returns the configured upload constraints.
func (s *importService) Limits() importModels.ImportLimits {
	return readLimits()
}

// Template returns the CSV header row for a resource, built from its descriptor's
// Columns(). Unknown resource yields ErrUnknownFormat.
func (s *importService) Template(resource string) ([]byte, error) {
	descriptor, ok := s.registry.Get(resource)
	if !ok {
		return nil, importErrors.ErrUnknownFormat
	}

	columns := descriptor.Columns()
	headers := make([]string, len(columns))
	for i, column := range columns {
		headers[i] = column.Key
	}

	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)
	if err := writer.Write(headers); err != nil {
		return nil, err
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Schema returns the resource's typed column specs. Unknown resource yields
// ErrUnknownFormat.
func (s *importService) Schema(resource string) (*importModels.SchemaResponse, error) {
	descriptor, ok := s.registry.Get(resource)
	if !ok {
		return nil, importErrors.ErrUnknownFormat
	}

	return &importModels.SchemaResponse{
		Resource: descriptor.Resource(),
		Columns:  descriptor.Columns(),
	}, nil
}

// ExtractImages inflates the optional companion image archive. The entry cap is
// the engine's row limit: an archive can never carry more images than the run
// can import rows.
func (s *importService) ExtractImages(archive []byte) (importModels.ImportImages, error) {
	return extractImages(archive, readLimits().MaxRows)
}

// parsedRow is one CSV data row paired with its 1-based CSV line number.
type parsedRow struct {
	line int
	data map[string]string
}

func (s *importService) Validate(ctx importModels.ImportContext, resource string, data []byte) (*importModels.ValidateResponse, error) {
	descriptor, rows, err := s.prepare(resource, data)
	if err != nil {
		return nil, err
	}

	response := &importModels.ValidateResponse{
		Resource: descriptor.Resource(),
		Total:    len(rows),
		Rows:     make([]importModels.RowResult, 0, len(rows)),
	}

	for _, row := range rows {
		result := descriptor.ValidateRow(ctx, row.line, row.data)
		switch result.Status {
		case importModels.RowStatusValid:
			response.Valid++
		case importModels.RowStatusDuplicate:
			response.Duplicate++
		default:
			response.Invalid++
		}
		response.Warnings += len(result.Warnings)
		response.Rows = append(response.Rows, result)
	}

	return response, nil
}

func (s *importService) Import(ctx importModels.ImportContext, resource string, data []byte) (*importModels.ImportResponse, error) {
	descriptor, rows, err := s.prepare(resource, data)
	if err != nil {
		return nil, err
	}

	meta := importModels.RunMeta{
		Resource:    descriptor.Resource(),
		Total:       len(rows),
		PerformedBy: ctx.PerformedBy,
		Timestamp:   time.Now().UTC(),
	}
	results := make([]importModels.RowResult, 0, len(rows))

	for _, row := range rows {
		result := descriptor.ProcessRow(ctx, row.line, row.data)
		switch result.Status {
		case importModels.RowStatusCreated:
			meta.Created++
		case importModels.RowStatusSkipped:
			meta.Skipped++
		default:
			meta.Failed++
		}
		meta.Warnings += len(result.Warnings)
		results = append(results, result)
	}

	descriptor.RecordRun(ctx, meta)

	return &importModels.ImportResponse{
		Resource: descriptor.Resource(),
		Run:      meta,
		Rows:     results,
	}, nil
}

// prepare parses the CSV, resolves the descriptor (explicit resource or header
// detection), and enforces the row limit.
func (s *importService) prepare(resource string, data []byte) (importInterfaces.ImportDescriptor, []parsedRow, error) {
	headers, rows, err := parseCSV(data)
	if err != nil {
		return nil, nil, err
	}

	var descriptor importInterfaces.ImportDescriptor
	if resource != "" {
		found := false
		if descriptor, found = s.registry.Get(resource); !found {
			return nil, nil, importErrors.ErrUnknownFormat
		}
	} else {
		found := false
		if descriptor, found = s.registry.Detect(headers); !found {
			return nil, nil, importErrors.ErrUnknownFormat
		}
	}

	if len(rows) > readLimits().MaxRows {
		return nil, nil, importErrors.ErrTooManyRows
	}

	return descriptor, rows, nil
}

// parseCSV reads the CSV, normalizes headers (trim + lowercase), and returns each
// data row as a header-keyed, value-trimmed map paired with its CSV line number
// (header is line 1, so the first data row is line 2).
func parseCSV(data []byte) ([]string, []parsedRow, error) {
	reader := csv.NewReader(bytes.NewReader(data))
	reader.FieldsPerRecord = -1 // tolerate ragged rows; missing cells map to ""
	reader.TrimLeadingSpace = true

	rawHeaders, err := reader.Read()
	if err == io.EOF {
		return nil, nil, importErrors.ErrFileEmpty
	}
	if err != nil {
		return nil, nil, importErrors.ErrFileInvalid
	}

	headers := make([]string, len(rawHeaders))
	for i, header := range rawHeaders {
		headers[i] = strings.ToLower(strings.TrimSpace(header))
	}

	var rows []parsedRow
	line := 1 // header consumed above
	for {
		record, readErr := reader.Read()
		if readErr == io.EOF {
			break
		}
		line++
		if readErr != nil {
			return nil, nil, importErrors.ErrFileInvalid
		}

		row := make(map[string]string, len(headers))
		for i, header := range headers {
			if header == "" {
				continue
			}
			value := ""
			if i < len(record) {
				value = strings.TrimSpace(record[i])
			}
			row[header] = value
		}
		rows = append(rows, parsedRow{line: line, data: row})
	}

	if len(rows) == 0 {
		return nil, nil, importErrors.ErrFileEmpty
	}

	return headers, rows, nil
}
