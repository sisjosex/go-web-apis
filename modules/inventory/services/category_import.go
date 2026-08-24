package services

import (
	"context"
	"strconv"
	"strings"

	"github.com/google/uuid"

	coreServices "josex/web/modules/core/services"
	importInterfaces "josex/web/modules/import/interfaces"
	importModels "josex/web/modules/import/models"
	inventoryErrors "josex/web/modules/inventory/errors"
	inventoryInterfaces "josex/web/modules/inventory/interfaces"
	inventoryModels "josex/web/modules/inventory/models"
)

// categoriesImportDescriptor loads the category tree from one file (D7). A row's
// `parent` is a name, resolved against what the tenant already has — including
// what an earlier row of the same file has just created, since the engine
// processes rows in order.
type categoriesImportDescriptor struct {
	categoryService inventoryInterfaces.CategoryService
	dbService       coreServices.DatabaseService
}

// NewCategoriesImportDescriptor builds the Categories import descriptor.
func NewCategoriesImportDescriptor(
	categoryService inventoryInterfaces.CategoryService,
	dbService coreServices.DatabaseService,
) importInterfaces.ImportDescriptor {
	return &categoriesImportDescriptor{
		categoryService: categoryService,
		dbService:       dbService,
	}
}

func (d *categoriesImportDescriptor) Resource() string {
	return "categories"
}

func (d *categoriesImportDescriptor) Columns() []importModels.ColumnSpec {
	return []importModels.ColumnSpec{
		{Key: "name", Type: importModels.ColumnTypeText, Required: true},
		{Key: "parent", Type: importModels.ColumnTypeText},
		{Key: "description", Type: importModels.ColumnTypeText},
		{Key: "display_order", Type: importModels.ColumnTypeNumber},
	}
}

// Matches recognizes a categories CSV by what it has and by what it does not:
// `name` is shared with the catalogue file, so the signature is name without
// the two columns that make a file a catalogue file. `price` and not
// `base_price` since INV-016 D3 collapsed the two price columns into one — a
// negative condition naming a column no descriptor publishes any more would let
// a categories file match the catalogue descriptor.
func (d *categoriesImportDescriptor) Matches(headers []string) bool {
	return hasHeaders(headers, "name") &&
		!hasHeaders(headers, "sku") &&
		!hasHeaders(headers, "price")
}

func (d *categoriesImportDescriptor) ValidateRow(ctx importModels.ImportContext, line int, row map[string]string) importModels.RowResult {
	result := importModels.RowResult{Line: line, Data: row}
	fieldErrors, warnings := d.checkFormat(ctx, row)
	result.Warnings = warnings

	if len(fieldErrors) > 0 {
		result.Status = importModels.RowStatusInvalid
		result.Errors = fieldErrors
		return result
	}

	// A parent that does not exist yet is one an earlier row of this file will
	// create, so nothing can be under it and there is nothing to call a
	// duplicate — checkFormat has already warned about it.
	var parentID *string
	if parent := strings.TrimSpace(row["parent"]); parent != "" {
		id, found := findCategoryID(d.dbService, ctx.TenantID, parent, nil)
		if !found {
			result.Status = importModels.RowStatusValid
			return result
		}
		parentID = &id
	}

	if _, found := findCategoryID(d.dbService, ctx.TenantID, row["name"], parentID); found {
		result.Status = importModels.RowStatusDuplicate
		result.Errors = []string{inventoryErrors.ImportCategoryDuplicate}
		return result
	}

	result.Status = importModels.RowStatusValid
	return result
}

func (d *categoriesImportDescriptor) ProcessRow(ctx importModels.ImportContext, line int, row map[string]string) importModels.RowResult {
	result := importModels.RowResult{Line: line, Data: row}
	fieldErrors, warnings := d.checkFormat(ctx, row)
	result.Warnings = warnings

	if len(fieldErrors) > 0 {
		result.Status = importModels.RowStatusFailed
		result.Errors = fieldErrors
		return result
	}

	name := strings.TrimSpace(row["name"])
	var parentID *string
	if parent := strings.TrimSpace(row["parent"]); parent != "" {
		id, found := findCategoryID(d.dbService, ctx.TenantID, parent, nil)
		if !found {
			result.Status = importModels.RowStatusFailed
			result.Errors = []string{inventoryErrors.ImportCategoryParentUnknown + errorParamSeparator + parent}
			return result
		}
		parentID = &id
	}

	if _, found := findCategoryID(d.dbService, ctx.TenantID, name, parentID); found {
		result.Status = importModels.RowStatusSkipped
		result.Errors = []string{inventoryErrors.ImportCategoryDuplicate}
		return result
	}

	if _, err := d.categoryService.CreateCategory(context.Background(), ctx.TenantID, d.buildDto(ctx.TenantID, row, name, parentID)); err != nil {
		result.Status = importModels.RowStatusFailed
		result.Errors = []string{inventoryErrors.ImportCreateFailed}
		return result
	}

	result.Status = importModels.RowStatusCreated
	return result
}

func (d *categoriesImportDescriptor) RecordRun(ctx importModels.ImportContext, meta importModels.RunMeta) {
	logImportRun(meta)
}

// checkFormat validates the row's own cells. A `parent` this tenant does not
// have yet is a warning rather than an error, because the dry run runs before
// any row of the file has been written: the parent may well be the row above.
// ProcessRow, which runs after those rows exist, turns the same miss into an
// error.
func (d *categoriesImportDescriptor) checkFormat(ctx importModels.ImportContext, row map[string]string) ([]string, []string) {
	var fieldErrors []string
	var warnings []string

	if strings.TrimSpace(row["name"]) == "" {
		fieldErrors = append(fieldErrors, inventoryErrors.ImportNameRequired)
	}
	if _, _, err := parseOptionalNumber(row["display_order"], 0); err != nil {
		fieldErrors = append(fieldErrors, inventoryErrors.ImportQuantityInvalid)
	}

	if parent := strings.TrimSpace(row["parent"]); parent != "" {
		if _, found := findCategoryID(d.dbService, ctx.TenantID, parent, nil); !found {
			warnings = append(warnings, inventoryErrors.ImportCategoryParentUnknown+errorParamSeparator+parent)
		}
	}

	return fieldErrors, warnings
}

// buildDto maps a CSV row to a CreateCategoryDto, giving the category a slug the
// tenant is not already using — the name is the operator's, the slug is ours.
func (d *categoriesImportDescriptor) buildDto(
	tenantID uuid.UUID,
	row map[string]string,
	name string,
	parentID *string,
) *inventoryModels.CreateCategoryDto {
	dto := &inventoryModels.CreateCategoryDto{
		Name:     name,
		Slug:     findFreeSlug(d.dbService, tenantID, slugify(name)),
		ParentID: parentID,
	}
	if description := strings.TrimSpace(row["description"]); description != "" {
		dto.Description = &description
	}
	if order, err := strconv.Atoi(strings.TrimSpace(row["display_order"])); err == nil {
		dto.DisplayOrder = &order
	}
	return dto
}
