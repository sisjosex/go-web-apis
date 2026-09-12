package services

import (
	"bytes"
	"encoding/csv"
	"io"
	"strconv"
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

// The two date shapes a cell may arrive in (IMPORT-001 D4); every descriptor
// reads the first.
const (
	isoDateLayout      = "2006-01-02"
	dayFirstDateLayout = "2/1/2006"
)

// utf8BOM is the byte-order mark Excel writes at the start of a "CSV UTF-8"
// file. Left in, it glues itself to the first header and no column matches.
var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

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

// Resources lists what a scope reaches (D2).
func (s *importService) Resources(scope []string) []importModels.ImportResourceInfo {
	return s.registry.Resources(scope)
}

// Detect names the resource a file is from its header line alone (A1-D1): the
// wizard queues several files by it, in import order, before any of them is
// validated — a lots file cannot be dry-run until its products exist.
func (s *importService) Detect(target importModels.ImportTarget, data []byte) (*importModels.DetectResponse, error) {
	headers, err := readHeaders(newCSVReader(data))
	if err != nil {
		return nil, err
	}

	descriptor, err := s.resolve(target, headers)
	if err != nil {
		return nil, err
	}

	response := &importModels.DetectResponse{Resource: descriptor.Resource()}
	for _, info := range s.registry.Resources(nil) {
		if info.Resource == response.Resource {
			response.Module = info.Module
		}
	}
	return response, nil
}

func (s *importService) Validate(ctx importModels.ImportContext, target importModels.ImportTarget, data []byte) (*importModels.ValidateResponse, error) {
	descriptor, rows, headers, err := s.prepare(target, data)
	if err != nil {
		return nil, err
	}
	ctx.Headers = headers
	ctx.Scratch = make(map[string]any)

	response := &importModels.ValidateResponse{
		Resource: descriptor.Resource(),
		Headers:  headers,
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

func (s *importService) Import(ctx importModels.ImportContext, target importModels.ImportTarget, data []byte) (*importModels.ImportResponse, error) {
	descriptor, rows, headers, err := s.prepare(target, data)
	if err != nil {
		return nil, err
	}
	ctx.Headers = headers
	ctx.Scratch = make(map[string]any)

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
		Headers:  headers,
		Run:      meta,
		Rows:     results,
	}, nil
}

// prepare parses the CSV, resolves the descriptor (explicit resource or header
// detection inside the scope), and enforces the row limit. The headers travel
// back out because a row is a map by the time a descriptor sees it:
// ImportContext.Headers is the only place the file's column order survives.
func (s *importService) prepare(target importModels.ImportTarget, data []byte) (importInterfaces.ImportDescriptor, []parsedRow, []string, error) {
	headers, rows, err := parseCSV(data)
	if err != nil {
		return nil, nil, nil, err
	}

	descriptor, err := s.resolve(target, headers)
	if err != nil {
		return nil, nil, nil, err
	}

	if len(rows) > readLimits().MaxRows {
		return nil, nil, nil, importErrors.ErrTooManyRows
	}

	normalizeCells(descriptor.Columns(), rows)

	return descriptor, rows, headers, nil
}

// resolve picks the descriptor a request targets. An explicit resource must sit
// inside the scope; otherwise the headers are matched within it. A file nothing
// in the scope recognizes but something outside it does is out-of-scope rather
// than unknown, so the operator is told where it belongs instead of that it is
// broken (IMPORT-001 D1).
func (s *importService) resolve(target importModels.ImportTarget, headers []string) (importInterfaces.ImportDescriptor, error) {
	if target.Resource != "" {
		descriptor, found := s.registry.Get(target.Resource)
		if !found {
			return nil, importErrors.ErrUnknownFormat
		}
		if !s.registry.InScope(target.Resource, target.Scope) {
			return nil, importErrors.ErrOutOfScope
		}
		return descriptor, nil
	}

	if descriptor, found := s.registry.Detect(headers, target.Scope); found {
		return descriptor, nil
	}
	if len(target.Scope) > 0 {
		if _, found := s.registry.Detect(headers, nil); found {
			return nil, importErrors.ErrOutOfScope
		}
	}
	return nil, importErrors.ErrUnknownFormat
}

// normalizeCells rewrites the typed cells a spreadsheet in a Spanish locale
// writes its own way — `1500,50`, `01/12/2026` — into the one shape every
// descriptor and the wizard read (IMPORT-001 D4). A cell that still does not
// parse is left as written, so the descriptor reports it in its own words.
func normalizeCells(columns []importModels.ColumnSpec, rows []parsedRow) {
	typed := make(map[string]importModels.ColumnType)
	for _, column := range columns {
		if column.Type == importModels.ColumnTypeNumber || column.Type == importModels.ColumnTypeDate {
			typed[column.Key] = column.Type
		}
	}
	if len(typed) == 0 {
		return
	}

	for _, row := range rows {
		for key, columnType := range typed {
			value := row.data[key]
			if value == "" {
				continue
			}
			if columnType == importModels.ColumnTypeNumber {
				row.data[key] = normalizeNumber(value)
			} else {
				row.data[key] = normalizeDate(value)
			}
		}
	}
}

// normalizeNumber reads a decimal comma, and a thousands separator when both
// marks appear, whichever comes last being the decimal one: `1500,50`,
// `1.500,50` and `1,500.50` all become `1500.50`.
func normalizeNumber(value string) string {
	lastComma := strings.LastIndex(value, ",")
	lastDot := strings.LastIndex(value, ".")

	candidate := value
	switch {
	case lastComma >= 0 && lastDot >= 0 && lastComma > lastDot:
		candidate = strings.Replace(strings.ReplaceAll(value, ".", ""), ",", ".", 1)
	case lastComma >= 0 && lastDot >= 0:
		candidate = strings.ReplaceAll(value, ",", "")
	case lastComma >= 0:
		candidate = strings.ReplaceAll(value, ",", ".")
	}

	if _, err := strconv.ParseFloat(candidate, 64); err != nil {
		return value
	}
	return candidate
}

// normalizeDate accepts the ISO date every descriptor reads and the
// day-first `DD/MM/YYYY` a Spanish spreadsheet shows (D4), one- or two-digit
// day and month.
func normalizeDate(value string) string {
	if _, err := time.Parse(isoDateLayout, value); err == nil {
		return value
	}
	parsed, err := time.Parse(dayFirstDateLayout, value)
	if err != nil {
		return value
	}
	return parsed.Format(isoDateLayout)
}

// sniffDelimiter picks the separator from the header line: Excel in a Spanish
// locale saves "CSV" with `;`, because `,` is its decimal mark. Separators
// inside a quoted header do not count.
func sniffDelimiter(data []byte) rune {
	commas, semicolons := 0, 0
	quoted := false
	for _, char := range data {
		if char == '\n' && !quoted {
			break
		}
		switch char {
		case '"':
			quoted = !quoted
		case ',':
			if !quoted {
				commas++
			}
		case ';':
			if !quoted {
				semicolons++
			}
		}
	}

	if semicolons > commas {
		return ';'
	}
	return ','
}

// newCSVReader opens a file the way every import reads it: a leading BOM
// dropped and the separator sniffed from the header line, both how Excel saves
// "CSV UTF-8".
func newCSVReader(data []byte) *csv.Reader {
	data = bytes.TrimPrefix(data, utf8BOM)

	reader := csv.NewReader(bytes.NewReader(data))
	reader.Comma = sniffDelimiter(data)
	reader.FieldsPerRecord = -1 // tolerate ragged rows; missing cells map to ""
	reader.TrimLeadingSpace = true
	return reader
}

// readHeaders reads the header line and normalizes it (trim + lowercase), the
// only part of a file detection needs.
func readHeaders(reader *csv.Reader) ([]string, error) {
	rawHeaders, err := reader.Read()
	if err == io.EOF {
		return nil, importErrors.ErrFileEmpty
	}
	if err != nil {
		return nil, importErrors.ErrFileInvalid
	}

	headers := make([]string, len(rawHeaders))
	for i, header := range rawHeaders {
		headers[i] = strings.ToLower(strings.TrimSpace(header))
	}
	return headers, nil
}

// parseCSV reads the CSV, normalizes headers (trim + lowercase), and returns each
// data row as a header-keyed, value-trimmed map paired with its CSV line number
// (header is line 1, so the first data row is line 2). A leading BOM and a `;`
// separator — both how Excel saves "CSV UTF-8" — are accepted as written.
func parseCSV(data []byte) ([]string, []parsedRow, error) {
	reader := newCSVReader(data)

	headers, err := readHeaders(reader)
	if err != nil {
		return nil, nil, err
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
		return nil, nil, importErrors.ErrFileNoRows
	}

	return headers, rows, nil
}
