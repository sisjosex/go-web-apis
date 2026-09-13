package services

import (
	"errors"
	"log"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"

	coreServices "josex/web/modules/core/services"
	importInterfaces "josex/web/modules/import/interfaces"
	importModels "josex/web/modules/import/models"
	inventoryConfig "josex/web/modules/inventory/config"
	inventoryErrors "josex/web/modules/inventory/errors"
	inventoryInterfaces "josex/web/modules/inventory/interfaces"
	inventoryModels "josex/web/modules/inventory/models"
)

const (
	// openingCountMovement is how an imported opening count is booked: an
	// ADJUSTMENT IN, because nothing was bought (D5). Costed stock arrives as a
	// lot instead — unit_cost only travels on PURCHASE and PRODUCTION.
	openingCountMovement = "ADJUSTMENT"
	movementDirectionIn  = "IN"

	// importMovementNote labels the movement in the stock history so a shop can
	// tell an opening count from a hand-entered adjustment.
	importMovementNote = "CSV import"

	// catalogueLedgerKey is where the run's ledger lives in ImportContext.Scratch
	// (INV-016 D9). The descriptor is registered once and shared by every
	// request, so the state one file accumulates cannot live on it.
	catalogueLedgerKey = "inventory.catalogue"

	// priceEpsilon is how close two money amounts have to be to count as equal.
	// Every price is DECIMAL(12,2) by the time it reaches the database, so
	// anything under half a cent is float arithmetic and not a disagreement.
	priceEpsilon = 0.005
)

// productCells are the columns that describe the product rather than the row's
// combination (D9). On a repeated `sku` each of them must be identical or
// empty — both mean "same product" — and a differing non-empty value is
// product-inconsistent.
var productCells = []string{"name", "description", "category", "subcategory", "image", "image_alt"}

// catalogueProduct is what the run remembers about one `sku` across the rows
// that repeat it (D9). Everything in it is in-file state: on a validate run
// nothing is written, and on a process run the rows after the first one attach
// to the ProductID the first one created.
type catalogueProduct struct {
	SKU       string
	ProductID string
	Cells     map[string]string
	BasePrice float64
	// Axes is the axis set of the product's first row, lowercased and in column
	// order. Every later row of the same sku must fill exactly these.
	Axes []string
	// Modifiers is the per-option ledger of D3, and Images the per-option ledger
	// of D7. Both are keyed by "axis=option" lowercased, both are written the
	// first time a row carries the option, and both fail loudly when a later row
	// disagrees.
	Modifiers map[string]float64
	Images    map[string]string
	// Combinations are the option sets this file has already named for this
	// product, so the second row naming one is a duplicate rather than a second
	// attempt at the same insert.
	Combinations map[string]bool
	// Skipped says the product's first row was a duplicate of one the tenant
	// already has, so nothing under it was created. Without it the rows below
	// would look like first rows of a product nobody declared and be refused for
	// having no name and no price — three misleading codes where the honest
	// answer is the one the row above already gave.
	Skipped bool
}

// catalogueLedger is one run's memory. VariantSkus is run-wide rather than
// per-product because product_skus.sku is UNIQUE per tenant.
type catalogueLedger struct {
	Products    map[string]*catalogueProduct
	VariantSkus map[string]bool
}

// catalogueRow is one row after reading: what it names, what it costs, and the
// per-option facts it contributes. ProcessRow acts on it without re-reading a
// single cell.
type catalogueRow struct {
	SKU         string
	Ledger      *catalogueLedger
	Product     *catalogueProduct
	IsFirst     bool
	Pairs       []axisPair
	Key         string
	Modifiers   []float64
	VariantSku  *string
	Images      []optionImage
	CategoryIDs []string
}

// optionImage is one picture of one option, named by the row that carries the
// option (D7).
type optionImage struct {
	Axis     string
	Option   string
	Filename string
	AltText  string
}

// productsImportDescriptor is the catalogue resource: one file, one row per
// sellable thing, one column per axis (INV-016 D9). It absorbs what INV-015
// split across a products file and its continuation rows.
type productsImportDescriptor struct {
	productService  inventoryInterfaces.ProductService
	categoryService inventoryInterfaces.CategoryService
	movementService inventoryInterfaces.MovementService
	stockService    inventoryInterfaces.StockService
	skuService      inventoryInterfaces.SkuService
	lookups         inventoryInterfaces.ImportLookupRepository
	mediaService    coreServices.MediaService
	config          *inventoryConfig.InventoryConfig
}

// NewProductsImportDescriptor builds the catalogue import descriptor out of the
// services that already exist. lookups backs the read-only previews the dry run
// needs (the SKU existence preview, category resolution by name, the option a
// picture hangs off); mediaService stores the images the companion archive
// carries; skuService creates the one combination a row names.
func NewProductsImportDescriptor(
	productService inventoryInterfaces.ProductService,
	categoryService inventoryInterfaces.CategoryService,
	movementService inventoryInterfaces.MovementService,
	stockService inventoryInterfaces.StockService,
	skuService inventoryInterfaces.SkuService,
	lookups inventoryInterfaces.ImportLookupRepository,
	mediaService coreServices.MediaService,
	config *inventoryConfig.InventoryConfig,
) importInterfaces.ImportDescriptor {
	return &productsImportDescriptor{
		productService:  productService,
		categoryService: categoryService,
		movementService: movementService,
		stockService:    stockService,
		skuService:      skuService,
		lookups:         lookups,
		mediaService:    mediaService,
		config:          config,
	}
}

func (d *productsImportDescriptor) Resource() string {
	return "products"
}

// Columns is the canonical column order with the type and validation a client
// needs to edit each cell. The two variant[…] entries are samples, not a fixed
// list: any number of them is accepted and the axis is whatever the bracket
// names (D1). They are in the template so the marker is visible without reading
// the docs, and product_stock and product_batches carry the same two (AC-14).
func (d *productsImportDescriptor) Columns() []importModels.ColumnSpec {
	columns := []importModels.ColumnSpec{
		{Key: "sku", Type: importModels.ColumnTypeText, Required: true},
		// name and price are required on a product's first row only, never on a
		// repeat row (D9), and Required is a flag the client enforces per cell —
		// so marking them here would block the wizard on the normal file. The
		// engine ignores the flag; readRow does the real check on IsFirst.
		{Key: "name", Type: importModels.ColumnTypeText},
		{Key: "description", Type: importModels.ColumnTypeText},
		{Key: "price", Type: importModels.ColumnTypeNumber},
		{Key: "category", Type: importModels.ColumnTypeText},
		{Key: "subcategory", Type: importModels.ColumnTypeText},
	}
	columns = append(columns, axisSampleColumns()...)
	return append(columns,
		importModels.ColumnSpec{Key: "variant_sku", Type: importModels.ColumnTypeText},
		importModels.ColumnSpec{Key: "stock", Type: importModels.ColumnTypeNumber},
		importModels.ColumnSpec{Key: "reorder_level", Type: importModels.ColumnTypeNumber},
		importModels.ColumnSpec{Key: "image", Type: importModels.ColumnTypeImage, Format: imageNameFormat},
		importModels.ColumnSpec{Key: "image_alt", Type: importModels.ColumnTypeText},
		importModels.ColumnSpec{
			Key:    imageColumnPrefix + sampleSecondAxis + axisColumnSuffix,
			Type:   importModels.ColumnTypeImage,
			Format: imageNameFormat,
		},
		importModels.ColumnSpec{
			Key:  imageAltColumnPrefix + sampleSecondAxis + axisColumnSuffix,
			Type: importModels.ColumnTypeText,
		},
	)
}

// Matches recognizes the catalogue file by sku + name + price. D6 made `sku`
// and the variant[…] columns common to all three product files, so what
// separates them is one column each: `price` here, `quantity` for
// product_stock, `lot_number` for product_batches.
func (d *productsImportDescriptor) Matches(headers []string) bool {
	return hasHeaders(headers, "sku", "name", "price")
}

func (d *productsImportDescriptor) ValidateRow(ctx importModels.ImportContext, line int, row map[string]string) importModels.RowResult {
	result := importModels.RowResult{Line: line, Data: row}

	parsed, fieldErrors, warnings := d.readRow(ctx, row, false)
	result.Warnings = warnings

	if len(fieldErrors) > 0 {
		result.Status = importModels.RowStatusInvalid
		if parsed.Product != nil && parsed.Product.Skipped {
			result.Status = importModels.RowStatusDuplicate
		}
		result.Errors = fieldErrors
		return result
	}

	// D4/D9 — what a duplicate is: the product for a first row, the combination
	// for a repeat row, and a variant_sku the tenant already has either way.
	if code := d.duplicateOf(ctx, parsed, row); code != "" {
		d.rememberSkipped(parsed, code)
		result.Status = importModels.RowStatusDuplicate
		result.Errors = []string{code}
		return result
	}

	d.remember(parsed)

	result.Status = importModels.RowStatusValid
	return result
}

func (d *productsImportDescriptor) ProcessRow(ctx importModels.ImportContext, line int, row map[string]string) importModels.RowResult {
	result := importModels.RowResult{Line: line, Data: row}

	parsed, fieldErrors, warnings := d.readRow(ctx, row, true)
	result.Warnings = warnings

	if len(fieldErrors) > 0 {
		result.Status = importModels.RowStatusFailed
		if parsed.Product != nil && parsed.Product.Skipped {
			result.Status = importModels.RowStatusSkipped
		}
		result.Errors = fieldErrors
		return result
	}

	// Only the in-file duplicate is decided here: a SKU the tenant already has
	// comes back from the create SP, which is the one reader that cannot race.
	if parsed.Product.Combinations[parsed.Key] {
		result.Status = importModels.RowStatusSkipped
		result.Errors = []string{inventoryErrors.ImportCombinationDuplicate}
		return result
	}

	if parsed.IsFirst {
		if code, status := d.createProduct(ctx, parsed, row); code != "" {
			d.rememberSkipped(parsed, code)
			result.Status = status
			result.Errors = []string{code}
			return result
		}
		// Everything below runs against a product that exists, so a failure
		// degrades the row with a warning instead of failing it.
		result.Warnings = append(result.Warnings, d.assignCategories(ctx, parsed.Product.ProductID, parsed.CategoryIDs)...)
		result.Warnings = append(result.Warnings, d.attachProductImage(ctx, parsed.Product.ProductID, row)...)
	}

	target := skuCandidate{ProductID: parsed.Product.ProductID}
	if len(parsed.Pairs) == 0 {
		// A row with no axis cell is the whole product: its stock seeds the
		// default bucket sp_create_product_with_variants just made (D9).
		resolved, code := resolveSkuByAxes(d.lookups, ctx, parsed.SKU, nil)
		if code != "" {
			result.Status = importModels.RowStatusFailed
			result.Errors = []string{code}
			return result
		}
		target = resolved
	} else {
		created, code, status := d.createCombination(ctx, parsed)
		if code != "" {
			result.Status = status
			result.Errors = []string{code}
			return result
		}
		target = skuCandidate{ProductID: created.ProductID, SkuID: created.SkuID, SKU: created.SKU}
		result.Warnings = append(result.Warnings, d.attachOptionImages(ctx, parsed)...)
	}

	failure, warnings := seedCombination(
		d.movementService, d.stockService, ctx, target, row["stock"], row["reorder_level"])
	result.Warnings = append(result.Warnings, warnings...)
	if failure != "" {
		result.Warnings = append(result.Warnings, failure)
	}

	d.remember(parsed)

	result.Status = importModels.RowStatusCreated
	return result
}

// RecordRun persists the run summary. Inventory keeps no audit trail of its own,
// so the summary goes to the server log; best-effort by contract, the engine
// ignores the outcome.
func (d *productsImportDescriptor) RecordRun(ctx importModels.ImportContext, meta importModels.RunMeta) {
	logImportRun(meta)
}

// readRow is the whole per-row contract in one pass, shared by the dry run and
// the real one: the row's axes, its place in the product the rows above
// declared, its price and the per-option facts it contributes. write is what
// separates the two — with it false nothing is created, so an unknown category
// is only announced as the warning it will raise.
//
// It never returns a nil row alongside an empty error list.
func (d *productsImportDescriptor) readRow(
	ctx importModels.ImportContext,
	row map[string]string,
	write bool,
) (*catalogueRow, []string, []string) {
	var fieldErrors []string
	var warnings []string

	parsed := &catalogueRow{SKU: strings.TrimSpace(row["sku"])}
	if parsed.SKU == "" {
		return parsed, []string{inventoryErrors.ImportProductSkuRequired}, nil
	}

	pairs, code := axisColumns(ctx.Headers, row)
	if code != "" {
		return parsed, []string{code}, nil
	}
	if d.config != nil && len(pairs) > d.config.MaxAxes {
		return parsed, []string{inventoryErrors.ImportAxesTooMany}, nil
	}
	parsed.Pairs = pairs
	parsed.Key = combinationKeyOf(pairs)

	ledger := d.ledger(ctx)
	parsed.Ledger = ledger
	product, known := ledger.Products[strings.ToLower(parsed.SKU)]
	parsed.IsFirst = !known
	if !known {
		product = &catalogueProduct{
			SKU:          parsed.SKU,
			Cells:        map[string]string{},
			Axes:         axisNamesOf(pairs),
			Modifiers:    map[string]float64{},
			Images:       map[string]string{},
			Combinations: map[string]bool{},
		}
		for _, cell := range productCells {
			product.Cells[cell] = strings.TrimSpace(row[cell])
		}
	}
	parsed.Product = product

	// Nothing was created for this product, so a row under it has nothing to
	// attach to and repeats what the row above already reported.
	if product.Skipped {
		return parsed, []string{inventoryErrors.ImportSkuDuplicate}, nil
	}

	price, priceGiven, priceErr := parseOptionalNumber(row["price"], 0)

	if parsed.IsFirst {
		if strings.TrimSpace(row["name"]) == "" {
			fieldErrors = append(fieldErrors, inventoryErrors.ImportNameRequired)
		}
		if priceErr != nil || !priceGiven {
			fieldErrors = append(fieldErrors, inventoryErrors.ImportPriceInvalid)
		}
		product.BasePrice = price
	} else {
		if priceErr != nil {
			fieldErrors = append(fieldErrors, inventoryErrors.ImportPriceInvalid)
		}
		if code := reconcileProductCells(product, row); code != "" {
			fieldErrors = append(fieldErrors, code)
		}
		if !sameAxisSet(product.Axes, pairs) {
			fieldErrors = append(fieldErrors, inventoryErrors.ImportAxesInconsistent)
		}
	}

	// INV-017 D2 — 0 is a price: a free item, a sample. Accepted, and said out
	// loud, because it is also what a forgotten cell looks like.
	if priceErr == nil && priceGiven && price == 0 {
		warnings = append(warnings, inventoryErrors.ImportPriceZero)
	}

	if code := d.readVariantSku(ledger, row, parsed); code != "" {
		fieldErrors = append(fieldErrors, code)
	}

	if _, _, err := parseOptionalNumber(row["stock"], 0); err != nil {
		fieldErrors = append(fieldErrors, inventoryErrors.ImportQuantityInvalid)
	}
	if _, _, err := parseOptionalNumber(row["reorder_level"], 0); err != nil {
		fieldErrors = append(fieldErrors, inventoryErrors.ImportReorderLevelInvalid)
	}

	// Attribution and the image ledger are only meaningful once the row is
	// readable: both write into the product's ledger, and a row that is about to
	// be refused must not leave anything behind.
	if len(fieldErrors) > 0 {
		return parsed, fieldErrors, warnings
	}

	rowPrice := product.BasePrice
	if priceGiven {
		rowPrice = price
	}
	modifiers, attributed, code := attributePrice(product, pairs, rowPrice)
	if code != "" {
		return parsed, []string{code + errorParamSeparator + strings.TrimSpace(row["price"])}, warnings
	}
	parsed.Modifiers = modifiers
	// A1-D1 — where the difference landed is derived, never written, so the row
	// says so out loud. It is the only place the operator learns both rules:
	// the base is the product's first row, and left-to-right picks the option.
	if attributed != "" {
		warnings = append(warnings, inventoryErrors.ImportPriceAttributed+errorParamSeparator+attributed)
	}

	images, imageWarnings, code := d.readOptionImages(ctx, row, product, pairs)
	if code != "" {
		return parsed, []string{code}, warnings
	}
	parsed.Images = images
	warnings = append(warnings, imageWarnings...)

	if image := strings.TrimSpace(row["image"]); parsed.IsFirst && image != "" {
		if _, found := ctx.Images.Lookup(image); !found {
			warnings = append(warnings, inventoryErrors.ImportImageMissing)
		}
	}

	if parsed.IsFirst {
		categoryIDs, categoryErrors, categoryWarnings := d.resolveCategories(ctx, row, write)
		if len(categoryErrors) > 0 {
			return parsed, categoryErrors, append(warnings, categoryWarnings...)
		}
		parsed.CategoryIDs = categoryIDs
		warnings = append(warnings, categoryWarnings...)
	}

	return parsed, nil, warnings
}

// remember writes the row into the run's ledger. It runs after the row is
// accepted, so a refused row leaves no trace and the row after it is judged
// against the same state it would have been.
func (d *productsImportDescriptor) remember(parsed *catalogueRow) {
	if parsed == nil || parsed.Product == nil {
		return
	}

	for index, pair := range parsed.Pairs {
		if index < len(parsed.Modifiers) {
			parsed.Product.Modifiers[optionKey(pair)] = parsed.Modifiers[index]
		}
	}
	for _, image := range parsed.Images {
		parsed.Product.Images[optionKey(axisPair{Axis: image.Axis, Option: image.Option})] = image.Filename
	}
	parsed.Ledger.Products[strings.ToLower(parsed.SKU)] = parsed.Product
	parsed.Product.Combinations[parsed.Key] = true
	if parsed.VariantSku != nil && parsed.Ledger != nil {
		parsed.Ledger.VariantSkus[strings.ToLower(*parsed.VariantSku)] = true
	}
}

// rememberSkipped records a product whose first row was refused as a duplicate
// of one the tenant already has, so the rows under it are judged as the repeat
// rows they are. Only the product-level duplicate does this: a duplicate
// combination or a taken code refuses one row, not the product.
func (d *productsImportDescriptor) rememberSkipped(parsed *catalogueRow, code string) {
	if parsed == nil || parsed.Product == nil || !parsed.IsFirst || code != inventoryErrors.ImportSkuDuplicate {
		return
	}

	parsed.Product.Skipped = true
	parsed.Ledger.Products[strings.ToLower(parsed.SKU)] = parsed.Product
}

// duplicateOf is the dry run's answer to "does this already exist": the product
// for a first row, the combination for a repeat row, and a variant_sku the
// tenant already has either way. It reads and never writes.
func (d *productsImportDescriptor) duplicateOf(ctx importModels.ImportContext, parsed *catalogueRow, row map[string]string) string {
	if parsed.Product.Combinations[parsed.Key] {
		return inventoryErrors.ImportCombinationDuplicate
	}
	if parsed.IsFirst && d.codeTaken(ctx, parsed.SKU).ProductSkuTaken {
		return inventoryErrors.ImportSkuDuplicate
	}
	if parsed.VariantSku != nil && d.codeTaken(ctx, *parsed.VariantSku).VariantSkuTaken {
		return inventoryErrors.ImportVariantSkuDuplicate
	}
	return ""
}

// createProduct creates the product a first row declares. D9 leaves it a plain
// product: its axes arrive one combination at a time, so nothing here builds a
// variant tree and nothing generates a cartesian product (D2).
func (d *productsImportDescriptor) createProduct(ctx importModels.ImportContext, parsed *catalogueRow, row map[string]string) (string, string) {
	dto := inventoryModels.CreateProductDto{
		SKU:       parsed.SKU,
		Name:      strings.TrimSpace(row["name"]),
		BasePrice: parsed.Product.BasePrice,
	}
	if description := strings.TrimSpace(row["description"]); description != "" {
		dto.Description = &description
	}

	product, err := d.productService.CreateProductWithVariants(ctx.Context(), ctx.TenantID, dto)
	if err != nil {
		// D4 — a repeated SKU is a skip, so the rest of the file still imports.
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Message == inventoryErrors.ProductSkuAlreadyExists {
			return inventoryErrors.ImportSkuDuplicate, importModels.RowStatusSkipped
		}
		return inventoryErrors.ImportCreateFailed, importModels.RowStatusFailed
	}

	parsed.Product.ProductID = product.ProductID
	return "", ""
}

// createCombination creates the one combination the row names, through the SP
// that creates the axis and the option on the way in (D2). Everything it can
// refuse is a named error, so a row that loses a race for a code is skipped
// rather than failed.
func (d *productsImportDescriptor) createCombination(
	ctx importModels.ImportContext,
	parsed *catalogueRow,
) (*inventoryModels.CreateCombinationResponse, string, string) {
	if d.skuService == nil || parsed.Product.ProductID == "" {
		return nil, inventoryErrors.ImportCombinationFailed, importModels.RowStatusFailed
	}

	axes := make([]inventoryModels.CombinationAxisDto, 0, len(parsed.Pairs))
	for index, pair := range parsed.Pairs {
		modifier := 0.0
		if index < len(parsed.Modifiers) {
			modifier = parsed.Modifiers[index]
		}
		axes = append(axes, inventoryModels.CombinationAxisDto{
			Axis:     pair.Axis,
			Option:   pair.Option,
			Modifier: modifier,
		})
	}

	created, err := d.skuService.CreateCombination(
		ctx.Context(),
		ctx.TenantID,
		parsed.Product.ProductID,
		inventoryModels.CreateCombinationDto{Axes: axes, SKU: parsed.VariantSku},
	)
	if err == nil {
		return created, "", ""
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Message {
		case skuCombinationExists:
			return nil, inventoryErrors.ImportCombinationDuplicate, importModels.RowStatusSkipped
		case skuCollision:
			return nil, inventoryErrors.ImportVariantSkuDuplicate, importModels.RowStatusSkipped
		case skuCapExceeded:
			return nil, inventoryErrors.ImportAxesTooMany, importModels.RowStatusFailed
		}
	}
	return nil, inventoryErrors.ImportCombinationFailed, importModels.RowStatusFailed
}

// readVariantSku reads the optional code column (D6/D8). A filled one is the
// code verbatim, so the only checks are the ones the row cannot recover from:
// longer than product_skus.sku holds, or already claimed earlier in this file.
// No prefix check — a code the operator went out of their way to type is the
// code they meant (D8).
func (d *productsImportDescriptor) readVariantSku(ledger *catalogueLedger, row map[string]string, parsed *catalogueRow) string {
	code := strings.TrimSpace(row["variant_sku"])
	if code == "" || len(parsed.Pairs) == 0 {
		return ""
	}

	if len(code) > skuMaxLength {
		return inventoryErrors.ImportVariantSkuTooLong
	}
	if ledger.VariantSkus[strings.ToLower(code)] {
		return inventoryErrors.ImportVariantSkuDuplicate
	}

	parsed.VariantSku = &code
	return ""
}

// readOptionImages reads the `image[<axis>]` and `image_alt[<axis>]` columns of
// one row into the pictures of the options it carries (D7). It is the same
// mechanism as the price ledger: a per-option fact declared on the rows that
// carry the option, accumulated for the length of the run, and loud when two
// rows disagree.
func (d *productsImportDescriptor) readOptionImages(
	ctx importModels.ImportContext,
	row map[string]string,
	product *catalogueProduct,
	pairs []axisPair,
) ([]optionImage, []string, string) {
	var images []optionImage
	var warnings []string

	for _, header := range ctx.Headers {
		axis, attempted, valid := parseMarkerColumn(header, imageColumnPrefix)
		if !attempted {
			continue
		}
		if !valid {
			return nil, nil, inventoryErrors.ImportAxisColumnInvalid + errorParamSeparator + header
		}

		filename := strings.TrimSpace(row[header])
		if filename == "" {
			continue
		}

		// A picture of an axis this row does not fill has no option to hang off.
		option, carried := optionOfAxis(pairs, axis)
		if !carried {
			continue
		}

		key := optionKey(axisPair{Axis: axis, Option: option})
		if known, seen := product.Images[key]; seen && !strings.EqualFold(known, filename) {
			return nil, nil, inventoryErrors.ImportOptionImageInconsistent + errorParamSeparator + option
		}

		if _, found := ctx.Images.Lookup(filename); !found {
			warnings = append(warnings, inventoryErrors.ImportImageMissing)
		}

		images = append(images, optionImage{
			Axis:     axis,
			Option:   option,
			Filename: filename,
			AltText:  strings.TrimSpace(row[imageAltColumnPrefix+strings.ToLower(axis)+axisColumnSuffix]),
		})
	}

	return images, warnings, ""
}

// attachOptionImages writes each picture the row named and hangs it off the
// option it is of, not off the combination (D7): product_media references one
// option, so the same photo serves every size of the black shirt. sp_add_product_media
// scopes IsPrimary to the option, so each one is primary within its own scope.
func (d *productsImportDescriptor) attachOptionImages(ctx importModels.ImportContext, parsed *catalogueRow) []string {
	var warnings []string

	for _, image := range parsed.Images {
		// Only the first row carrying the option attaches it; the rows after it
		// have already been checked to name the same file.
		if _, seen := parsed.Product.Images[optionKey(axisPair{Axis: image.Axis, Option: image.Option})]; seen {
			continue
		}
		if d.mediaService == nil {
			continue
		}

		content, found := ctx.Images.Lookup(image.Filename)
		if !found {
			continue
		}

		optionID, ok := findVariantOptionID(d.lookups, ctx, parsed.Product.ProductID, image.Axis, image.Option)
		if !ok {
			warnings = append(warnings, inventoryErrors.ImportMediaFailed)
			continue
		}

		url, err := d.mediaService.Save(productsMediaCategory, coreServices.MediaFile{Filename: image.Filename, Content: content})
		if err != nil {
			warnings = append(warnings, inventoryErrors.ImportMediaFailed)
			continue
		}

		dto := inventoryModels.AddProductMediaDto{
			VariantOptionID: &optionID,
			MediaType:       "image",
			URL:             url,
			IsPrimary:       true,
		}
		if image.AltText != "" {
			alt := image.AltText
			dto.AltText = &alt
		}

		if _, err := d.productService.AddProductMedia(ctx.Context(), ctx.TenantID, parsed.Product.ProductID, dto); err != nil {
			warnings = append(warnings, inventoryErrors.ImportMediaFailed)
		}
	}

	return warnings
}

// ledger returns the run's memory, creating it on the first row. Scratch is
// per-run by construction (the engine makes a fresh one for every validate and
// every process pass), so nothing here leaks into the next import.
func (d *productsImportDescriptor) ledger(ctx importModels.ImportContext) *catalogueLedger {
	if ctx.Scratch == nil {
		return &catalogueLedger{Products: map[string]*catalogueProduct{}, VariantSkus: map[string]bool{}}
	}
	if existing, ok := ctx.Scratch[catalogueLedgerKey].(*catalogueLedger); ok {
		return existing
	}

	ledger := &catalogueLedger{Products: map[string]*catalogueProduct{}, VariantSkus: map[string]bool{}}
	ctx.Scratch[catalogueLedgerKey] = ledger
	return ledger
}

// resolveCategories turns the `category` and `subcategory` cells into the ids
// the row will be assigned to. A name this tenant does not have is created
// rather than refused (D7/D8) and reported as a warning; the create_categories
// option turns that off, and then an unknown name is a row error quoting it back.
//
// The subcategory is looked up among the children of the category, so two
// tenants' "Gaseosas" under different parents never collide. With write false —
// the dry run — nothing is created: the run's pendingCategories ledger stands in
// for what the rows above will have created, so a category is announced once,
// on the row the real run creates it at, and a subcategory whose parent does
// not exist yet is only looked for there.
func (d *productsImportDescriptor) resolveCategories(ctx importModels.ImportContext, row map[string]string, write bool) ([]string, []string, []string) {
	categoryName := strings.TrimSpace(row["category"])
	subName := strings.TrimSpace(row["subcategory"])
	if (categoryName == "" && subName == "") || d.lookups == nil {
		return nil, nil, nil
	}

	create := ctx.Options.BoolOr("create_categories", true)
	pending := pendingCategoriesOf(ctx)

	var categoryIDs []string
	var warnings []string
	var parentID *string
	parentKey := ""
	parentPending := false

	if categoryName != "" {
		id, code, warning := d.categoryFor(ctx, pending, categoryName, nil, "", false, create, write)
		if code != "" {
			return nil, []string{code}, warnings
		}
		if warning != "" {
			warnings = append(warnings, warning)
		}
		if id != "" {
			categoryIDs = append(categoryIDs, id)
			parentID = &id
			parentKey = id
		} else {
			parentPending = true
			parentKey, _ = pending.find("", categoryName)
		}
	}

	if subName != "" {
		id, code, warning := d.categoryFor(ctx, pending, subName, parentID, parentKey, parentPending, create, write)
		if code != "" {
			return categoryIDs, []string{code}, warnings
		}
		if warning != "" {
			warnings = append(warnings, warning)
		}
		if id != "" {
			categoryIDs = append(categoryIDs, id)
		}
	}

	return categoryIDs, nil, warnings
}

// categoryFor finds one category by name, creating it when the run is allowed
// to. parentPending says the parent itself is only going to exist after this
// run, so there is nothing to search under and the lookup is skipped; parentKey
// is where the dry run's ledger files it. It returns the id, the row error code
// if the name cannot be used, and the warning to attach when the category was
// (or will be) created.
func (d *productsImportDescriptor) categoryFor(
	ctx importModels.ImportContext,
	pending *pendingCategories,
	name string,
	parentID *string,
	parentKey string,
	parentPending, create, write bool,
) (string, string, string) {
	if !parentPending {
		if id, found := findCategoryID(d.lookups, ctx, name, parentID); found {
			return id, "", ""
		}
	}

	if !create {
		return "", inventoryErrors.ImportCategoryUnknown + errorParamSeparator + name, ""
	}

	warning := inventoryErrors.ImportCategoryCreated + errorParamSeparator + name
	if !write {
		// A row above already creates it, and the real run finds it by then.
		if _, found := pending.find(parentKey, name); found {
			return "", "", ""
		}
		pending.add(parentKey, name)
		return "", "", warning
	}

	created, err := d.categoryService.CreateCategory(ctx.Context(), ctx.TenantID, &inventoryModels.CreateCategoryDto{
		Name:     name,
		Slug:     findFreeSlug(d.lookups, ctx, slugify(name)),
		ParentID: parentID,
	})
	if err != nil {
		return "", inventoryErrors.ImportCategoryUnknown + errorParamSeparator + name, ""
	}
	return created.ID.String(), "", warning
}

// assignCategories maps the product onto every category the row resolved. The
// product already exists by now, so a failed assignment degrades the row.
func (d *productsImportDescriptor) assignCategories(ctx importModels.ImportContext, productID string, categoryIDs []string) []string {
	var warnings []string

	for _, categoryID := range categoryIDs {
		if err := d.categoryService.AssignProductToCategory(ctx.Context(), ctx.TenantID, productID, categoryID); err != nil {
			warnings = append(warnings, inventoryErrors.ImportCategoryAssignFailed)
		}
	}

	return warnings
}

// attachProductImage writes the archive entry the `image` cell names and hangs
// it off the product as its product-level primary — unchanged by D7, which adds
// the option-level pictures beside it rather than instead of it.
func (d *productsImportDescriptor) attachProductImage(ctx importModels.ImportContext, productID string, row map[string]string) []string {
	filename := strings.TrimSpace(row["image"])
	if filename == "" || d.mediaService == nil {
		return nil
	}

	content, found := ctx.Images.Lookup(filename)
	if !found {
		return nil
	}

	url, err := d.mediaService.Save(productsMediaCategory, coreServices.MediaFile{Filename: filename, Content: content})
	if err != nil {
		return []string{inventoryErrors.ImportMediaFailed}
	}

	dto := inventoryModels.AddProductMediaDto{
		MediaType: "image",
		URL:       url,
		IsPrimary: true,
	}
	if alt := strings.TrimSpace(row["image_alt"]); alt != "" {
		dto.AltText = &alt
	}

	if _, err := d.productService.AddProductMedia(ctx.Context(), ctx.TenantID, productID, dto); err != nil {
		return []string{inventoryErrors.ImportMediaFailed}
	}
	return nil
}

// codeTaken is the read-only preview of what the create SPs enforce, so the dry
// run reports a duplicate without writing anything: a product code for a first
// row, and a combination code the row supplies — product_skus is UNIQUE
// (tenant_id, sku), so one another product already carries is a duplicate
// before anything is written (AC-8). A failed lookup previews nothing; the real
// run still gets the SP's answer.
func (d *productsImportDescriptor) codeTaken(ctx importModels.ImportContext, sku string) inventoryModels.ImportSkuExistence {
	sku = strings.TrimSpace(sku)
	if sku == "" || d.lookups == nil {
		return inventoryModels.ImportSkuExistence{}
	}

	existence, err := d.lookups.SkuExists(ctx.Context(), ctx.TenantID, sku)
	if err != nil {
		return inventoryModels.ImportSkuExistence{}
	}
	return *existence
}

// reconcileProductCells checks a repeat row against the product's first one
// (D9). Identical and empty both mean "same product", so the fill-down operator
// and the leave-it-blank operator are both right; a differing non-empty value is
// the one thing that cannot be meant, and it names the column it came from.
func reconcileProductCells(product *catalogueProduct, row map[string]string) string {
	for _, cell := range productCells {
		value := strings.TrimSpace(row[cell])
		if value == "" || value == product.Cells[cell] {
			continue
		}
		return inventoryErrors.ImportProductInconsistent + errorParamSeparator + cell
	}
	return ""
}

// sameAxisSet reports whether a repeat row fills exactly the axes the product's
// first row did (D1). Order does not matter — a spreadsheet cannot reorder its
// columns between rows — but presence does.
func sameAxisSet(axes []string, pairs []axisPair) bool {
	if len(axes) != len(pairs) {
		return false
	}

	present := make(map[string]bool, len(axes))
	for _, axis := range axes {
		present[axis] = true
	}
	for _, pair := range pairs {
		if !present[strings.ToLower(pair.Axis)] {
			return false
		}
	}
	return true
}

// attributePrice turns the row's price into the per-option modifiers D3 asks
// for: the difference over the product's base price, minus what the options
// already carry, goes to the first axis column of the row whose option has no
// modifier yet, and the row's remaining unfixed options are fixed at 0.
//
// A row whose options are all already fixed and whose modifiers do not sum to
// its difference is price-inconsistent: it is the one case where the file says
// two incompatible things about the same option, and there is nothing left to
// absorb the difference.
//
// The second return is the attribution the row's warning quotes back — the
// option that took the difference and how much it took, `XL +1200` (A1-D1).
// Empty when nothing non-zero was attributed, which is most rows: neither
// column order nor the base-price rule is worth a message when the row costs
// exactly what its product does.
func attributePrice(product *catalogueProduct, pairs []axisPair, rowPrice float64) ([]float64, string, string) {
	difference := rowPrice - product.BasePrice
	modifiers := make([]float64, len(pairs))

	fixed := 0.0
	firstUnfixed := -1
	for index, pair := range pairs {
		if modifier, known := product.Modifiers[optionKey(pair)]; known {
			modifiers[index] = modifier
			fixed += modifier
			continue
		}
		if firstUnfixed < 0 {
			firstUnfixed = index
		}
	}

	if firstUnfixed < 0 {
		if math.Abs(fixed-difference) > priceEpsilon {
			return nil, "", inventoryErrors.ImportPriceInconsistent
		}
		return modifiers, "", ""
	}

	modifiers[firstUnfixed] = difference - fixed
	if math.Abs(modifiers[firstUnfixed]) <= priceEpsilon {
		return modifiers, "", ""
	}
	return modifiers, describeAttribution(pairs[firstUnfixed], modifiers[firstUnfixed]), ""
}

// describeAttribution writes the attribution the way an operator reads a price
// list: the option, then the signed amount it adds. The amount drops a trailing
// `.00` so the common whole-currency case reads `XL +1200` rather than
// `XL +1200.00`.
func describeAttribution(pair axisPair, amount float64) string {
	sign := "+"
	if amount < 0 {
		sign = "-"
	}
	formatted := strconv.FormatFloat(math.Abs(amount), 'f', -1, 64)
	return strings.TrimSpace(pair.Option) + " " + sign + formatted
}

// optionKey is how both per-option ledgers are keyed: the axis and the option,
// lowercased, so the same option written two ways is one option.
func optionKey(pair axisPair) string {
	return strings.ToLower(strings.TrimSpace(pair.Axis)) + "=" + strings.ToLower(strings.TrimSpace(pair.Option))
}

// combinationKeyOf is a row's identity under D9: the option set it names,
// order-independent. A row naming no axis is the product itself, and its key is
// the empty string — which is also what makes two rows for the same simple
// product a duplicate.
func combinationKeyOf(pairs []axisPair) string {
	keys := make([]string, 0, len(pairs))
	for _, pair := range pairs {
		keys = append(keys, optionKey(pair))
	}
	sort.Strings(keys)
	return strings.Join(keys, ";")
}

// axisNamesOf is the row's axis set, lowercased and in column order.
func axisNamesOf(pairs []axisPair) []string {
	names := make([]string, 0, len(pairs))
	for _, pair := range pairs {
		names = append(names, strings.ToLower(pair.Axis))
	}
	return names
}

// optionOfAxis finds which option of one axis a row carries.
func optionOfAxis(pairs []axisPair, axis string) (string, bool) {
	for _, pair := range pairs {
		if strings.EqualFold(pair.Axis, axis) {
			return pair.Option, true
		}
	}
	return "", false
}

// hasHeaders reports whether every named header is present, which is how a
// descriptor claims a CSV: a subset test, so extra columns never stop a match.
func hasHeaders(headers []string, required ...string) bool {
	present := make(map[string]bool, len(headers))
	for _, header := range headers {
		present[header] = true
	}

	for _, header := range required {
		if !present[header] {
			return false
		}
	}
	return true
}

// performedByString is the acting user as the movement and stock services take
// it: a string id, or nil when the run has no actor.
func performedByString(ctx importModels.ImportContext) *string {
	if ctx.PerformedBy == nil {
		return nil
	}
	value := ctx.PerformedBy.String()
	return &value
}

// logImportRun is the RecordRun of every inventory descriptor: the module keeps
// no audit trail of its own, and the engine ignores the outcome either way.
func logImportRun(meta importModels.RunMeta) {
	log.Printf("📥 import %s: %d rows — %d created, %d skipped, %d failed, %d warnings",
		meta.Resource, meta.Total, meta.Created, meta.Skipped, meta.Failed, meta.Warnings)
}
