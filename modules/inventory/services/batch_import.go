package services

import (
	"context"
	"strings"
	"time"

	coreModels "josex/web/modules/core/models"
	coreServices "josex/web/modules/core/services"
	importInterfaces "josex/web/modules/import/interfaces"
	importModels "josex/web/modules/import/models"
	inventoryErrors "josex/web/modules/inventory/errors"
	inventoryInterfaces "josex/web/modules/inventory/interfaces"
	inventoryModels "josex/web/modules/inventory/models"
)

// productBatchesImportDescriptor loads the lots with their expiry dates the FIFO
// sale path reads (D5). Since 20260821000004 a lot books its own PURCHASE
// movement, so this file seeds the stock of its combinations by itself — never
// run it alongside a product_stock file for the same SKU, or the quantity counts
// twice.
type productBatchesImportDescriptor struct {
	batchService inventoryInterfaces.BatchService
	dbService    coreServices.DatabaseService
}

// NewProductBatchesImportDescriptor builds the product_batches import descriptor.
func NewProductBatchesImportDescriptor(
	batchService inventoryInterfaces.BatchService,
	dbService coreServices.DatabaseService,
) importInterfaces.ImportDescriptor {
	return &productBatchesImportDescriptor{
		batchService: batchService,
		dbService:    dbService,
	}
}

func (d *productBatchesImportDescriptor) Resource() string {
	return "product_batches"
}

func (d *productBatchesImportDescriptor) Columns() []importModels.ColumnSpec {
	columns := []importModels.ColumnSpec{
		{Key: "sku", Type: importModels.ColumnTypeText, Required: true},
	}
	columns = append(columns, axisSampleColumns()...)
	return append(columns,
		importModels.ColumnSpec{Key: "lot_number", Type: importModels.ColumnTypeText, Required: true},
		importModels.ColumnSpec{Key: "purchase_date", Type: importModels.ColumnTypeDate, Required: true, Format: dateFormat},
		importModels.ColumnSpec{Key: "expiry_date", Type: importModels.ColumnTypeDate, Required: true, Format: dateFormat},
		importModels.ColumnSpec{Key: "unit_cost", Type: importModels.ColumnTypeNumber, Required: true},
		importModels.ColumnSpec{Key: "initial_quantity", Type: importModels.ColumnTypeNumber, Required: true},
	)
}

// Matches recognizes a lots CSV by sku plus lot_number.
func (d *productBatchesImportDescriptor) Matches(headers []string) bool {
	return hasHeaders(headers, "sku", "lot_number")
}

func (d *productBatchesImportDescriptor) ValidateRow(ctx importModels.ImportContext, line int, row map[string]string) importModels.RowResult {
	result := importModels.RowResult{Line: line, Data: row}

	if fieldErrors, _ := d.check(ctx, row); len(fieldErrors) > 0 {
		result.Status = importModels.RowStatusInvalid
		result.Errors = fieldErrors
		return result
	}

	result.Status = importModels.RowStatusValid
	return result
}

func (d *productBatchesImportDescriptor) ProcessRow(ctx importModels.ImportContext, line int, row map[string]string) importModels.RowResult {
	result := importModels.RowResult{Line: line, Data: row}

	fieldErrors, target := d.check(ctx, row)
	if len(fieldErrors) > 0 {
		result.Status = importModels.RowStatusFailed
		result.Errors = fieldErrors
		return result
	}

	if _, err := d.batchService.CreateBatch(context.Background(), ctx.TenantID, d.buildDto(row, target)); err != nil {
		result.Status = importModels.RowStatusFailed
		result.Errors = []string{inventoryErrors.ImportBatchFailed}
		return result
	}

	result.Status = importModels.RowStatusCreated
	return result
}

func (d *productBatchesImportDescriptor) RecordRun(ctx importModels.ImportContext, meta importModels.RunMeta) {
	logImportRun(meta)
}

// check validates the row and resolves the combination the lot belongs to. Every
// column but the `variant[…]` ones is required: a lot with no cost or no expiry
// date is not a lot the FIFO path can price or rotate. The axis columns are how
// the lot points back at its combination (D6) — the same headers the catalogue
// file declares it with, so nobody types an identifier.
func (d *productBatchesImportDescriptor) check(ctx importModels.ImportContext, row map[string]string) ([]string, skuCandidate) {
	var fieldErrors []string

	if strings.TrimSpace(row["sku"]) == "" {
		fieldErrors = append(fieldErrors, inventoryErrors.ImportSkuRequired)
	}
	if strings.TrimSpace(row["lot_number"]) == "" {
		fieldErrors = append(fieldErrors, inventoryErrors.ImportLotNumberRequired)
	}

	purchaseDate, purchaseOk := parseImportDate(row["purchase_date"])
	expiryDate, expiryOk := parseImportDate(row["expiry_date"])
	if !purchaseOk || !expiryOk {
		fieldErrors = append(fieldErrors, inventoryErrors.ImportDateInvalid)
	} else if expiryDate.Before(purchaseDate) {
		fieldErrors = append(fieldErrors, inventoryErrors.ImportExpiryBeforePurchase)
	}

	if cost, given, err := parseOptionalNumber(row["unit_cost"], 0); err != nil || !given || cost == 0 {
		fieldErrors = append(fieldErrors, inventoryErrors.ImportUnitCostInvalid)
	}
	if quantity, given, err := parseOptionalNumber(row["initial_quantity"], 0); err != nil || !given || quantity == 0 {
		fieldErrors = append(fieldErrors, inventoryErrors.ImportQuantityInvalid)
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

// buildDto maps a validated row to a CreateBatchDto. The SKU always travels:
// sp_create_batch resolves the default bucket for a product without axes and
// refuses one stocked by variant that names none (INV-014 D1), and check() has
// already turned that refusal into a row error rather than a 500.
func (d *productBatchesImportDescriptor) buildDto(row map[string]string, target skuCandidate) *inventoryModels.CreateBatchDto {
	purchaseDate, _ := parseImportDate(row["purchase_date"])
	expiryDate, _ := parseImportDate(row["expiry_date"])
	unitCost, _, _ := parseOptionalNumber(row["unit_cost"], 0)
	quantity, _, _ := parseOptionalNumber(row["initial_quantity"], 0)

	skuID := target.SkuID
	return &inventoryModels.CreateBatchDto{
		ProductID:       target.ProductID,
		SkuID:           &skuID,
		LotNumber:       strings.TrimSpace(row["lot_number"]),
		PurchaseDate:    coreModels.DateOnly(purchaseDate),
		ExpiryDate:      coreModels.DateOnly(expiryDate),
		UnitCost:        unitCost,
		InitialQuantity: quantity,
	}
}

// parseImportDate reads a date cell in the one layout every inventory import
// accepts. An empty cell is not a date, so the caller reports it as one error.
func parseImportDate(cell string) (time.Time, bool) {
	parsed, err := time.Parse(dateLayout, strings.TrimSpace(cell))
	if err != nil {
		return time.Time{}, false
	}
	return parsed, true
}
