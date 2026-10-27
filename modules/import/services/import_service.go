package services

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"

	"josex/web/config"
	"josex/web/modules/core/utils"
	importErrors "josex/web/modules/import/errors"
	importInterfaces "josex/web/modules/import/interfaces"
	importModels "josex/web/modules/import/models"
)

// Fallback limits used only when config has not been loaded (e.g. isolated tests).
const (
	defaultMaxFileSizeMB = 5
	defaultMaxRows       = 5000
)

// The two date shapes a cell may arrive in (IMPORT-001 D4); every descriptor
// reads the first.
const (
	isoDateLayout      = "2006-01-02"
	dayFirstDateLayout = "2/1/2006"
)

// RowUnchanged is why an existing record the operator chose to update was skipped: the row would
// change nothing on it (D4).
const RowUnchanged = "import.row.unchanged"

// The template formats GET /import/template writes.
const (
	TemplateCSV  = "csv"
	TemplateXLSX = "xlsx"
)

// utf8BOM is the byte-order mark Excel writes at the start of a "CSV UTF-8"
// file. Left in, it glues itself to the first header and no column matches.
var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

type importService struct {
	registry importInterfaces.ImportRegistry
	// repo keeps the remembered mappings and the history; nil runs the engine
	// without either (a unit test driving descriptors).
	repo importInterfaces.ImportRepository
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

// NewImportService creates the generic import engine backed by the given registry
// and, when not nil, the repository of mappings and runs.
func NewImportService(registry importInterfaces.ImportRegistry, repo importInterfaces.ImportRepository) importInterfaces.ImportService {
	return &importService{registry: registry, repo: repo}
}

// Limits returns the configured upload constraints.
func (s *importService) Limits() importModels.ImportLimits {
	return readLimits()
}

// Template returns the header row for a resource, built from its descriptor's
// Columns(), as CSV or as an Excel workbook. Unknown resource yields
// ErrUnknownFormat.
func (s *importService) Template(resource, format string) ([]byte, error) {
	descriptor, ok := s.registry.Get(resource)
	if !ok {
		return nil, importErrors.ErrUnknownFormat
	}

	columns := descriptor.Columns()
	headers := make([]string, len(columns))
	for i, column := range columns {
		headers[i] = column.Key
	}

	if format == TemplateXLSX {
		return workbookTemplate(headers)
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

func workbookTemplate(headers []string) ([]byte, error) {
	book := excelize.NewFile()
	defer func() { _ = book.Close() }()
	cells := make([]any, len(headers))
	for i, header := range headers {
		cells[i] = header
	}
	if err := book.SetSheetRow(book.GetSheetName(0), "A1", &cells); err != nil {
		return nil, err
	}
	buf, err := book.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Schema returns the resource's typed column specs labelled in lang, its run
// switches and whether an existing record can be updated. Unknown resource
// yields ErrUnknownFormat.
func (s *importService) Schema(resource, lang string) (*importModels.SchemaResponse, error) {
	descriptor, ok := s.registry.Get(resource)
	if !ok {
		return nil, importErrors.ErrUnknownFormat
	}

	response := &importModels.SchemaResponse{
		Resource: descriptor.Resource(),
		Columns:  labelledColumns(descriptor, lang),
		Options:  []importModels.OptionSpec{},
	}
	if provider, ok := descriptor.(importInterfaces.OptionsProvider); ok {
		for _, option := range provider.Options() {
			option.Label = translateOr(lang, "import."+resource+".option."+option.Key, option.Key)
			response.Options = append(response.Options, option)
		}
	}
	if updater, ok := descriptor.(importInterfaces.Updater); ok {
		response.Updatable = true
		if keys := updater.KeyColumns(); len(keys) > 0 {
			response.KeyLabel = translateOr(lang, "import."+resource+"."+keys[0], keys[0])
		}
	}
	return response, nil
}

// labelledColumns is the descriptor's columns with each label in lang.
func labelledColumns(descriptor importInterfaces.ImportDescriptor, lang string) []importModels.ColumnSpec {
	columns := descriptor.Columns()
	out := make([]importModels.ColumnSpec, len(columns))
	for i, column := range columns {
		column.Label = translateOr(lang, "import."+descriptor.Resource()+"."+column.Key, column.Key)
		out[i] = column
	}
	return out
}

// translateOr is the translation of key in lang, or fallback when there is none.
func translateOr(lang, key, fallback string) string {
	if text := utils.GetTranslation(lang, key); text != key {
		return text
	}
	return fallback
}

// ExtractImages inflates the optional companion image archive. The entry cap is
// the engine's row limit: an archive can never carry more images than the run
// can import rows.
func (s *importService) ExtractImages(archive []byte) (importModels.ImportImages, error) {
	return extractImages(archive, readLimits().MaxRows)
}

// parsedRow is one data row, keyed by field, paired with its line in the file
// (the heading line is 1, so the first data row is line 2). dropped is a row the
// operator chose to leave out on the review step.
type parsedRow struct {
	line    int
	data    map[string]string
	dropped bool
}

// Resources lists what a scope reaches (D2), named in lang, with when the
// tenant last imported each.
func (s *importService) Resources(ctx context.Context, tenantID uuid.UUID, lang string, scope []string) []importModels.ImportResourceInfo {
	resources := s.registry.Resources(scope)
	var lastRuns map[string]time.Time
	if s.repo != nil && tenantID != uuid.Nil {
		var err error
		if lastRuns, err = s.repo.LastRuns(ctx, tenantID); err != nil {
			slog.Warn("import: last runs unread", "err", err)
		}
	}
	for i := range resources {
		key := "import.resource." + resources[i].Resource
		resources[i].Label = translateOr(lang, key, resources[i].Resource)
		resources[i].Description = translateOr(lang, key+".description", "")
		if at, ok := lastRuns[resources[i].Resource]; ok {
			resources[i].LastRunAt = &at
		}
	}
	return resources
}

// Mapping answers the mapping remembered for a file with these headings (D3).
func (s *importService) Mapping(ctx context.Context, tenantID uuid.UUID, resource string, headers []string) (*importModels.MappingResponse, error) {
	if _, ok := s.registry.Get(resource); !ok {
		return nil, importErrors.ErrUnknownFormat
	}
	if s.repo == nil {
		return &importModels.MappingResponse{}, nil
	}
	mapping, err := s.repo.GetMapping(ctx, tenantID, resource, headerSignature(headers))
	if err != nil {
		return nil, err
	}
	return &importModels.MappingResponse{Mapping: mapping}, nil
}

// Runs pages the tenant's import history.
func (s *importService) Runs(ctx context.Context, tenantID uuid.UUID, query importModels.RunsQuery) (*importModels.RunsResponse, error) {
	if s.repo == nil {
		return &importModels.RunsResponse{Runs: []importModels.RunRow{}}, nil
	}
	return s.repo.ListRuns(ctx, tenantID, query)
}

// Detect names the resource a file is from its heading line alone (A1-D1): the
// page queues several files by it, in import order, before any of them is
// validated — a lots file cannot be dry-run until its products exist.
func (s *importService) Detect(target importModels.ImportTarget, data []byte, fileName string) (*importModels.DetectResponse, error) {
	file, err := readFile(data, fileName, 0)
	if err != nil {
		return nil, err
	}

	descriptor, err := s.resolve(target, file.headers, nil)
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

func (s *importService) Validate(ctx importModels.ImportContext, req importModels.ImportRequest) (*importModels.ValidateResponse, error) {
	run, err := s.prepare(ctx, req)
	if err != nil {
		return nil, err
	}
	ctx.Headers = run.keys
	ctx.Scratch = make(map[string]any)
	ctx.OnExisting = onExisting(req.OnExisting)

	response := &importModels.ValidateResponse{
		Resource:    run.descriptor.Resource(),
		Sheet:       run.file.sheet,
		Total:       len(run.rows),
		Headers:     run.matches,
		Suggestions: suggestions(run.descriptor, ctx.Lang, run.matches),
		Preview:     preview(run.file),
		Issues:      []importModels.RowResult{},
	}

	for _, row := range run.rows {
		if row.dropped {
			continue
		}
		result := run.descriptor.ValidateRow(ctx, row.line, row.data)
		switch result.Status {
		case importModels.RowStatusValid:
			if len(result.Warnings) > 0 {
				response.Warnings++
			} else {
				response.Valid++
			}
		case importModels.RowStatusDuplicate:
			response.Existing++
		default:
			response.Invalid++
		}
		if result.Status != importModels.RowStatusValid || len(result.Warnings) > 0 {
			response.Issues = append(response.Issues, result)
		}
	}

	return response, nil
}

func (s *importService) Import(ctx importModels.ImportContext, req importModels.ImportRequest) (*importModels.ImportResponse, error) {
	run, err := s.prepare(ctx, req)
	if err != nil {
		return nil, err
	}
	ctx.Headers = run.keys
	ctx.Scratch = make(map[string]any)
	ctx.OnExisting = onExisting(req.OnExisting)

	tally := runTally{meta: importModels.RunMeta{
		Resource:    run.descriptor.Resource(),
		FileName:    req.FileName,
		Total:       len(run.rows),
		PerformedBy: ctx.PerformedBy,
		Timestamp:   time.Now().UTC(),
	}, failed: []importModels.RowResult{}, warned: []importModels.RowResult{}}
	for _, row := range run.rows {
		tally.add(processRow(ctx, run.descriptor, row))
	}
	meta := tally.meta

	run.descriptor.RecordRun(ctx, meta)
	s.record(ctx, &meta)
	if req.Remember {
		s.remember(ctx, run)
	}

	return &importModels.ImportResponse{
		Resource:   run.descriptor.Resource(),
		Run:        meta,
		FailedRows: tally.failed,
		WarnedRows: tally.warned,
	}, nil
}

// processRow writes one row: a dropped one is skipped, and a row skipped as
// existing goes to the descriptor's Updater when the operator chose "Actualizar"
// (D4) — skipped again, with the reason, when it would change nothing.
func processRow(ctx importModels.ImportContext, descriptor importInterfaces.ImportDescriptor, row parsedRow) importModels.RowResult {
	if row.dropped {
		return importModels.RowResult{Line: row.line, Status: importModels.RowStatusSkipped, Data: row.data}
	}
	result := descriptor.ProcessRow(ctx, row.line, row.data)
	updater, updatable := descriptor.(importInterfaces.Updater)
	if !updatable || ctx.OnExisting != importModels.OnExistingUpdate || result.Status != importModels.RowStatusSkipped {
		return result
	}
	result = updater.UpdateRow(ctx, row.line, row.data)
	if result.Status == importModels.RowStatusSkipped && len(result.Errors) == 0 {
		result.Errors = []string{RowUnchanged}
	}
	return result
}

// runTally counts a run's rows and keeps the ones the answer lists: those not
// written, and those written with a warning.
type runTally struct {
	meta   importModels.RunMeta
	failed []importModels.RowResult
	warned []importModels.RowResult
}

func (t *runTally) add(result importModels.RowResult) {
	switch result.Status {
	case importModels.RowStatusCreated:
		t.meta.Created++
	case importModels.RowStatusUpdated:
		t.meta.Updated++
	case importModels.RowStatusSkipped:
		t.meta.Skipped++
	default:
		t.meta.Failed++
	}
	written := result.Status == importModels.RowStatusCreated || result.Status == importModels.RowStatusUpdated
	if !written {
		t.failed = append(t.failed, result)
	} else if len(result.Warnings) > 0 {
		t.warned = append(t.warned, result)
	}
	t.meta.Warnings += len(result.Warnings)
}

// record writes the run to the history. Best-effort: the rows are already
// written, so a bookkeeping failure never fails the import.
func (s *importService) record(ctx importModels.ImportContext, meta *importModels.RunMeta) {
	if s.repo == nil || ctx.TenantID == uuid.Nil {
		return
	}
	id, err := s.repo.RecordRun(ctx.Context(), ctx.TenantID, *meta)
	if err != nil {
		slog.Warn("import: run not recorded", "resource", meta.Resource, "err", err)
		return
	}
	meta.ID = &id
}

// remember keeps how this file shape mapped (D3), every heading that filled a
// field, so the next file with the same headings maps itself.
func (s *importService) remember(ctx importModels.ImportContext, run *preparedRun) {
	if s.repo == nil || ctx.TenantID == uuid.Nil {
		return
	}
	mapping := map[string]string{}
	for _, match := range run.matches {
		if match.Key != nil {
			mapping[foldHeading(match.Header)] = *match.Key
		}
	}
	if err := s.repo.UpsertMapping(ctx.Context(), ctx.TenantID, run.descriptor.Resource(), run.signature, mapping); err != nil {
		slog.Warn("import: mapping not remembered", "resource", run.descriptor.Resource(), "err", err)
	}
}

func onExisting(value string) string {
	if value == importModels.OnExistingUpdate {
		return value
	}
	return importModels.OnExistingSkip
}

// preparedRun is a file read, resolved to its descriptor, its headings mapped
// to fields and its rows keyed by them.
type preparedRun struct {
	descriptor importInterfaces.ImportDescriptor
	file       *sheetFile
	matches    []importModels.HeaderMatch
	// keys are the field each column fills, in file order: a column without a
	// field keeps its lower-cased heading — the products file's variant[<axis>]
	// markers (INV-016 D1) are read that way — and an ignored one is left out.
	keys      []string
	rows      []parsedRow
	signature string
}

// prepare reads the file, resolves the descriptor (explicit resource or
// detection inside the scope), maps the headings — chosen by hand, else
// remembered for this file shape, else matched by key, label or alias — and
// applies the review step's edits.
func (s *importService) prepare(ctx importModels.ImportContext, req importModels.ImportRequest) (*preparedRun, error) {
	maxRows := readLimits().MaxRows
	file, err := readFile(req.Data, req.FileName, maxRows)
	if err != nil {
		return nil, err
	}
	if len(file.records) == 0 {
		return nil, importErrors.ErrFileNoRows
	}
	if len(file.records) > maxRows {
		return nil, importErrors.ErrTooManyRows
	}

	descriptor, err := s.resolve(req.Target, file.headers, req.Mapping)
	if err != nil {
		return nil, err
	}

	run := &preparedRun{descriptor: descriptor, file: file, signature: headerSignature(file.headers)}
	var saved map[string]string
	if s.repo != nil && ctx.TenantID != uuid.Nil {
		if saved, err = s.repo.GetMapping(ctx.Context(), ctx.TenantID, descriptor.Resource(), run.signature); err != nil {
			slog.Warn("import: saved mapping unread", "resource", descriptor.Resource(), "err", err)
		}
	}
	run.matches = mapHeaders(descriptor, file, req.Mapping, saved)

	columnKeys := columnKeysOf(file, run.matches)
	for _, key := range columnKeys {
		if key != "" {
			run.keys = append(run.keys, key)
		}
	}
	for index, record := range file.records {
		run.rows = append(run.rows, rowOf(index+2, record, columnKeys, req.Edits))
	}

	normalizeCells(descriptor.Columns(), run.rows)
	return run, nil
}

// columnKeysOf is the field each column fills: its match, else its lower-cased
// heading, else "" for a column ignored by choice or without a heading.
func columnKeysOf(file *sheetFile, matches []importModels.HeaderMatch) []string {
	keys := make([]string, len(file.headers))
	for i, match := range matches {
		switch {
		case match.Key != nil:
			keys[i] = *match.Key
		case match.Source == importModels.HeaderSourceChosen || file.headers[i] == "":
			keys[i] = ""
		default:
			keys[i] = strings.ToLower(file.headers[i])
		}
	}
	return keys
}

// rowOf keys one record by field and applies its review edit: fixed cells
// replace the file's, a null edit drops the row. Two headings on one field: the
// filled one wins.
func rowOf(line int, record []string, columnKeys []string, edits map[int]map[string]string) parsedRow {
	row := parsedRow{line: line, data: make(map[string]string, len(columnKeys))}
	for i, key := range columnKeys {
		value := ""
		if i < len(record) {
			value = record[i]
		}
		if _, set := row.data[key]; key == "" || (set && value == "") {
			continue
		}
		row.data[key] = value
	}
	if edit, edited := edits[line]; edited {
		row.dropped = edit == nil
		for key, value := range edit {
			row.data[key] = strings.TrimSpace(value)
		}
	}
	return row
}

// resolve picks the descriptor a request targets. An explicit resource must sit
// inside the scope; otherwise the headings are matched within it, each
// descriptor reading them through its own aliases. A file nothing in the scope
// recognizes but something outside it does is out-of-scope rather than
// unknown, so the operator is told where it belongs instead of that it is
// broken (IMPORT-001 D1).
func (s *importService) resolve(target importModels.ImportTarget, headers []string, chosen map[string]*string) (importInterfaces.ImportDescriptor, error) {
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

	file := &sheetFile{headers: headers}
	if descriptor := s.detect(file, target.Scope, chosen); descriptor != nil {
		return descriptor, nil
	}
	if len(target.Scope) > 0 && s.detect(file, nil, chosen) != nil {
		return nil, importErrors.ErrOutOfScope
	}
	return nil, importErrors.ErrUnknownFormat
}

// detect is the first descriptor in the scope, in registration order, whose
// header signature matches the headings as it maps them.
func (s *importService) detect(file *sheetFile, scope []string, chosen map[string]*string) importInterfaces.ImportDescriptor {
	for _, info := range s.registry.Resources(scope) {
		descriptor, _ := s.registry.Get(info.Resource)
		keys := make([]string, 0, len(file.headers))
		for i, match := range mapHeaders(descriptor, file, chosen, nil) {
			if match.Key != nil {
				keys = append(keys, *match.Key)
			} else {
				keys = append(keys, strings.ToLower(file.headers[i]))
			}
		}
		if descriptor.Matches(keys) {
			return descriptor
		}
	}
	return nil
}

// mapHeaders resolves each heading to a field: the operator's choice (a null
// one ignores the column), else the mapping remembered for this file shape,
// else a heading equal, folded, to a field's key, label (Spanish or English)
// or alias. A field is filled by one heading only — the first that claims it.
func mapHeaders(descriptor importInterfaces.ImportDescriptor, file *sheetFile, chosen map[string]*string, saved map[string]string) []importModels.HeaderMatch {
	columns := descriptor.Columns()
	known := make(map[string]bool, len(columns))
	for _, column := range columns {
		known[column.Key] = true
	}
	detected := headingIndex(descriptor)
	byFold := make(map[string]*string, len(chosen))
	for header, key := range chosen {
		byFold[foldHeading(header)] = key
	}

	picker := headingPicker{chosen: byFold, saved: saved, detected: detected, known: known, used: map[string]bool{}}
	matches := make([]importModels.HeaderMatch, len(file.headers))
	for i, header := range file.headers {
		match := importModels.HeaderMatch{Header: header, Example: example(file, i)}
		key, source := picker.pick(foldHeading(header))
		if key != "" {
			k := key
			match.Key = &k
		}
		if key != "" || source == importModels.HeaderSourceChosen {
			match.Source = source
		}
		matches[i] = match
	}
	return matches
}

// headingPicker resolves one folded heading at a time, remembering the fields
// the headings before it took.
type headingPicker struct {
	chosen   map[string]*string
	saved    map[string]string
	detected map[string][]string
	known    map[string]bool
	used     map[string]bool
}

// pick is the heading's field and where it came from: chosen (a null choice
// ignores it), saved for this file shape, or the first free field it names.
func (p headingPicker) pick(folded string) (string, string) {
	key, source := "", ""
	if choice, ok := p.chosen[folded]; ok {
		source = importModels.HeaderSourceChosen
		if choice != nil && p.known[*choice] {
			key = *choice
		}
	} else if remembered, ok := p.saved[folded]; ok && p.known[remembered] && !p.used[remembered] {
		key, source = remembered, importModels.HeaderSourceSaved
	} else {
		for _, candidate := range p.detected[folded] {
			if !p.used[candidate] {
				key, source = candidate, importModels.HeaderSourceDetected
				break
			}
		}
	}
	if key != "" {
		p.used[key] = true
	}
	return key, source
}

// headingIndex is every folded heading that names one of the descriptor's
// fields — its key, its label in either language, its aliases — with the fields
// it may name in priority order: keys first, then the columns in order. A
// supplier's "Descripción" is the product's name, and its description only when
// a "Nombre" heading took the name.
func headingIndex(descriptor importInterfaces.ImportDescriptor) map[string][]string {
	index := map[string][]string{}
	add := func(heading, key string) {
		folded := foldHeading(heading)
		if folded == "" {
			return
		}
		for _, existing := range index[folded] {
			if existing == key {
				return
			}
		}
		index[folded] = append(index[folded], key)
	}
	resource := descriptor.Resource()
	for _, column := range descriptor.Columns() {
		add(column.Key, column.Key)
	}
	for _, column := range descriptor.Columns() {
		for _, lang := range []string{"es", "en"} {
			add(translateOr(lang, "import."+resource+"."+column.Key, ""), column.Key)
		}
		for _, alias := range column.Aliases {
			add(alias, column.Key)
		}
	}
	return index
}

// suggestions pair each required field no heading filled with an unmatched
// heading that shares a word with its key, label or aliases: "U. medida" for
// "Unidad de medida".
func suggestions(descriptor importInterfaces.ImportDescriptor, lang string, matches []importModels.HeaderMatch) []importModels.HeaderSuggestion {
	filled := map[string]bool{}
	for _, match := range matches {
		if match.Key != nil {
			filled[*match.Key] = true
		}
	}
	out := []importModels.HeaderSuggestion{}
	taken := map[string]bool{}
	for _, column := range labelledColumns(descriptor, lang) {
		if !column.Required || filled[column.Key] {
			continue
		}
		words := significantWords(append([]string{column.Key, column.Label}, column.Aliases...)...)
		for _, match := range matches {
			if match.Key != nil || match.Source == importModels.HeaderSourceChosen || taken[match.Header] {
				continue
			}
			if sharesWord(words, significantWords(match.Header)) {
				out = append(out, importModels.HeaderSuggestion{Key: column.Key, Header: match.Header})
				taken[match.Header] = true
				break
			}
		}
	}
	return out
}

// significantWords are the folded words of three letters or more, the ones
// that can tell a field apart ("de", "la" cannot).
func significantWords(values ...string) map[string]bool {
	words := map[string]bool{}
	for _, value := range values {
		for _, word := range strings.Fields(foldHeading(value)) {
			if len(word) >= 3 {
				words[word] = true
			}
		}
	}
	return words
}

func sharesWord(a, b map[string]bool) bool {
	for word := range b {
		if a[word] {
			return true
		}
	}
	return false
}

// example is the column's first filled cell, what the column step shows beside
// the heading.
func example(file *sheetFile, column int) string {
	for _, record := range file.records {
		if column < len(record) && record[column] != "" {
			return record[column]
		}
	}
	return ""
}

// preview is the file's first rows as written, one cell per heading.
func preview(file *sheetFile) [][]string {
	count := min(len(file.records), importModels.PreviewRowCount)
	rows := make([][]string, count)
	for i := range count {
		row := make([]string, len(file.headers))
		copy(row, file.records[i])
		rows[i] = row
	}
	return rows
}

// headerSignature names a file shape (D3): the sha256 of its folded, non-empty
// headings, sorted, so column order and case do not make a new shape.
func headerSignature(headers []string) string {
	folded := make([]string, 0, len(headers))
	for _, header := range headers {
		if f := foldHeading(header); f != "" {
			folded = append(folded, f)
		}
	}
	sort.Strings(folded)
	sum := sha256.Sum256([]byte(strings.Join(folded, "\n")))
	return hex.EncodeToString(sum[:])
}

// normalizeCells rewrites the typed cells a spreadsheet in a Spanish locale
// writes its own way — `1500,50`, `01/12/2026` — into the one shape every
// descriptor and the page read (IMPORT-001 D4). A cell that still does not
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
