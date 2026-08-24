package services

import (
	"strings"

	coreServices "josex/web/modules/core/services"
	importInterfaces "josex/web/modules/import/interfaces"
	importModels "josex/web/modules/import/models"
	inventoryErrors "josex/web/modules/inventory/errors"
	inventoryInterfaces "josex/web/modules/inventory/interfaces"
)

// productStockImportDescriptor loads an opening count onto each combination:
// one row per combination, named by `sku` plus the row's `variant[…]` columns
// (INV-016 D6). A row adds a movement, it never sets an absolute quantity —
// running the file twice adds twice, which is why the resource is create-only
// and says so in the docs.
//
// It has no menu entry of its own (D9): the catalogue file seeds the stock of
// what it creates, and this is the path for a catalogue that already exists,
// which the create-only rule leaves no other way to restock.
type productStockImportDescriptor struct {
	movementService inventoryInterfaces.MovementService
	stockService    inventoryInterfaces.StockService
	dbService       coreServices.DatabaseService
}

// NewProductStockImportDescriptor builds the product_stock import descriptor.
func NewProductStockImportDescriptor(
	movementService inventoryInterfaces.MovementService,
	stockService inventoryInterfaces.StockService,
	dbService coreServices.DatabaseService,
) importInterfaces.ImportDescriptor {
	return &productStockImportDescriptor{
		movementService: movementService,
		stockService:    stockService,
		dbService:       dbService,
	}
}

func (d *productStockImportDescriptor) Resource() string {
	return "product_stock"
}

func (d *productStockImportDescriptor) Columns() []importModels.ColumnSpec {
	columns := []importModels.ColumnSpec{
		{Key: "sku", Type: importModels.ColumnTypeText, Required: true},
	}
	columns = append(columns, axisSampleColumns()...)
	return append(columns,
		importModels.ColumnSpec{Key: "quantity", Type: importModels.ColumnTypeNumber, Required: true},
		importModels.ColumnSpec{Key: "reorder_level", Type: importModels.ColumnTypeNumber},
	)
}

// Matches recognizes a stock CSV by sku plus quantity — the pair no other
// inventory resource declares.
func (d *productStockImportDescriptor) Matches(headers []string) bool {
	return hasHeaders(headers, "sku", "quantity") && !hasHeaders(headers, "lot_number")
}

func (d *productStockImportDescriptor) ValidateRow(ctx importModels.ImportContext, line int, row map[string]string) importModels.RowResult {
	result := importModels.RowResult{Line: line, Data: row}

	if fieldErrors, _ := d.check(ctx, row); len(fieldErrors) > 0 {
		result.Status = importModels.RowStatusInvalid
		result.Errors = fieldErrors
		return result
	}

	result.Status = importModels.RowStatusValid
	return result
}

func (d *productStockImportDescriptor) ProcessRow(ctx importModels.ImportContext, line int, row map[string]string) importModels.RowResult {
	result := importModels.RowResult{Line: line, Data: row}

	fieldErrors, target := d.check(ctx, row)
	if len(fieldErrors) > 0 {
		result.Status = importModels.RowStatusFailed
		result.Errors = fieldErrors
		return result
	}

	code, warnings := seedCombination(
		d.movementService, d.stockService, ctx, target, row["quantity"], row["reorder_level"])
	result.Warnings = append(result.Warnings, warnings...)
	if code != "" {
		result.Status = importModels.RowStatusFailed
		result.Errors = []string{code}
		return result
	}

	result.Status = importModels.RowStatusCreated
	return result
}

func (d *productStockImportDescriptor) RecordRun(ctx importModels.ImportContext, meta importModels.RunMeta) {
	logImportRun(meta)
}

// check validates the row and resolves the combination it lands on. Resolution
// is part of validation on purpose: an unknown SKU or a combination a product
// does not have is exactly what the dry run exists to show, and it costs the
// same one query either way. D6 makes that stronger than it is for the
// catalogue file — a stock row's option set either resolves against the
// database or it does not, with no in-file state involved.
func (d *productStockImportDescriptor) check(ctx importModels.ImportContext, row map[string]string) ([]string, skuCandidate) {
	var fieldErrors []string

	if strings.TrimSpace(row["sku"]) == "" {
		fieldErrors = append(fieldErrors, inventoryErrors.ImportSkuRequired)
	}
	if quantity, given, err := parseOptionalNumber(row["quantity"], 0); err != nil || !given || quantity == 0 {
		fieldErrors = append(fieldErrors, inventoryErrors.ImportQuantityInvalid)
	}
	if _, _, err := parseOptionalNumber(row["reorder_level"], 0); err != nil {
		fieldErrors = append(fieldErrors, inventoryErrors.ImportReorderLevelInvalid)
	}

	pairs, code := axisColumns(ctx.Headers, row)
	if code != "" {
		fieldErrors = append(fieldErrors, code)
	}

	if len(fieldErrors) > 0 {
		return fieldErrors, skuCandidate{}
	}

	target, code := resolveSkuByAxes(d.dbService, ctx.TenantID, row["sku"], pairs)
	if code != "" {
		return []string{code}, skuCandidate{}
	}
	return nil, target
}
