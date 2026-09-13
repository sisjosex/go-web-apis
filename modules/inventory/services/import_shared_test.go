package services

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"

	inventoryErrors "josex/web/modules/inventory/errors"
	inventoryInterfaces "josex/web/modules/inventory/interfaces"
	inventoryModels "josex/web/modules/inventory/models"
)

// missingLookups is a tenant with nothing in it: every lookup misses. It is what
// lets a white-box test drive the dry run's category ledger, which only runs
// once the lookup has missed.
type missingLookups struct{}

var _ inventoryInterfaces.ImportLookupRepository = missingLookups{}

func (missingLookups) ResolveSkuByAxes(context.Context, uuid.UUID, string, []string, []string) (*inventoryModels.ImportSkuResolution, error) {
	return &inventoryModels.ImportSkuResolution{}, nil
}

func (missingLookups) FindCategoryID(context.Context, uuid.UUID, string, *string) (*string, error) {
	return nil, nil
}

func (missingLookups) FindVariantOptionID(context.Context, uuid.UUID, string, string, string) (*string, error) {
	return nil, nil
}

func (missingLookups) SkuExists(context.Context, uuid.UUID, string) (*inventoryModels.ImportSkuExistence, error) {
	return &inventoryModels.ImportSkuExistence{}, nil
}

func (missingLookups) FreeCategorySlug(_ context.Context, _ uuid.UUID, base string, _ int) (string, error) {
	return base, nil
}

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"Bebidas":         "bebidas",
		"  Yerba  Mate  ": "yerba-mate",
		"Café & Té":       "caf-t",
		"🥤":               "category",
	}

	for name, expected := range cases {
		if got := slugify(name); got != expected {
			t.Errorf("slugify(%q) = %q, expected %q", name, got, expected)
		}
	}
}

func TestParseOptionalNumber(t *testing.T) {
	if _, given, err := parseOptionalNumber("  ", 0); given || err != nil {
		t.Errorf("an empty cell is not given and not an error, got %v / %v", given, err)
	}
	if value, given, err := parseOptionalNumber("1500,50", 0); !given || err != nil || value != 1500.50 {
		t.Errorf("a decimal comma should be accepted, got %v / %v / %v", value, given, err)
	}
	if _, _, err := parseOptionalNumber("-1", 0); err == nil {
		t.Error("a value below the bound should be rejected")
	}
	if _, _, err := parseOptionalNumber("muchas", 0); err == nil {
		t.Error("a non-number should be rejected")
	}
}

// ---------------------------------------------------------------------------
// INV-016 — the variant[<axis>] marker (D1)
// ---------------------------------------------------------------------------

func TestParseMarkerColumn_ReadsTheAxisAndTitleCasesIt(t *testing.T) {
	axis, attempted, valid := parseMarkerColumn("variant[talle]", axisColumnPrefix)
	if !attempted || !valid || axis != "Talle" {
		t.Errorf("variant[talle] is the axis Talle, got %q / %v / %v", axis, attempted, valid)
	}

	if axis, _, _ := parseMarkerColumn("variant[tipo de tela]", axisColumnPrefix); axis != "Tipo de tela" {
		t.Errorf("only the first rune is capitalised, got %q", axis)
	}
}

func TestParseMarkerColumn_TellsAMalformedMarkerFromAnUnrelatedColumn(t *testing.T) {
	// Malformed: it carries the prefix, so it is reported rather than ignored.
	for _, header := range []string{"variant[", "variant[color", "variant[]", "variant[   ]"} {
		_, attempted, valid := parseMarkerColumn(header, axisColumnPrefix)
		if !attempted || valid {
			t.Errorf("%q is an attempted marker and not a valid one, got %v / %v", header, attempted, valid)
		}
	}

	// Not a marker at all: ignored with every other unrecognised column, and
	// variant_sku in particular is a reserved column and never an axis.
	for _, header := range []string{"notas", "variant", "variant_sku", "image"} {
		if _, attempted, _ := parseMarkerColumn(header, axisColumnPrefix); attempted {
			t.Errorf("%q is not a marker attempt", header)
		}
	}
}

func TestParseMarkerColumn_SharesTheBracketWithTheImageColumns(t *testing.T) {
	if axis, _, valid := parseMarkerColumn("image[color]", imageColumnPrefix); !valid || axis != "Color" {
		t.Errorf("image[color] is the axis Color, got %q / %v", axis, valid)
	}
	if axis, _, valid := parseMarkerColumn("image_alt[color]", imageAltColumnPrefix); !valid || axis != "Color" {
		t.Errorf("image_alt[color] is the axis Color, got %q / %v", axis, valid)
	}
	// image_alt[…] does not answer to the image[…] prefix, so one row never
	// reads the same cell twice.
	if _, attempted, _ := parseMarkerColumn("image_alt[color]", imageColumnPrefix); attempted {
		t.Error("image_alt[color] is not an image[…] column")
	}
}

func TestAxisColumns_KeepsTheFilesColumnOrder(t *testing.T) {
	headers := []string{"sku", "name", "price", "variant[talle]", "variant[color]", "stock"}
	row := map[string]string{
		"sku": "REMERA-BAS", "name": "Remera Basica", "price": "7500",
		"variant[talle]": "M", "variant[color]": "Negro", "stock": "10",
	}

	pairs, code := axisColumns(headers, row)

	if code != "" {
		t.Fatalf("unexpected code %q", code)
	}
	if len(pairs) != 2 || pairs[0].Axis != "Talle" || pairs[1].Axis != "Color" {
		t.Fatalf("axes come out left to right, got %+v", pairs)
	}
	if pairs[0].Option != "M" || pairs[1].Option != "Negro" {
		t.Errorf("unexpected options %+v", pairs)
	}
}

// D1 — a product's axis set is the set of markers its rows actually fill, so
// one file gives COLA one axis and REMERA-BAS two.
func TestAxisColumns_OnlyAFilledCellIsAnAxis(t *testing.T) {
	headers := []string{"sku", "variant[talle]", "variant[color]", "variant[tamano]"}

	cola, code := axisColumns(headers, map[string]string{
		"sku": "COLA", "variant[talle]": "", "variant[color]": "   ", "variant[tamano]": "1.5L",
	})
	if code != "" {
		t.Fatalf("unexpected code %q", code)
	}
	if len(cola) != 1 || cola[0].Axis != "Tamano" || cola[0].Option != "1.5L" {
		t.Errorf("a cell holding only spaces counts as empty, got %+v", cola)
	}

	simple, code := axisColumns(headers, map[string]string{"sku": "YERBA-1K"})
	if code != "" || len(simple) != 0 {
		t.Errorf("a row filling no marker is a simple product, got %+v / %q", simple, code)
	}
}

func TestAxisColumns_MalformedMarkerAndRepeatedAxis(t *testing.T) {
	_, code := axisColumns([]string{"sku", "variant[]"}, map[string]string{"sku": "COLA"})
	if !strings.HasPrefix(code, inventoryErrors.ImportAxisColumnInvalid) {
		t.Errorf("a malformed marker is axis-column-invalid, got %q", code)
	}

	_, code = axisColumns(
		[]string{"variant[color]", "variant[Color]"},
		map[string]string{"variant[color]": "Negro"},
	)
	if !strings.HasPrefix(code, inventoryErrors.ImportAxisColumnInvalid) {
		t.Errorf("the same axis declared twice is axis-column-invalid, got %q", code)
	}
}

// D1 — anything that is not a marker is ignored without touching the outcome.
func TestAxisColumns_UnrecognisedColumnsAreIgnored(t *testing.T) {
	headers := []string{"sku", "notas", "variant[talle]", "proveedor"}
	row := map[string]string{"sku": "REMERA-BAS", "notas": "liquidacion", "variant[talle]": "M", "proveedor": "ACME"}

	pairs, code := axisColumns(headers, row)

	if code != "" {
		t.Fatalf("a stray column is not an error, got %q", code)
	}
	if len(pairs) != 1 || pairs[0].Axis != "Talle" {
		t.Errorf("only the marker is an axis, got %+v", pairs)
	}
}

func TestFormatAxisPairs_QuotesTheCombinationBack(t *testing.T) {
	got := formatAxisPairs([]axisPair{{Axis: "Talle", Option: "M"}, {Axis: "Color", Option: "Negro"}})
	if got != "Talle=M, Color=Negro" {
		t.Errorf("unexpected format %q", got)
	}
}
