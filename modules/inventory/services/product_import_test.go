package services

import (
	"strings"
	"testing"

	importModels "josex/web/modules/import/models"
	inventoryConfig "josex/web/modules/inventory/config"
	inventoryErrors "josex/web/modules/inventory/errors"
)

// newProductsDescriptor builds the descriptor with no dependency but its config:
// every case below is a white-box test of the row reading, the ledger and the
// price attribution, and the parts that need a database (the duplicate preview,
// category resolution, the combination SP) are covered by the integration tests
// in modules/inventory/tests.
func newProductsDescriptor() *productsImportDescriptor {
	return &productsImportDescriptor{
		config: &inventoryConfig.InventoryConfig{MaxAxes: 3, MaxCombinations: 100},
	}
}

// catalogueContext is one run: the file's columns in order and the empty ledger
// the engine hands every descriptor.
func catalogueContext(headers []string, images map[string][]byte) importModels.ImportContext {
	return importModels.ImportContext{
		Headers: headers,
		Images:  importModels.ImportImages(images),
		Scratch: map[string]any{},
	}
}

// catalogueHeaders is the sample file of D9: two axes, a price, and the columns
// a row uses to seed its own stock.
func catalogueHeaders() []string {
	return []string{
		"sku", "name", "description", "price", "category",
		"variant[talle]", "variant[color]", "variant_sku", "stock", "reorder_level",
	}
}

// readAll walks a whole file the way the engine does — one row at a time,
// against one ledger — and returns what each row produced.
func readAll(t *testing.T, rows []map[string]string) ([]*catalogueRow, [][]string) {
	t.Helper()

	parsedRows, codes, _ := readAllVerbose(t, rows)
	return parsedRows, codes
}

// readAllVerbose is readAll with the warnings kept, for the codes a row reports
// without being refused.
func readAllVerbose(t *testing.T, rows []map[string]string) ([]*catalogueRow, [][]string, [][]string) {
	t.Helper()

	descriptor := newProductsDescriptor()
	ctx := catalogueContext(catalogueHeaders(), nil)

	parsedRows := make([]*catalogueRow, 0, len(rows))
	codes := make([][]string, 0, len(rows))
	warnings := make([][]string, 0, len(rows))
	for _, row := range rows {
		parsed, fieldErrors, rowWarnings := descriptor.readRow(ctx, row, false)
		if len(fieldErrors) == 0 {
			descriptor.remember(parsed)
		}
		parsedRows = append(parsedRows, parsed)
		codes = append(codes, fieldErrors)
		warnings = append(warnings, rowWarnings)
	}
	return parsedRows, codes, warnings
}

// ---------------------------------------------------------------------------
// The header signature and the published columns (D6, AC-14)
// ---------------------------------------------------------------------------

func TestProductsMatches_NeedsSkuNameAndPrice(t *testing.T) {
	descriptor := newProductsDescriptor()

	if !descriptor.Matches([]string{"sku", "name", "price", "variant[talle]"}) {
		t.Error("a catalogue header row should match")
	}
	// D6 — sku and the variant[…] columns are shared by the three product
	// files, so each is separated by one column of its own.
	if descriptor.Matches([]string{"sku", "variant[talle]", "quantity", "reorder_level"}) {
		t.Error("a product_stock header row must not match the catalogue")
	}
	if descriptor.Matches([]string{"sku", "variant[talle]", "lot_number", "unit_cost"}) {
		t.Error("a product_batches header row must not match the catalogue")
	}
	if descriptor.Matches([]string{"name", "parent", "description"}) {
		t.Error("a categories header row must not match the catalogue")
	}
	// D3 retired base_price, so the old signature is gone with it.
	if descriptor.Matches([]string{"sku", "name", "base_price"}) {
		t.Error("base_price is no longer a catalogue column")
	}
}

func TestProductsColumns_PublishTheMarkerAndRetireTheCellGrammar(t *testing.T) {
	// A1.1 — only sku is required per cell. name and price are enforced on a
	// product's first row (readRow), which the flag cannot express: the client
	// applies it to every cell, and D9 makes the empty repeat row normal.
	required := map[string]bool{"sku": true}
	published := map[string]bool{}

	for _, column := range newProductsDescriptor().Columns() {
		published[column.Key] = true
		if column.Required != required[column.Key] {
			t.Errorf("%s: required = %v, expected %v", column.Key, column.Required, required[column.Key])
		}
	}

	// AC-14 — the template carries two sample variant[…] columns so the marker
	// is visible without reading the docs.
	for _, key := range []string{"variant[talle]", "variant[color]", "image[color]", "image_alt[color]"} {
		if !published[key] {
			t.Errorf("the template should publish %q", key)
		}
	}
	// AC-13 — no descriptor publishes a packed cell any more.
	for _, key := range []string{"axes", "combination", "base_price", "initial_stock"} {
		if published[key] {
			t.Errorf("%q should not be published any more", key)
		}
	}
}

// ---------------------------------------------------------------------------
// One row per sellable thing (D9)
// ---------------------------------------------------------------------------

func TestReadRow_SimpleProductAndCombinationsInOneFile(t *testing.T) {
	rows, codes := readAll(t, []map[string]string{
		{"sku": "YERBA-1K", "name": "Yerba 1kg", "price": "4200", "stock": "12"},
		{"sku": "REMERA-BAS", "name": "Remera Basica", "price": "7500", "variant[talle]": "M", "variant[color]": "Negro", "stock": "10"},
		{"sku": "REMERA-BAS", "variant[talle]": "L", "variant[color]": "Negro", "stock": "8"},
	})

	for line, code := range codes {
		if len(code) > 0 {
			t.Fatalf("row %d should be readable, got %v", line, code)
		}
	}

	if len(rows[0].Pairs) != 0 || !rows[0].IsFirst {
		t.Errorf("a row with no variant cell is a simple product, got %+v", rows[0])
	}
	if len(rows[1].Pairs) != 2 || !rows[1].IsFirst {
		t.Errorf("the first REMERA-BAS row declares the product, got %+v", rows[1])
	}
	// D9 — a repeated sku is normal, not a duplicate, and the discriminator
	// INV-015 A1 needed does not exist: this row fills neither name nor price.
	if rows[2].IsFirst {
		t.Error("a repeat row attaches to the product above it")
	}
	if rows[1].Product != rows[2].Product {
		t.Error("both rows belong to the same product")
	}
	if rows[1].Key == rows[2].Key {
		t.Error("M/Negro and L/Negro are different combinations")
	}
}

// D9 — identical and empty both mean "same product"; a differing non-empty
// value is the one thing that cannot be meant (AC-2b).
func TestReadRow_RepeatRowCellsMayRepeatOrBeBlank(t *testing.T) {
	_, codes := readAll(t, []map[string]string{
		{"sku": "REMERA-BAS", "name": "Remera Basica", "price": "7500", "variant[talle]": "M", "variant[color]": "Negro"},
		{"sku": "REMERA-BAS", "name": "Remera Basica", "variant[talle]": "L", "variant[color]": "Negro"},
		{"sku": "REMERA-BAS", "variant[talle]": "XL", "variant[color]": "Negro"},
	})

	for line, code := range codes {
		if len(code) > 0 {
			t.Errorf("row %d should be readable, got %v", line, code)
		}
	}
}

func TestReadRow_DisagreeingProductCellIsInconsistent(t *testing.T) {
	_, codes := readAll(t, []map[string]string{
		{"sku": "REMERA-BAS", "name": "Remera Basica", "price": "7500", "variant[talle]": "M", "variant[color]": "Negro"},
		{"sku": "REMERA-BAS", "name": "Remera Premium", "variant[talle]": "L", "variant[color]": "Negro"},
	})

	if len(codes[1]) != 1 || !strings.HasPrefix(codes[1][0], inventoryErrors.ImportProductInconsistent) {
		t.Fatalf("a differing name is product-inconsistent, got %v", codes[1])
	}
	// The message names the column it came from.
	if !strings.HasSuffix(codes[1][0], errorParamSeparator+"name") {
		t.Errorf("the code should quote the column, got %q", codes[1][0])
	}
}

// AC-6 — a product whose rows fill different axis columns is refused.
func TestReadRow_RaggedAxisSetIsInconsistent(t *testing.T) {
	_, codes := readAll(t, []map[string]string{
		{"sku": "REMERA-BAS", "name": "Remera Basica", "price": "7500", "variant[talle]": "M", "variant[color]": "Negro"},
		{"sku": "REMERA-BAS", "variant[talle]": "L"},
	})

	if len(codes[1]) != 1 || codes[1][0] != inventoryErrors.ImportAxesInconsistent {
		t.Fatalf("a row filling fewer axes is axes-inconsistent, got %v", codes[1])
	}
}

// AC-6 — a cell holding only spaces counts as empty, so this is one axis and
// not two, on both rows.
func TestReadRow_ASpacesOnlyCellIsEmpty(t *testing.T) {
	rows, codes := readAll(t, []map[string]string{
		{"sku": "COLA", "name": "Gaseosa Cola", "price": "2200", "variant[talle]": "   ", "variant[color]": "  "},
		{"sku": "COLA", "variant[talle]": "", "variant[color]": ""},
	})

	for line, code := range codes {
		if len(code) > 0 {
			t.Fatalf("row %d should be readable, got %v", line, code)
		}
	}
	if len(rows[0].Pairs) != 0 {
		t.Errorf("a spaces-only cell declares no axis, got %+v", rows[0].Pairs)
	}
}

// D9 — the identity is (sku, option set), so two rows naming the same
// combination collide and two rows naming the same simple product do too.
func TestReadRow_TheSameCombinationTwiceIsADuplicate(t *testing.T) {
	descriptor := newProductsDescriptor()
	ctx := catalogueContext(catalogueHeaders(), nil)

	first, _, _ := descriptor.readRow(ctx, map[string]string{
		"sku": "REMERA-BAS", "name": "Remera Basica", "price": "7500",
		"variant[talle]": "M", "variant[color]": "Negro",
	}, false)
	descriptor.remember(first)

	// The same combination written with different casing is the same one.
	second, fieldErrors, _ := descriptor.readRow(ctx, map[string]string{
		"sku": "remera-bas", "variant[talle]": "m", "variant[color]": "negro",
	}, false)
	if len(fieldErrors) > 0 {
		t.Fatalf("the row is readable, it is merely a duplicate, got %v", fieldErrors)
	}
	if !second.Product.Combinations[second.Key] {
		t.Error("the second row names a combination the file already carries")
	}
}

// ---------------------------------------------------------------------------
// The axis marker (D1)
// ---------------------------------------------------------------------------

func TestReadRow_MalformedMarkerAndTooManyAxes(t *testing.T) {
	descriptor := newProductsDescriptor()

	ctx := catalogueContext([]string{"sku", "name", "price", "variant[]"}, nil)
	_, fieldErrors, _ := descriptor.readRow(ctx, map[string]string{
		"sku": "COLA", "name": "Gaseosa Cola", "price": "2200",
	}, false)
	if len(fieldErrors) != 1 || !strings.HasPrefix(fieldErrors[0], inventoryErrors.ImportAxisColumnInvalid) {
		t.Fatalf("variant[] is axis-column-invalid, got %v", fieldErrors)
	}

	// AC-9 — more axis columns than INVENTORY_MAX_AXES allows.
	ctx = catalogueContext([]string{
		"sku", "name", "price",
		"variant[a]", "variant[b]", "variant[c]", "variant[d]",
	}, nil)
	_, fieldErrors, _ = descriptor.readRow(ctx, map[string]string{
		"sku": "COLA", "name": "Gaseosa Cola", "price": "2200",
		"variant[a]": "1", "variant[b]": "2", "variant[c]": "3", "variant[d]": "4",
	}, false)
	if len(fieldErrors) != 1 || fieldErrors[0] != inventoryErrors.ImportAxesTooMany {
		t.Fatalf("a fourth axis is axes-too-many, got %v", fieldErrors)
	}
}

// AC-9 — a variant[voltaje] column works with no code change, and a stray
// column is ignored without touching the row's outcome.
func TestReadRow_AnyAxisNameWorksAndStrayColumnsAreIgnored(t *testing.T) {
	descriptor := newProductsDescriptor()
	ctx := catalogueContext([]string{"sku", "name", "price", "variant[voltaje]", "notas"}, nil)

	parsed, fieldErrors, _ := descriptor.readRow(ctx, map[string]string{
		"sku": "TALADRO", "name": "Taladro", "price": "45000",
		"variant[voltaje]": "220V", "notas": "liquidacion",
	}, false)

	if len(fieldErrors) > 0 {
		t.Fatalf("unexpected errors %v", fieldErrors)
	}
	if len(parsed.Pairs) != 1 || parsed.Pairs[0].Axis != "Voltaje" || parsed.Pairs[0].Option != "220V" {
		t.Errorf("variant[voltaje] is the axis Voltaje, got %+v", parsed.Pairs)
	}
}

// ---------------------------------------------------------------------------
// The required cells and the optional code (D3, D6, D8)
// ---------------------------------------------------------------------------

func TestReadRow_FirstRowNeedsANameAndAPrice(t *testing.T) {
	descriptor := newProductsDescriptor()
	ctx := catalogueContext(catalogueHeaders(), nil)

	_, fieldErrors, _ := descriptor.readRow(ctx, map[string]string{"sku": "COLA"}, false)

	if !slicesContain(fieldErrors, inventoryErrors.ImportNameRequired) {
		t.Errorf("a first row needs a name, got %v", fieldErrors)
	}
	if !slicesContain(fieldErrors, inventoryErrors.ImportPriceInvalid) {
		t.Errorf("a first row needs a price, got %v", fieldErrors)
	}

	_, fieldErrors, _ = descriptor.readRow(ctx, map[string]string{"name": "Gaseosa"}, false)
	if len(fieldErrors) != 1 || fieldErrors[0] != inventoryErrors.ImportProductSkuRequired {
		t.Errorf("a row with no sku is product-sku-required, got %v", fieldErrors)
	}
}

func TestReadRow_PriceMustBeANumber(t *testing.T) {
	descriptor := newProductsDescriptor()
	ctx := catalogueContext(catalogueHeaders(), nil)

	for _, cell := range []string{"gratis", "-100"} {
		_, fieldErrors, _ := descriptor.readRow(ctx, map[string]string{
			"sku": "COLA", "name": "Gaseosa Cola", "price": cell,
		}, false)
		if !slicesContain(fieldErrors, inventoryErrors.ImportPriceInvalid) {
			t.Errorf("%q is not a price, got %v", cell, fieldErrors)
		}
	}
}

// AC-8 / D8 — a filled code is taken verbatim; only the two failures the row
// cannot recover from are checked, and there is no prefix rule.
func TestReadRow_VariantSkuIsVerbatimAndOnlyLengthAndClashesAreChecked(t *testing.T) {
	descriptor := newProductsDescriptor()
	ctx := catalogueContext(catalogueHeaders(), nil)

	// A legacy code sharing nothing with the product's SKU is accepted.
	parsed, fieldErrors, _ := descriptor.readRow(ctx, map[string]string{
		"sku": "REMERA-BAS", "name": "Remera Basica", "price": "7500",
		"variant[talle]": "M", "variant[color]": "Negro", "variant_sku": "7791234567890",
	}, false)
	if len(fieldErrors) > 0 {
		t.Fatalf("a legacy code is not an error, got %v", fieldErrors)
	}
	if parsed.VariantSku == nil || *parsed.VariantSku != "7791234567890" {
		t.Fatalf("the code travels verbatim, got %v", parsed.VariantSku)
	}
	descriptor.remember(parsed)

	// The same code twice in one file.
	_, fieldErrors, _ = descriptor.readRow(ctx, map[string]string{
		"sku": "REMERA-BAS", "variant[talle]": "L", "variant[color]": "Negro",
		"variant_sku": "7791234567890",
	}, false)
	if !slicesContain(fieldErrors, inventoryErrors.ImportVariantSkuDuplicate) {
		t.Errorf("a repeated code is variant-sku-duplicate, got %v", fieldErrors)
	}

	// Longer than product_skus.sku holds.
	_, fieldErrors, _ = descriptor.readRow(ctx, map[string]string{
		"sku": "REMERA-BAS", "variant[talle]": "XL", "variant[color]": "Negro",
		"variant_sku": strings.Repeat("X", skuMaxLength+1),
	}, false)
	if !slicesContain(fieldErrors, inventoryErrors.ImportVariantSkuTooLong) {
		t.Errorf("an over-long code is variant-sku-too-long, got %v", fieldErrors)
	}
}

// ---------------------------------------------------------------------------
// The price ledger (D3)
// ---------------------------------------------------------------------------

// AC-4 — the sample of D3: 800 on 1.5L, 1200 on XL and 0 on everything else.
func TestAttributePrice_TheWorkedExampleOfD3(t *testing.T) {
	rows, codes := readAll(t, []map[string]string{
		{"sku": "COLA", "name": "Gaseosa Cola", "price": "2200", "variant[talle]": "", "variant[color]": ""},
	})
	if len(codes[0]) > 0 {
		t.Fatalf("unexpected errors %v", codes[0])
	}
	if len(rows[0].Modifiers) != 0 {
		t.Errorf("a product with no axis has no modifier, got %v", rows[0].Modifiers)
	}

	// One axis: unambiguous, whatever the column order.
	rows, codes = readAll(t, []map[string]string{
		{"sku": "COLA", "name": "Gaseosa Cola", "price": "2200", "variant[talle]": "500ml"},
		{"sku": "COLA", "variant[talle]": "1.5L", "price": "3000"},
	})
	for line, code := range codes {
		if len(code) > 0 {
			t.Fatalf("row %d unexpected errors %v", line, code)
		}
	}
	if rows[0].Modifiers[0] != 0 {
		t.Errorf("the first row is the base, got %v", rows[0].Modifiers)
	}
	if rows[1].Modifiers[0] != 800 {
		t.Errorf("1.5L should carry 800, got %v", rows[1].Modifiers)
	}

	// Two axes: left to right, so the difference lands on Talle and Color is
	// fixed at 0.
	rows, codes = readAll(t, []map[string]string{
		{"sku": "REMERA-BAS", "name": "Remera Basica", "price": "7500", "variant[talle]": "M", "variant[color]": "Negro"},
		{"sku": "REMERA-BAS", "variant[talle]": "L", "variant[color]": "Negro", "price": "7500"},
		{"sku": "REMERA-BAS", "variant[talle]": "XL", "variant[color]": "Blanco", "price": "8700"},
	})
	for line, code := range codes {
		if len(code) > 0 {
			t.Fatalf("row %d unexpected errors %v", line, code)
		}
	}
	if rows[2].Modifiers[0] != 1200 || rows[2].Modifiers[1] != 0 {
		t.Errorf("XL carries the 1200 and Blanco 0, got %v", rows[2].Modifiers)
	}
	if rows[1].Modifiers[0] != 0 {
		t.Errorf("L is fixed at 0, got %v", rows[1].Modifiers)
	}
}

// A1-D1 — the attribution is derived, so the row says where it landed. One axis
// and two axes both report, and a row that costs what its product costs stays
// silent.
func TestAttributePrice_ReportsWhereTheDifferenceLanded(t *testing.T) {
	_, codes, warnings := readAllVerbose(t, []map[string]string{
		{"sku": "COLA", "name": "Gaseosa Cola", "price": "2200", "variant[talle]": "500ml"},
		{"sku": "COLA", "variant[talle]": "1.5L", "price": "3000"},
	})
	for line, code := range codes {
		if len(code) > 0 {
			t.Fatalf("row %d unexpected errors %v", line, code)
		}
	}
	// The base row attributes nothing, so it says nothing.
	if slicesContain(warnings[0], inventoryErrors.ImportPriceAttributed) {
		t.Errorf("the base row has nothing to attribute, got %v", warnings[0])
	}
	expected := inventoryErrors.ImportPriceAttributed + errorParamSeparator + "1.5L +800"
	if !slicesContain(warnings[1], expected) {
		t.Errorf("expected %q, got %v", expected, warnings[1])
	}

	// Two axes: the difference goes to the leftmost unfixed option, and that is
	// exactly the choice the operator cannot see in the file.
	_, codes, warnings = readAllVerbose(t, []map[string]string{
		{"sku": "REMERA-BAS", "name": "Remera Basica", "price": "7500", "variant[talle]": "M", "variant[color]": "Negro"},
		{"sku": "REMERA-BAS", "variant[talle]": "L", "variant[color]": "Negro", "price": "7500"},
		{"sku": "REMERA-BAS", "variant[talle]": "XL", "variant[color]": "Blanco", "price": "8700"},
	})
	for line, code := range codes {
		if len(code) > 0 {
			t.Fatalf("row %d unexpected errors %v", line, code)
		}
	}
	expected = inventoryErrors.ImportPriceAttributed + errorParamSeparator + "XL +1200"
	if !slicesContain(warnings[2], expected) {
		t.Errorf("expected %q, got %v", expected, warnings[2])
	}
	// L costs what M costs: nothing was attributed, so nothing is reported.
	if slicesContain(warnings[1], inventoryErrors.ImportPriceAttributed) {
		t.Errorf("a row at the base price is silent, got %v", warnings[1])
	}
}

// A cheaper variant is legal (D3's note on the base) and reads as a subtraction.
func TestAttributePrice_ANegativeAttributionKeepsItsSign(t *testing.T) {
	_, codes, warnings := readAllVerbose(t, []map[string]string{
		{"sku": "COLA", "name": "Gaseosa Cola", "price": "3000", "variant[talle]": "1.5L"},
		{"sku": "COLA", "variant[talle]": "500ml", "price": "2200"},
	})
	if len(codes[1]) > 0 {
		t.Fatalf("unexpected errors %v", codes[1])
	}
	expected := inventoryErrors.ImportPriceAttributed + errorParamSeparator + "500ml -800"
	if !slicesContain(warnings[1], expected) {
		t.Errorf("expected %q, got %v", expected, warnings[1])
	}
}

// AC-5 — a row whose options are all already fixed and whose modifiers do not
// sum to its difference is refused, quoting the cell, and writes nothing.
func TestAttributePrice_ARowThatCannotBeReconciledIsRefused(t *testing.T) {
	rows, codes := readAll(t, []map[string]string{
		{"sku": "REMERA-BAS", "name": "Remera Basica", "price": "7500", "variant[talle]": "M", "variant[color]": "Negro"},
		{"sku": "REMERA-BAS", "variant[talle]": "XL", "variant[color]": "Blanco", "price": "8700"},
		// M and Negro are both fixed at 0 by the first row, so this row's 9000
		// has nothing left to land on.
		{"sku": "REMERA-BAS", "variant[talle]": "M", "variant[color]": "Negro", "price": "9000"},
	})

	if len(codes[2]) != 1 || !strings.HasPrefix(codes[2][0], inventoryErrors.ImportPriceInconsistent) {
		t.Fatalf("the third row is price-inconsistent, got %v", codes[2])
	}
	if !strings.HasSuffix(codes[2][0], errorParamSeparator+"9000") {
		t.Errorf("the code should quote the cell, got %q", codes[2][0])
	}
	// It left nothing behind: the ledger still says what the first two rows said.
	if rows[0].Product.Modifiers[optionKey(axisPair{Axis: "Talle", Option: "M"})] != 0 {
		t.Error("a refused row must not move the ledger")
	}
}

// A row that repeats a combination's own price is fine — it sums.
func TestAttributePrice_AConsistentRepeatIsAccepted(t *testing.T) {
	_, codes := readAll(t, []map[string]string{
		{"sku": "REMERA-BAS", "name": "Remera Basica", "price": "7500", "variant[talle]": "M", "variant[color]": "Negro"},
		{"sku": "REMERA-BAS", "variant[talle]": "XL", "variant[color]": "Blanco", "price": "8700"},
		// XL = 1200 and Negro = 0, so 8700 is exactly what XL/Negro costs.
		{"sku": "REMERA-BAS", "variant[talle]": "XL", "variant[color]": "Negro", "price": "8700"},
	})

	if len(codes[2]) > 0 {
		t.Fatalf("a row that adds up is accepted, got %v", codes[2])
	}
}

// A blank price on a repeat row means "the product's", exactly like every other
// cell under D9.
func TestAttributePrice_ABlankPriceIsTheProductsOwn(t *testing.T) {
	rows, codes := readAll(t, []map[string]string{
		{"sku": "REMERA-BAS", "name": "Remera Basica", "price": "7500", "variant[talle]": "M", "variant[color]": "Negro"},
		{"sku": "REMERA-BAS", "variant[talle]": "L", "variant[color]": "Negro"},
	})

	if len(codes[1]) > 0 {
		t.Fatalf("unexpected errors %v", codes[1])
	}
	if rows[1].Modifiers[0] != 0 {
		t.Errorf("a blank price carries no difference, got %v", rows[1].Modifiers)
	}
}

// ---------------------------------------------------------------------------
// The image ledger (D7)
// ---------------------------------------------------------------------------

func TestReadOptionImages_OnePictureNamesOneOption(t *testing.T) {
	descriptor := newProductsDescriptor()
	headers := []string{"sku", "name", "price", "variant[talle]", "variant[color]", "image[color]", "image_alt[color]"}
	ctx := catalogueContext(headers, map[string][]byte{"negra.png": []byte("bytes")})

	parsed, fieldErrors, warnings := descriptor.readRow(ctx, map[string]string{
		"sku": "REMERA-BAS", "name": "Remera Basica", "price": "7500",
		"variant[talle]": "M", "variant[color]": "Negro",
		"image[color]": "negra.png", "image_alt[color]": "Remera negra",
	}, false)

	if len(fieldErrors) > 0 {
		t.Fatalf("unexpected errors %v", fieldErrors)
	}
	if len(warnings) > 0 {
		t.Fatalf("the archive carries the file, so no warning: %v", warnings)
	}
	if len(parsed.Images) != 1 {
		t.Fatalf("expected one option image, got %+v", parsed.Images)
	}
	if parsed.Images[0].Axis != "Color" || parsed.Images[0].Option != "Negro" {
		t.Errorf("image[color] is the picture of the row's colour, got %+v", parsed.Images[0])
	}
	if parsed.Images[0].AltText != "Remera negra" {
		t.Errorf("the alt text travels with it, got %q", parsed.Images[0].AltText)
	}
}

// AC-12 — two rows sharing an option but naming different files disagree.
func TestReadOptionImages_TwoRowsMustNameTheSameFileForOneOption(t *testing.T) {
	descriptor := newProductsDescriptor()
	headers := []string{"sku", "name", "price", "variant[talle]", "variant[color]", "image[color]"}
	ctx := catalogueContext(headers, map[string][]byte{"negra.png": []byte("a"), "otra.png": []byte("b")})

	first, _, _ := descriptor.readRow(ctx, map[string]string{
		"sku": "REMERA-BAS", "name": "Remera Basica", "price": "7500",
		"variant[talle]": "M", "variant[color]": "Negro", "image[color]": "negra.png",
	}, false)
	descriptor.remember(first)

	_, fieldErrors, _ := descriptor.readRow(ctx, map[string]string{
		"sku": "REMERA-BAS", "variant[talle]": "L", "variant[color]": "Negro",
		"image[color]": "otra.png",
	}, false)

	if len(fieldErrors) != 1 || !strings.HasPrefix(fieldErrors[0], inventoryErrors.ImportOptionImageInconsistent) {
		t.Fatalf("a second file for one option is option-image-inconsistent, got %v", fieldErrors)
	}

	// The same file again is fine.
	_, fieldErrors, _ = descriptor.readRow(ctx, map[string]string{
		"sku": "REMERA-BAS", "variant[talle]": "XL", "variant[color]": "Negro",
		"image[color]": "negra.png",
	}, false)
	if len(fieldErrors) > 0 {
		t.Errorf("naming the same file again is not a disagreement, got %v", fieldErrors)
	}
}

// AC-12 — a file the ZIP does not carry is a warning, not an error.
func TestReadOptionImages_AMissingFileIsAWarning(t *testing.T) {
	descriptor := newProductsDescriptor()
	headers := []string{"sku", "name", "price", "variant[color]", "image[color]"}
	ctx := catalogueContext(headers, nil)

	_, fieldErrors, warnings := descriptor.readRow(ctx, map[string]string{
		"sku": "REMERA-BAS", "name": "Remera Basica", "price": "7500",
		"variant[color]": "Negro", "image[color]": "ausente.png",
	}, false)

	if len(fieldErrors) > 0 {
		t.Fatalf("a missing file does not refuse the row, got %v", fieldErrors)
	}
	if !slicesContain(warnings, inventoryErrors.ImportImageMissing) {
		t.Errorf("a missing file is image-missing, got %v", warnings)
	}
}

// A picture of an axis the row does not fill has no option to hang off.
func TestReadOptionImages_APictureOfAnAxisTheRowDoesNotFillIsSkipped(t *testing.T) {
	descriptor := newProductsDescriptor()
	headers := []string{"sku", "name", "price", "variant[talle]", "variant[color]", "image[color]"}
	ctx := catalogueContext(headers, map[string][]byte{"negra.png": []byte("a")})

	parsed, fieldErrors, _ := descriptor.readRow(ctx, map[string]string{
		"sku": "COLA", "name": "Gaseosa Cola", "price": "2200",
		"variant[talle]": "500ml", "image[color]": "negra.png",
	}, false)

	if len(fieldErrors) > 0 {
		t.Fatalf("unexpected errors %v", fieldErrors)
	}
	if len(parsed.Images) != 0 {
		t.Errorf("nothing to attach it to, got %+v", parsed.Images)
	}
}

// ---------------------------------------------------------------------------
// The ledger is per run, not per descriptor
// ---------------------------------------------------------------------------

func TestLedger_IsScopedToOneRun(t *testing.T) {
	descriptor := newProductsDescriptor()
	first := catalogueContext(catalogueHeaders(), nil)
	second := catalogueContext(catalogueHeaders(), nil)

	row := map[string]string{
		"sku": "REMERA-BAS", "name": "Remera Basica", "price": "7500",
		"variant[talle]": "M", "variant[color]": "Negro",
	}

	parsed, _, _ := descriptor.readRow(first, row, false)
	descriptor.remember(parsed)

	// The same row in a second run is a first row again: two concurrent imports
	// of the same file must not see each other's state.
	again, _, _ := descriptor.readRow(second, row, false)
	if !again.IsFirst {
		t.Error("a second run starts from an empty ledger")
	}
	if again.Product == parsed.Product {
		t.Error("the two runs must not share a product")
	}
}

func slicesContain(codes []string, wanted string) bool {
	for _, code := range codes {
		if code == wanted || strings.HasPrefix(code, wanted+errorParamSeparator) {
			return true
		}
	}
	return false
}
