package services

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"

	importErrors "josex/web/modules/import/errors"
	importInterfaces "josex/web/modules/import/interfaces"
	importModels "josex/web/modules/import/models"
)

// readFile distinguishes a file with nothing in it from a file whose header is
// present but carries no rows below it. Both used to return ErrFileEmpty, which
// told the operator of a 0-byte upload that it "has no data rows" (A2).

func TestReadFile_EmptyBytesAreFileEmpty(t *testing.T) {
	_, err := readFile([]byte(""), "x.csv", 10)

	if !errors.Is(err, importErrors.ErrFileEmpty) {
		t.Fatalf("expected ErrFileEmpty, got %v", err)
	}
}

func TestValidate_HeaderWithoutRowsIsNoRows(t *testing.T) {
	service := excelService()

	for _, data := range []string{"sku,precio,fecha\r\n", "sku,precio,fecha\r\n\r\n\r\n"} {
		_, err := service.Validate(importModels.ImportContext{}, request("excel", []byte(data)))
		if !errors.Is(err, importErrors.ErrFileNoRows) {
			t.Fatalf("expected ErrFileNoRows for %q, got %v", data, err)
		}
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
		{Key: "sku", Type: importModels.ColumnTypeText, Required: true, Aliases: []string{"Cód. Art."}},
		{Key: "precio", Type: importModels.ColumnTypeNumber, Aliases: []string{"P. venta"}},
		{Key: "fecha", Type: importModels.ColumnTypeDate},
		{Key: "unidad", Type: importModels.ColumnTypeText, Required: true, Aliases: []string{"Unidad de medida"}},
	}
}

func (excelDescriptor) Matches(headers []string) bool { return true }

func (excelDescriptor) ValidateRow(ctx importModels.ImportContext, line int, row map[string]string) importModels.RowResult {
	result := importModels.RowResult{Line: line, Data: row, Status: importModels.RowStatusValid}
	if _, err := strconv.ParseFloat(row["precio"], 64); err != nil {
		result.Status = importModels.RowStatusInvalid
	}
	if row["fecha"] != "" {
		if _, err := time.Parse("2006-01-02", row["fecha"]); err != nil {
			result.Status = importModels.RowStatusInvalid
		}
	}
	return result
}

func (excelDescriptor) ProcessRow(ctx importModels.ImportContext, line int, row map[string]string) importModels.RowResult {
	return importModels.RowResult{Line: line, Data: row, Status: importModels.RowStatusCreated}
}

func (excelDescriptor) RecordRun(ctx importModels.ImportContext, meta importModels.RunMeta) {}

func excelService() importInterfaces.ImportService {
	registry := NewRegistry()
	registry.Register("test", excelDescriptor{})
	return NewImportService(registry, nil)
}

func request(resource string, data []byte) importModels.ImportRequest {
	return importModels.ImportRequest{Target: importModels.ImportTarget{Resource: resource}, Data: data, FileName: "file.csv"}
}

func TestReadFile_SpanishExcelExportIsRead(t *testing.T) {
	file, err := readFile(spanishExcelFile, "precios.csv", 10)

	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if len(file.headers) != 3 || file.headers[0] != "Fecha" || file.headers[1] != "SKU" || file.headers[2] != "Precio" {
		t.Fatalf("expected the BOM stripped and `;` split, got %q", file.headers)
	}
	if len(file.records) != 2 || file.records[1][2] != "1.234,5" {
		t.Fatalf("expected two rows with the quoted cell intact, got %v", file.records)
	}
}

func TestReadFile_QuotedSeparatorsDoNotDecideTheDelimiter(t *testing.T) {
	file, err := readFile([]byte("\"a;b;c\",d\r\n1,2\r\n"), "x.csv", 10)

	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if len(file.headers) != 2 || file.headers[0] != "a;b;c" {
		t.Fatalf("expected a comma-separated header, got %q", file.headers)
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

func TestValidate_SpanishExcelExportValidatesCleanAndPreviewsAsWritten(t *testing.T) {
	response, err := excelService().Validate(importModels.ImportContext{}, request("excel", spanishExcelFile))

	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if response.Valid != 2 || response.Invalid != 0 || len(response.Issues) != 0 {
		t.Fatalf("expected 2 valid rows and no issue, got %+v", response)
	}
	if len(response.Headers) != 3 || response.Headers[0].Header != "Fecha" || *response.Headers[0].Key != "fecha" {
		t.Errorf("expected the headings in file order, matched to their fields, got %+v", response.Headers)
	}
	if len(response.Preview) != 2 || response.Preview[1][2] != "1.234,5" {
		t.Errorf("expected the preview as written, got %v", response.Preview)
	}
}

// IMPORT-002 — headings a business writes its own way: aliases, a heading the
// operator maps by hand, an ignored one, and a missing required field offered
// the heading that looks like it.

func TestValidate_AliasesAndSuggestions(t *testing.T) {
	data := []byte("Cód. Art.;Descripción;P. venta;U. medida\r\nA-1;Martillo;12,50;pza\r\n")

	response, err := excelService().Validate(importModels.ImportContext{}, request("excel", data))

	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	keys := map[string]string{}
	for _, match := range response.Headers {
		if match.Key != nil {
			keys[match.Header] = *match.Key + "/" + match.Source
		}
	}
	if keys["Cód. Art."] != "sku/detected" || keys["P. venta"] != "precio/detected" {
		t.Errorf("expected the aliases detected, got %v", keys)
	}
	if _, mapped := keys["U. medida"]; mapped {
		t.Errorf("expected «U. medida» not taken without the operator, got %v", keys)
	}
	if len(response.Suggestions) != 1 || response.Suggestions[0].Key != "unidad" || response.Suggestions[0].Header != "U. medida" {
		t.Errorf("expected «U. medida» offered for unidad, got %+v", response.Suggestions)
	}
}

func TestValidate_ChosenMappingWinsAndNullIgnores(t *testing.T) {
	unidad := "unidad"
	req := request("excel", []byte("Cód. Art.;P. venta;U. medida;Notas\r\nA-1;12;pza;x\r\n"))
	req.Mapping = map[string]*string{"U. medida": &unidad, "Notas": nil}

	response, err := excelService().Validate(importModels.ImportContext{}, req)

	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if got := response.Headers[2]; got.Key == nil || *got.Key != "unidad" || got.Source != importModels.HeaderSourceChosen {
		t.Errorf("expected «U. medida» chosen for unidad, got %+v", got)
	}
	if got := response.Headers[3]; got.Key != nil || got.Source != importModels.HeaderSourceChosen {
		t.Errorf("expected «Notas» ignored by choice, got %+v", got)
	}
	if len(response.Suggestions) != 0 {
		t.Errorf("expected nothing left to suggest, got %+v", response.Suggestions)
	}
}

func TestFoldHeading(t *testing.T) {
	cases := map[string]string{"Cód. Art.": "cod art", "first_name": "first name", "  P.VENTA ": "p venta", "Año": "ano"}
	for input, expected := range cases {
		if got := foldHeading(input); got != expected {
			t.Errorf("foldHeading(%q) = %q, expected %q", input, got, expected)
		}
	}
	if headerSignature([]string{"SKU", "Precio"}) != headerSignature([]string{"precio", "sku", ""}) {
		t.Errorf("expected the signature blind to order, case and blank headings")
	}
}

// D1 — an Excel workbook reads like the CSV saved from it: the first sheet with
// a filled cell, its name answered.

func TestValidate_WorkbookReadsFirstFilledSheet(t *testing.T) {
	book := excelize.NewFile()
	defer func() { _ = book.Close() }()
	if _, err := book.NewSheet("Precios"); err != nil {
		t.Fatal(err)
	}
	_ = book.SetSheetRow("Precios", "A1", &[]any{"Cód. Art.", "Descripción", "P. venta"})
	_ = book.SetSheetRow("Precios", "A2", &[]any{"A-1", "Martillo", 12.5})
	buf, err := book.WriteToBuffer()
	if err != nil {
		t.Fatal(err)
	}

	response, err := excelService().Validate(importModels.ImportContext{},
		importModels.ImportRequest{Target: importModels.ImportTarget{Resource: "excel"}, Data: buf.Bytes(), FileName: "lista.xlsx"})

	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if response.Sheet != "Precios" || response.Total != 1 || response.Valid != 1 {
		t.Errorf("expected the Precios sheet's one valid row, got %+v", response)
	}
}

// D4 — "Actualizar" hands the rows a descriptor skipped as existing to its
// Updater; "Dejarlo" leaves them skipped. A row dropped on the review step is
// skipped either way, and the run is recorded.

type updatableDescriptor struct{ excelDescriptor }

func (updatableDescriptor) ProcessRow(ctx importModels.ImportContext, line int, row map[string]string) importModels.RowResult {
	if row["sku"] == "OLD" {
		return importModels.RowResult{Line: line, Data: row, Status: importModels.RowStatusSkipped}
	}
	return importModels.RowResult{Line: line, Data: row, Status: importModels.RowStatusCreated}
}

func (updatableDescriptor) KeyColumns() []string { return []string{"sku"} }

func (updatableDescriptor) UpdateRow(ctx importModels.ImportContext, line int, row map[string]string) importModels.RowResult {
	return importModels.RowResult{Line: line, Data: row, Status: importModels.RowStatusUpdated}
}

type memoryRepo struct {
	runs     []importModels.RunMeta
	mappings map[string]map[string]string
}

func (m *memoryRepo) GetMapping(_ context.Context, _ uuid.UUID, resource, signature string) (map[string]string, error) {
	return m.mappings[resource+signature], nil
}

func (m *memoryRepo) UpsertMapping(_ context.Context, _ uuid.UUID, resource, signature string, mapping map[string]string) error {
	m.mappings[resource+signature] = mapping
	return nil
}

func (m *memoryRepo) RecordRun(_ context.Context, _ uuid.UUID, meta importModels.RunMeta) (uuid.UUID, error) {
	m.runs = append(m.runs, meta)
	return uuid.New(), nil
}

func (m *memoryRepo) ListRuns(context.Context, uuid.UUID, importModels.RunsQuery) (*importModels.RunsResponse, error) {
	return &importModels.RunsResponse{}, nil
}

func (m *memoryRepo) LastRuns(context.Context, uuid.UUID) (map[string]time.Time, error) {
	return nil, nil
}

func TestImport_OnExistingUpdateOrSkip(t *testing.T) {
	registry := NewRegistry()
	registry.Register("test", updatableDescriptor{})
	repo := &memoryRepo{mappings: map[string]map[string]string{}}
	service := NewImportService(registry, repo)
	ctx := importModels.ImportContext{TenantID: uuid.New()}
	data := []byte("sku,precio\r\nNEW,1\r\nOLD,2\r\nDROP,3\r\n")

	for onExisting, expected := range map[string][3]int{"update": {1, 1, 1}, "skip": {1, 0, 2}} {
		req := request("excel", data)
		req.OnExisting = onExisting
		req.Edits = map[int]map[string]string{4: nil}

		response, err := service.Import(ctx, req)

		if err != nil {
			t.Fatalf("%s: expected success, got %v", onExisting, err)
		}
		run := response.Run
		if run.Created != expected[0] || run.Updated != expected[1] || run.Skipped != expected[2] || run.ID == nil {
			t.Errorf("%s: expected created/updated/skipped %v and a recorded id, got %+v", onExisting, expected, run)
		}
		if len(response.FailedRows) != expected[2] {
			t.Errorf("%s: expected the skipped rows handed back, got %+v", onExisting, response.FailedRows)
		}
	}
	if len(repo.runs) != 2 {
		t.Errorf("expected both runs recorded, got %d", len(repo.runs))
	}
}

// D3 — a mapping remembered on import maps the next file of the same shape
// without the operator, whatever its column order.

func TestImport_RememberedMappingMapsTheNextFile(t *testing.T) {
	repo := &memoryRepo{mappings: map[string]map[string]string{}}
	registry := NewRegistry()
	registry.Register("test", excelDescriptor{})
	service := NewImportService(registry, repo)
	ctx := importModels.ImportContext{TenantID: uuid.New()}
	unidad := "unidad"

	req := request("excel", []byte("Código interno;U. medida;P. venta\r\nA-1;pza;3\r\n"))
	sku := "sku"
	req.Mapping = map[string]*string{"Código interno": &sku, "U. medida": &unidad}
	req.Remember = true
	if _, err := service.Import(ctx, req); err != nil {
		t.Fatalf("expected success, got %v", err)
	}

	response, err := service.Validate(ctx, request("excel", []byte("P. venta;U. medida;Código interno\r\n3;pza;A-1\r\n")))

	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if got := response.Headers[2]; got.Key == nil || *got.Key != "sku" || got.Source != importModels.HeaderSourceSaved {
		t.Errorf("expected «Código interno» remembered as sku, got %+v", got)
	}
	if got := response.Headers[1]; got.Key == nil || *got.Key != "unidad" || got.Source != importModels.HeaderSourceSaved {
		t.Errorf("expected «U. medida» remembered as unidad, got %+v", got)
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
	return NewImportService(registry, nil)
}

func TestResources_ScopeReachesModulesAndResources(t *testing.T) {
	service := scopedService()
	ctx := context.Background()

	inventory := service.Resources(ctx, uuid.Nil, "es", []string{"inventory"})
	mixed := service.Resources(ctx, uuid.Nil, "es", []string{"users", "product_batches"})

	if len(inventory) != 2 || inventory[0].Resource != "products" || inventory[1].Module != "inventory" {
		t.Errorf("expected both inventory resources in registration order, got %+v", inventory)
	}
	if len(mixed) != 2 || mixed[0].Resource != "users" || mixed[1].Resource != "product_batches" {
		t.Errorf("expected a module-less mix of resources, got %+v", mixed)
	}
	if len(service.Resources(ctx, uuid.Nil, "es", nil)) != 3 {
		t.Errorf("expected an empty scope to reach everything")
	}
}

func scoped(scope []string, data string) importModels.ImportRequest {
	return importModels.ImportRequest{Target: importModels.ImportTarget{Scope: scope}, Data: []byte(data), FileName: "f.csv"}
}

func TestValidate_DetectsInsideTheScope(t *testing.T) {
	response, err := scopedService().Validate(importModels.ImportContext{}, scoped([]string{"inventory"}, "sku,lot_number\r\nA,L-1\r\n"))

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

	_, detected := service.Validate(importModels.ImportContext{}, scoped(scope, "email\r\nana@example.com\r\n"))
	pinnedReq := scoped(scope, "email\r\nana@example.com\r\n")
	pinnedReq.Target.Resource = "users"
	_, pinned := service.Validate(importModels.ImportContext{}, pinnedReq)
	_, unknown := service.Validate(importModels.ImportContext{}, scoped(scope, "nothing\r\nx\r\n"))

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
	response, err := scopedService().Detect(importModels.ImportTarget{Scope: []string{"inventory"}}, []byte("\xEF\xBB\xBFsku;lot_number\r\n"), "f.csv")

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

	_, outOfScope := service.Detect(scope, []byte("email\r\n"), "f.csv")
	_, empty := service.Detect(scope, []byte(""), "f.csv")

	if !errors.Is(outOfScope, importErrors.ErrOutOfScope) {
		t.Errorf("expected ErrOutOfScope, got %v", outOfScope)
	}
	if !errors.Is(empty, importErrors.ErrFileEmpty) {
		t.Errorf("expected ErrFileEmpty, got %v", empty)
	}
}

func TestValidate_HeaderAndOneRowKeepsLineAndTrims(t *testing.T) {
	response, err := scopedService().Validate(importModels.ImportContext{}, scoped(nil, "Email, Notes\r\n ana@example.com , x\r\n"))

	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if response.Total != 1 || response.Preview[0][0] != "ana@example.com" {
		t.Errorf("expected one trimmed row, got %+v", response)
	}
}
