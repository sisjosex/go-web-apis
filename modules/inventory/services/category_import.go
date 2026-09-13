package services

import (
	"strconv"
	"strings"

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
	lookups         inventoryInterfaces.ImportLookupRepository
}

// NewCategoriesImportDescriptor builds the Categories import descriptor.
func NewCategoriesImportDescriptor(
	categoryService inventoryInterfaces.CategoryService,
	lookups inventoryInterfaces.ImportLookupRepository,
) importInterfaces.ImportDescriptor {
	return &categoriesImportDescriptor{
		categoryService: categoryService,
		lookups:         lookups,
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

// ValidateRow judges the row against what the real run will see when it gets
// here: the tenant's categories plus the ones the rows above will have created,
// which on a dry run exist only in the pendingCategories ledger. That is what
// lets a child of the row above validate, and what makes a parent no row
// creates the error it is going to be.
func (d *categoriesImportDescriptor) ValidateRow(ctx importModels.ImportContext, line int, row map[string]string) importModels.RowResult {
	result := importModels.RowResult{Line: line, Data: row}

	if fieldErrors := d.checkFormat(row); len(fieldErrors) > 0 {
		result.Status = importModels.RowStatusInvalid
		result.Errors = fieldErrors
		return result
	}

	pending := pendingCategoriesOf(ctx)
	name := strings.TrimSpace(row["name"])

	var parentID *string
	parentKey := ""
	if parent := strings.TrimSpace(row["parent"]); parent != "" {
		if id, found := findCategoryID(d.lookups, ctx, parent, nil); found {
			parentID = &id
			parentKey = id
		} else if path, found := pending.find("", parent); found {
			parentKey = path
		} else {
			result.Status = importModels.RowStatusInvalid
			result.Errors = []string{inventoryErrors.ImportCategoryParentUnknown + errorParamSeparator + parent}
			return result
		}
	}

	// Under a parent the run has not created yet nothing can exist, so only the
	// ledger is asked.
	_, exists := pending.find(parentKey, name)
	if !exists && (parentKey == "" || parentID != nil) {
		_, exists = findCategoryID(d.lookups, ctx, name, parentID)
	}
	if exists {
		result.Status = importModels.RowStatusDuplicate
		result.Errors = []string{inventoryErrors.ImportCategoryDuplicate}
		return result
	}

	pending.add(parentKey, name)

	result.Status = importModels.RowStatusValid
	return result
}

func (d *categoriesImportDescriptor) ProcessRow(ctx importModels.ImportContext, line int, row map[string]string) importModels.RowResult {
	result := importModels.RowResult{Line: line, Data: row}

	if fieldErrors := d.checkFormat(row); len(fieldErrors) > 0 {
		result.Status = importModels.RowStatusFailed
		result.Errors = fieldErrors
		return result
	}

	name := strings.TrimSpace(row["name"])
	var parentID *string
	if parent := strings.TrimSpace(row["parent"]); parent != "" {
		id, found := findCategoryID(d.lookups, ctx, parent, nil)
		if !found {
			result.Status = importModels.RowStatusFailed
			result.Errors = []string{inventoryErrors.ImportCategoryParentUnknown + errorParamSeparator + parent}
			return result
		}
		parentID = &id
	}

	if _, found := findCategoryID(d.lookups, ctx, name, parentID); found {
		result.Status = importModels.RowStatusSkipped
		result.Errors = []string{inventoryErrors.ImportCategoryDuplicate}
		return result
	}

	if _, err := d.categoryService.CreateCategory(ctx.Context(), ctx.TenantID, d.buildDto(ctx, row, name, parentID)); err != nil {
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

// checkFormat validates the row's own cells, the part of the row that does not
// depend on what the run has created so far.
func (d *categoriesImportDescriptor) checkFormat(row map[string]string) []string {
	var fieldErrors []string

	if strings.TrimSpace(row["name"]) == "" {
		fieldErrors = append(fieldErrors, inventoryErrors.ImportNameRequired)
	}
	if _, _, err := parseOptionalNumber(row["display_order"], 0); err != nil {
		fieldErrors = append(fieldErrors, inventoryErrors.ImportQuantityInvalid)
	}

	return fieldErrors
}

// buildDto maps a CSV row to a CreateCategoryDto, giving the category a slug the
// tenant is not already using — the name is the operator's, the slug is ours.
func (d *categoriesImportDescriptor) buildDto(
	ctx importModels.ImportContext,
	row map[string]string,
	name string,
	parentID *string,
) *inventoryModels.CreateCategoryDto {
	dto := &inventoryModels.CreateCategoryDto{
		Name:     name,
		Slug:     findFreeSlug(d.lookups, ctx, slugify(name)),
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
