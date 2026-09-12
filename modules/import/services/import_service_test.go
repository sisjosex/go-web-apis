package services

import (
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	importErrors "josex/web/modules/import/errors"
	importInterfaces "josex/web/modules/import/interfaces"
	importModels "josex/web/modules/import/models"
)

// parseCSV distinguishes a file with nothing in it from a file whose header is
// present but carries no rows below it. Both used to return ErrFileEmpty, which
// told the operator of a 0-byte upload that it "has no data rows" (A2).

func TestParseCSV_EmptyBytesAreFileEmpty(t *testing.T) {
	_, _, err := parseCSV([]byte(""))

	if !errors.Is(err, importErrors.ErrFileEmpty) {
		t.Fatalf("expected ErrFileEmpty, got %v", err)
	}
}

func TestParseCSV_HeaderWithoutRowsIsNoRows(t *testing.T) {
	_, _, err := parseCSV([]byte("first_name,last_name,email\r\n"))

	if !errors.Is(err, importErrors.ErrFileNoRows) {
		t.Fatalf("expected ErrFileNoRows, got %v", err)
	}
}

func TestParseCSV_BlankLinesAfterHeaderAreNoRows(t *testing.T) {
	_, _, err := parseCSV([]byte("first_name,last_name,email\r\n\r\n\r\n"))

	if !errors.Is(err, importErrors.ErrFileNoRows) {
		t.Fatalf("expected ErrFileNoRows, got %v", err)
	}
}

// IMPORT-001 — a CSV saved by Excel in a Spanish locale arrives with a BOM, `;`
// between cells, a decimal comma and a day-first date, and must validate as
// written.

// spanishExcelFile is that export, with its columns deliberately out of the
// order the descriptor declares them.
var spanishExcelFile = []byte("\xEF\xBB\xBFFecha;SKU;Precio\r\n01/12/2026;A-1;1500,50\r\n7/3/2027;B-2;\"1.234,5\"\r\n")

// excelDescriptor refuses any row whose number or date the engine did not
// normalize, so a clean validate proves the normalization happened upstream.
type excelDescriptor struct{}

func (excelDescriptor) Resource() string { return "excel" }

func (excelDescriptor) Columns() []importModels.ColumnSpec {
	return []importModels.ColumnSpec{
		{Key: "sku", Type: importModels.ColumnTypeText, Required: true},
		{Key: "precio", Type: importModels.ColumnTypeNumber},
		{Key: "fecha", Type: importModels.ColumnTypeDate},
	}
}

func (excelDescriptor) Matches(headers []string) bool { return true }

func (excelDescriptor) ValidateRow(ctx importModels.ImportContext, line int, row map[string]string) importModels.RowResult {
	result := importModels.RowResult{Line: line, Data: row, Status: importModels.RowStatusValid}
	if _, err := strconv.ParseFloat(row["precio"], 64); err != nil {
		result.Status = importModels.RowStatusInvalid
	}
	if _, err := time.Parse("2006-01-02", row["fecha"]); err != nil {
		result.Status = importModels.RowStatusInvalid
	}
	return result
}

func (excelDescriptor) ProcessRow(ctx importModels.ImportContext, line int, row map[string]string) importModels.RowResult {
	return importModels.RowResult{Line: line, Data: row, Status: importModels.RowStatusCreated}
}

func (excelDescriptor) RecordRun(ctx importModels.ImportContext, meta importModels.RunMeta) {}

func TestParseCSV_SpanishExcelExportIsRead(t *testing.T) {
	headers, rows, err := parseCSV(spanishExcelFile)

	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if len(headers) != 3 || headers[0] != "fecha" || headers[1] != "sku" || headers[2] != "precio" {
		t.Fatalf("expected the BOM stripped and `;` split, got %q", headers)
	}
	if len(rows) != 2 || rows[1].data["precio"] != "1.234,5" {
		t.Fatalf("expected two rows with the quoted cell intact, got %v", rows)
	}
}

func TestParseCSV_QuotedSeparatorsDoNotDecideTheDelimiter(t *testing.T) {
	headers, _, err := parseCSV([]byte("\"a;b;c\",d\r\n1,2\r\n"))

	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if len(headers) != 2 || headers[0] != "a;b;c" {
		t.Fatalf("expected a comma-separated header, got %q", headers)
	}
}

func TestNormalizeNumber_ReadsEitherDecimalMark(t *testing.T) {
	cases := map[string]string{
		"1500,50":  "1500.50",
		"1.500,50": "1500.50",
		"1,500.50": "1500.50",
		"1500.50":  "1500.50",
		"12":       "12",
		"abc":      "abc",
		"1,2,3":    "1,2,3",
	}

	for input, expected := range cases {
		if got := normalizeNumber(input); got != expected {
			t.Errorf("normalizeNumber(%q) = %q, expected %q", input, got, expected)
		}
	}
}

func TestNormalizeDate_AcceptsDayFirst(t *testing.T) {
	cases := map[string]string{
		"2026-12-01": "2026-12-01",
		"01/12/2026": "2026-12-01",
		"7/3/2027":   "2027-03-07",
		"31/02/2026": "31/02/2026",
		"mañana":     "mañana",
	}

	for input, expected := range cases {
		if got := normalizeDate(input); got != expected {
			t.Errorf("normalizeDate(%q) = %q, expected %q", input, got, expected)
		}
	}
}

func TestValidate_SpanishExcelExportValidatesCleanInFileOrder(t *testing.T) {
	registry := NewRegistry()
	registry.Register("test", excelDescriptor{})
	service := NewImportService(registry)

	response, err := service.Validate(importModels.ImportContext{}, importModels.ImportTarget{Resource: "excel"}, spanishExcelFile)

	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if response.Valid != 2 || response.Invalid != 0 {
		t.Fatalf("expected 2 valid rows and none invalid, got %+v", response.Rows)
	}
	if strings.Join(response.Headers, ",") != "fecha,sku,precio" {
		t.Errorf("expected headers in file order, got %q", response.Headers)
	}
	if response.Rows[1].Data["precio"] != "1234.5" || response.Rows[1].Data["fecha"] != "2027-03-07" {
		t.Errorf("expected the normalized values handed back, got %v", response.Rows[1].Data)
	}
}

// IMPORT-001 D1 — a scope names resources or modules, and detection never
// reaches past it.

// headerDescriptor claims any file carrying its one header.
type headerDescriptor struct {
	excelDescriptor
	resource string
	header   string
}

func (d headerDescriptor) Resource() string { return d.resource }

func (d headerDescriptor) Matches(headers []string) bool {
	for _, header := range headers {
		if header == d.header {
			return true
		}
	}
	return false
}

func scopedService() importInterfaces.ImportService {
	registry := NewRegistry()
	registry.Register("users", headerDescriptor{resource: "users", header: "email"})
	registry.Register("inventory", headerDescriptor{resource: "products", header: "price"})
	registry.Register("inventory", headerDescriptor{resource: "product_batches", header: "lot_number"})
	return NewImportService(registry)
}

func TestResources_ScopeReachesModulesAndResources(t *testing.T) {
	service := scopedService()

	inventory := service.Resources([]string{"inventory"})
	mixed := service.Resources([]string{"users", "product_batches"})

	if len(inventory) != 2 || inventory[0].Resource != "products" || inventory[1].Module != "inventory" {
		t.Errorf("expected both inventory resources in registration order, got %+v", inventory)
	}
	if len(mixed) != 2 || mixed[0].Resource != "users" || mixed[1].Resource != "product_batches" {
		t.Errorf("expected a module-less mix of resources, got %+v", mixed)
	}
	if len(service.Resources(nil)) != 3 {
		t.Errorf("expected an empty scope to reach everything")
	}
}

func TestValidate_DetectsInsideTheScope(t *testing.T) {
	service := scopedService()

	response, err := service.Validate(importModels.ImportContext{},
		importModels.ImportTarget{Scope: []string{"inventory"}}, []byte("sku,lot_number\r\nA,L-1\r\n"))

	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if response.Resource != "product_batches" {
		t.Errorf("expected product_batches, got %q", response.Resource)
	}
}

func TestValidate_FileOfAnotherScopeIsOutOfScope(t *testing.T) {
	service := scopedService()
	scope := []string{"inventory"}

	_, detected := service.Validate(importModels.ImportContext{},
		importModels.ImportTarget{Scope: scope}, []byte("email\r\nana@example.com\r\n"))
	_, pinned := service.Validate(importModels.ImportContext{},
		importModels.ImportTarget{Resource: "users", Scope: scope}, []byte("email\r\nana@example.com\r\n"))
	_, unknown := service.Validate(importModels.ImportContext{},
		importModels.ImportTarget{Scope: scope}, []byte("nothing\r\nx\r\n"))

	if !errors.Is(detected, importErrors.ErrOutOfScope) || !errors.Is(pinned, importErrors.ErrOutOfScope) {
		t.Errorf("expected ErrOutOfScope for a users file, got %v / %v", detected, pinned)
	}
	if !errors.Is(unknown, importErrors.ErrUnknownFormat) {
		t.Errorf("expected ErrUnknownFormat for a file nothing claims, got %v", unknown)
	}
}

// A1-D1 — detection reads the header line and nothing else, so a file is queued
// before any of it is validated.

func TestDetect_ReadsTheHeaderLineOnly(t *testing.T) {
	service := scopedService()

	response, err := service.Detect(importModels.ImportTarget{Scope: []string{"inventory"}}, []byte("\xEF\xBB\xBFsku;lot_number\r\n"))

	if err != nil {
		t.Fatalf("expected a header-only file to be detected, got %v", err)
	}
	if response.Resource != "product_batches" || response.Module != "inventory" {
		t.Errorf("expected product_batches of inventory, got %+v", response)
	}
}

func TestDetect_OutOfScopeAndEmpty(t *testing.T) {
	service := scopedService()
	scope := importModels.ImportTarget{Scope: []string{"inventory"}}

	_, outOfScope := service.Detect(scope, []byte("email\r\n"))
	_, empty := service.Detect(scope, []byte(""))

	if !errors.Is(outOfScope, importErrors.ErrOutOfScope) {
		t.Errorf("expected ErrOutOfScope, got %v", outOfScope)
	}
	if !errors.Is(empty, importErrors.ErrFileEmpty) {
		t.Errorf("expected ErrFileEmpty, got %v", empty)
	}
}

func TestParseCSV_HeaderAndOneRowSucceeds(t *testing.T) {
	headers, rows, err := parseCSV([]byte("First_Name, email\r\nAna, ana@example.com\r\n"))

	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if len(headers) != 2 || headers[0] != "first_name" || headers[1] != "email" {
		t.Fatalf("expected normalized headers, got %v", headers)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0].line != 2 {
		t.Errorf("expected the first data row on line 2, got %d", rows[0].line)
	}
	if rows[0].data["email"] != "ana@example.com" {
		t.Errorf("expected the value trimmed, got %q", rows[0].data["email"])
	}
}
