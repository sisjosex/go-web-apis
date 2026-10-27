//go:build integration
// +build integration

package inventory_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/xuri/excelize/v2"

	"josex/web/modules/core/testhelpers"
	importModels "josex/web/modules/import/models"
	importRepos "josex/web/modules/import/repositories"
)

// ============================================================================
// IMPORT-002 — the engine behind the import page, driven with the products file
// ============================================================================

// postImportFile uploads one file with the page's fields and decodes the answer.
func postImportFile(t *testing.T, helper *testhelpers.ApiTestHelper, path, name string, content []byte, fields map[string]string) map[string]interface{} {
	t.Helper()
	w := helper.DoMultipartRequest("POST", path, fields,
		[]testhelpers.MultipartFile{{Field: "file", Filename: name, Content: content}}, map[string]string{})
	if w.Code != http.StatusOK {
		t.Fatalf("%s returned %d: %s", path, w.Code, w.Body.String())
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("decode %s: %v — %s", path, err, w.Body.String())
	}
	return decoded
}

// headingKeys is {heading: field} of a validate answer's mapped headings.
func headingKeys(response map[string]interface{}) map[string]string {
	keys := map[string]string{}
	headers, _ := response["headers"].([]interface{})
	for _, entry := range headers {
		match, _ := entry.(map[string]interface{})
		if key, ok := match["key"].(string); ok {
			keys[fmt.Sprint(match["header"])] = key
		}
	}
	return keys
}

func runOf(response map[string]interface{}) map[string]interface{} {
	run, _ := response["run"].(map[string]interface{})
	return run
}

// supplierWorkbook is a supplier's price list as Excel saves it: its own
// headings, on a sheet named for it.
func supplierWorkbook(t *testing.T, sku string, price float64) []byte {
	t.Helper()
	book := excelize.NewFile()
	defer book.Close()
	sheet := "Lista de precios"
	if err := book.SetSheetName(book.GetSheetName(0), sheet); err != nil {
		t.Fatal(err)
	}
	_ = book.SetSheetRow(sheet, "A1", &[]any{"Cód. Art.", "Descripción", "P. venta"})
	_ = book.SetSheetRow(sheet, "A2", &[]any{sku, "Martillo de carpintero", price})
	buf, err := book.WriteToBuffer()
	if err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// TestImportProducts_WorkbookHeadingsResolveAndMappingIsRemembered - step 1: an
// .xlsx with a supplier's headings validates as products with sku, name and
// price resolved; imported with remember, the same headings get their mapping
// back.
func TestImportProducts_WorkbookHeadingsResolveAndMappingIsRemembered(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()
	sku := importPrefix("XLSX")
	book := supplierWorkbook(t, sku, 12.5)

	dryRun := postImportFile(t, helper, "/import/validate", "lista.xlsx", book, map[string]string{"scope": "inventory"})

	assert.Equal(t, "products", dryRun["resource"], "%v", dryRun)
	assert.Equal(t, "Lista de precios", dryRun["sheet"], "%v", dryRun)
	assert.Equal(t, map[string]string{"Cód. Art.": "sku", "Descripción": "name", "P. venta": "price"}, headingKeys(dryRun))
	assert.Equal(t, float64(1), readyRows(dryRun), "%v", dryRun)

	done := postImportFile(t, helper, "/import", "lista.xlsx", book,
		map[string]string{"resource": "products", "remember": "true"})
	assert.Equal(t, float64(1), runOf(done)["created"], "%v", done)

	headings, _ := json.Marshal([]string{"Cód. Art.", "Descripción", "P. venta"})
	w := helper.DoRequest("GET", "/import/mapping?resource=products&headers="+url.QueryEscape(string(headings)), nil, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var remembered importModels.MappingResponse
	_ = json.Unmarshal(w.Body.Bytes(), &remembered)
	assert.Equal(t, map[string]string{"cod art": "sku", "descripcion": "name", "p venta": "price"}, remembered.Mapping)
}

// TestImportProducts_OnExistingUpdateOrSkip - D4: "Actualizar" writes the
// file's price over a product the shop has; "Dejarlo" leaves it.
func TestImportProducts_OnExistingUpdateOrSkip(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()
	sku := importPrefix("UPD")
	file := func(price string) []byte { return []byte("sku,name,price\n" + sku + ",Tornillo," + price + "\n") }
	priceOf := func() float64 {
		w := helper.DoRequest("GET", "/inventory/products/sku/"+sku, nil, map[string]string{})
		var product struct {
			BasePrice float64 `json:"base_price"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &product)
		return product.BasePrice
	}

	created := postImportFile(t, helper, "/import", "p.csv", file("10"), map[string]string{"resource": "products"})
	assert.Equal(t, float64(1), runOf(created)["created"], "%v", created)

	dryRun := postImportFile(t, helper, "/import/validate", "p.csv", file("99"), map[string]string{"resource": "products"})
	assert.Equal(t, float64(1), dryRun["existing"], "the dry run counts the known code: %v", dryRun)

	updated := postImportFile(t, helper, "/import", "p.csv", file("99"),
		map[string]string{"resource": "products", "on_existing": "update"})
	assert.Equal(t, float64(1), runOf(updated)["updated"], "%v", updated)
	assert.InDelta(t, 99, priceOf(), 0.001)

	kept := postImportFile(t, helper, "/import", "p.csv", file("50"),
		map[string]string{"resource": "products", "on_existing": "skip"})
	assert.Equal(t, float64(0), runOf(kept)["updated"], "%v", kept)
	assert.Equal(t, float64(1), runOf(kept)["skipped"], "%v", kept)
	assert.InDelta(t, 99, priceOf(), 0.001)
}

// TestImportRuns_ListedForTheirTenantOnly - the history shows the business's own
// runs, with the file and who ran it, never another business's.
func TestImportRuns_ListedForTheirTenantOnly(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()
	sku := importPrefix("RUN")
	name := sku + ".csv"

	done := postImportFile(t, helper, "/import", name, []byte("sku,name,price\n"+sku+",Clavo,1\n"), map[string]string{"resource": "products"})
	ownID := fmt.Sprint(runOf(done)["id"])
	foreignID, err := importRepos.NewImportRepository(helper.DB()).RecordRun(context.Background(), uuid.New(),
		importModels.RunMeta{Resource: "products", FileName: "foreign.csv", Total: 1})
	if err != nil {
		t.Fatalf("record a foreign run: %v", err)
	}

	w := helper.DoRequest("GET", "/import/runs?resource=products&page_size=100", nil, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var runs importModels.RunsResponse
	_ = json.Unmarshal(w.Body.Bytes(), &runs)
	listed := map[string]importModels.RunRow{}
	for _, run := range runs.Runs {
		listed[run.ID.String()] = run
	}
	if own, ok := listed[ownID]; assert.True(t, ok, "the business's run is listed: %s", w.Body.String()) {
		assert.Equal(t, name, *own.FileName)
		assert.NotNil(t, own.PerformedByName)
	}
	assert.NotContains(t, listed, foreignID.String(), "another business's run is never listed")
}

// TestImportProducts_FiveThousandRowsValidateUnderTenSeconds - D2: the dry run
// of a 5 000-row file answers in time, with only the counts and the preview.
func TestImportProducts_FiveThousandRowsValidateUnderTenSeconds(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()
	prefix := importPrefix("BIG")
	var file strings.Builder
	file.WriteString("sku,name,price\n")
	for i := range 5000 {
		fmt.Fprintf(&file, "%s-%d,Producto %d,%d\n", prefix, i, i, i+1)
	}

	start := time.Now()
	dryRun := postImportFile(t, helper, "/import/validate", "big.csv", []byte(file.String()), map[string]string{"resource": "products"})
	elapsed := time.Since(start)

	assert.Equal(t, float64(5000), dryRun["total"], "%v", dryRun["total"])
	assert.Equal(t, float64(5000), readyRows(dryRun))
	assert.Len(t, dryRun["preview"], importModels.PreviewRowCount)
	assert.Less(t, elapsed, 10*time.Second, "5 000 rows took %s", elapsed)
}
