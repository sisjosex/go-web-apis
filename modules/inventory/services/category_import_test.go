package services

import (
	"strings"
	"testing"

	importModels "josex/web/modules/import/models"
	inventoryErrors "josex/web/modules/inventory/errors"
)

// validateCategories runs a categories file through the dry run against a tenant
// with no categories, the way the engine does: one row at a time, one Scratch.
func validateCategories(rows []map[string]string) []importModels.RowResult {
	descriptor := &categoriesImportDescriptor{lookups: missingLookups{}}
	ctx := importModels.ImportContext{Scratch: map[string]any{}}

	results := make([]importModels.RowResult, 0, len(rows))
	for index, row := range rows {
		results = append(results, descriptor.ValidateRow(ctx, index+2, row))
	}
	return results
}

// 4-categorias.csv: each row's parent is the row above, which the real run has
// created by then — so the dry run validates all three (INV-017).
func TestCategoriesValidateRow_AChildOfTheRowAboveIsValid(t *testing.T) {
	results := validateCategories([]map[string]string{
		{"name": "Limpieza", "display_order": "40"},
		{"name": "Detergentes", "parent": "Limpieza", "display_order": "10"},
		{"name": "Liquidos", "parent": "detergentes", "display_order": "10"},
	})

	for index, result := range results {
		if result.Status != importModels.RowStatusValid || len(result.Warnings) > 0 {
			t.Errorf("row %d should be valid with no warning, got %s %v %v",
				index+2, result.Status, result.Errors, result.Warnings)
		}
	}
}

// A parent no row above creates is what the real run fails, so the dry run
// fails it too instead of warning.
func TestCategoriesValidateRow_AParentNoRowCreatesIsInvalid(t *testing.T) {
	results := validateCategories([]map[string]string{
		{"name": "Detergentes", "parent": "Limpieza"},
		{"name": "Limpieza"},
	})

	if results[0].Status != importModels.RowStatusInvalid {
		t.Fatalf("a parent created only below the row is unknown when the row runs, got %s", results[0].Status)
	}
	expected := inventoryErrors.ImportCategoryParentUnknown + errorParamSeparator + "Limpieza"
	if len(results[0].Errors) != 1 || results[0].Errors[0] != expected {
		t.Errorf("expected %q, got %v", expected, results[0].Errors)
	}
	if results[1].Status != importModels.RowStatusValid {
		t.Errorf("the parent row itself is fine, got %s %v", results[1].Status, results[1].Errors)
	}
}

// The second row naming a category the first creates is a duplicate, as the
// real run's skip will say.
func TestCategoriesValidateRow_TheSameCategoryTwiceIsADuplicate(t *testing.T) {
	results := validateCategories([]map[string]string{
		{"name": "Limpieza"},
		{"name": "LIMPIEZA"},
		{"name": "Detergentes", "parent": "Limpieza"},
		{"name": "detergentes", "parent": "limpieza"},
	})

	for _, index := range []int{1, 3} {
		if results[index].Status != importModels.RowStatusDuplicate ||
			!strings.HasPrefix(strings.Join(results[index].Errors, ","), inventoryErrors.ImportCategoryDuplicate) {
			t.Errorf("row %d should be a duplicate, got %s %v", index+2, results[index].Status, results[index].Errors)
		}
	}
}
