package services

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	importModels "josex/web/modules/import/models"
	inventoryErrors "josex/web/modules/inventory/errors"
	inventoryInterfaces "josex/web/modules/inventory/interfaces"
	inventoryModels "josex/web/modules/inventory/models"
)

const (
	// errorParamSeparator appends a value to a row code, for the codes whose
	// message quotes back what the cell said:
	// "inventory.import.category-unknown|Bebidas". The wizard splits on the first
	// one and interpolates the tail as {value}; a code without it is translated
	// as-is.
	errorParamSeparator = "|"

	// The D1 marker: a column named `variant[<axis>]` is an inventory axis and
	// nothing else ever is, which is what makes ignoring every other
	// unrecognised column safe. `image[<axis>]` and `image_alt[<axis>]` are the
	// same bracket naming the option a picture is of (D7).
	axisColumnPrefix     = "variant["
	imageColumnPrefix    = "image["
	imageAltColumnPrefix = "image_alt["
	axisColumnSuffix     = "]"

	// The two axes every template publishes, so the marker is visible in the
	// downloaded file without reading the docs (AC-14). They are samples: any
	// number of variant[…] columns is accepted and the axis is whatever the
	// bracket names. The three product files carry the same two.
	sampleFirstAxis  = "talle"
	sampleSecondAxis = "color"

	// skuMaxLength is what product_skus.sku holds (VARCHAR(50)). A code longer
	// than this is refused by the row rather than by a truncation nobody asked
	// for.
	skuMaxLength = 50

	// The named errors sp_create_product_sku_combination raises, mapped to row
	// codes by the caller. Matching on the message is how every SP failure in
	// this module is already told apart from a 500.
	skuCombinationExists = "inventory.skus.combination-exists"
	skuCollision         = "inventory.skus.sku-collision"
	skuCapExceeded       = "inventory.skus.cap-exceeded"

	// The shapes written for humans — template placeholder and schema hint, so
	// both sides of the contract move together. The two packed-cell formats
	// INV-015 published are gone with the cells (D4).
	imageNameFormat = "filename.jpg"
	dateFormat      = "YYYY-MM-DD"

	// dateLayout is the Go layout every date column of every inventory import
	// accepts.
	dateLayout = "2006-01-02"

	// productsMediaCategory is the media directory imported product images are
	// written to.
	productsMediaCategory = "products"

	// slugCollisionLimit caps how far the slug of an auto-created category is
	// suffixed before the row gives up: product_categories.slug is UNIQUE per
	// tenant, so two categories named the same under different parents collide.
	slugCollisionLimit = 20

	// pendingCategoriesKey is where a dry run keeps the categories its rows
	// would have created by the time the next row is read.
	pendingCategoriesKey = "inventory.pendingCategories"
)

// slugSeparatorPattern collapses everything that is not alphanumeric into the
// single '-' a category slug is made of.
var slugSeparatorPattern = regexp.MustCompile(`[^a-z0-9]+`)

// skuCandidate is the combination a row resolved to: the product it belongs to
// and the SKU an opening count, a lot or a picture then hangs off.
type skuCandidate struct {
	ProductID string
	SkuID     string
	SKU       string
}

// axisPair is one axis column of one row: the axis its marker names and the
// option its cell holds, already Title-cased and trimmed.
type axisPair struct {
	Axis   string
	Option string
}

// parseMarkerColumn reads a bracketed column marker — `variant[talle]`,
// `image[color]` — into the axis it names (D1, D7). It answers two questions,
// not one: attempted says the header carries the prefix at all, so a malformed
// marker is reported instead of being swallowed by the rule that ignores every
// unrecognised column; valid says it also closes on a non-empty name.
//
// Headers arrive lowercased from parseCSV, so the axis is Title-cased here and
// that is the name it is stored under.
func parseMarkerColumn(header, prefix string) (string, bool, bool) {
	header = strings.TrimSpace(header)
	if !strings.HasPrefix(header, prefix) {
		return "", false, false
	}

	inner := strings.TrimSuffix(strings.TrimPrefix(header, prefix), axisColumnSuffix)
	if !strings.HasSuffix(header, axisColumnSuffix) || strings.TrimSpace(inner) == "" {
		return "", true, false
	}
	return titleCaseAxis(inner), true, true
}

// titleCaseAxis capitalises the first rune and leaves the rest alone, so
// `variant[talle]` is the axis Talle and `variant[tipo de tela]` is Tipo de
// tela rather than Tipo De Tela.
func titleCaseAxis(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	runes := []rune(name)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

// axisColumns reads the `variant[<axis>]` columns of one row, in the order the
// file writes them (D1). Only a filled cell counts: a product's axis set is the
// set of markers its rows actually fill, so the same file gives COLA one axis
// and REMERA-BAS two.
//
// headers is what makes the order real — a row is a map by the time it gets
// here, and D3's attribution and the axis sort_order behind the derived code
// both read left to right. It returns the row code to report, empty when the
// row's axes are readable.
func axisColumns(headers []string, row map[string]string) ([]axisPair, string) {
	var pairs []axisPair
	seen := make(map[string]bool)

	for _, header := range headers {
		axis, attempted, valid := parseMarkerColumn(header, axisColumnPrefix)
		if !attempted {
			continue
		}
		if !valid {
			return nil, inventoryErrors.ImportAxisColumnInvalid + errorParamSeparator + header
		}
		if seen[strings.ToLower(axis)] {
			return nil, inventoryErrors.ImportAxisColumnInvalid + errorParamSeparator + header
		}
		seen[strings.ToLower(axis)] = true

		if option := strings.TrimSpace(row[header]); option != "" {
			pairs = append(pairs, axisPair{Axis: axis, Option: option})
		}
	}

	return pairs, ""
}

// formatAxisPairs writes a row's axes back the way the operator wrote them, for
// the messages that quote the combination they could not find.
func formatAxisPairs(pairs []axisPair) string {
	parts := make([]string, 0, len(pairs))
	for _, pair := range pairs {
		parts = append(parts, pair.Axis+"="+pair.Option)
	}
	return strings.Join(parts, ", ")
}

// resolveSkuByAxes finds the combination a row names: the SKU of the product
// `sku` identifies whose option set is exactly the row's pairs (D6). Exactly,
// not a subset — requiring every axis is what makes it a single match, so
// INV-015's "unambiguous subset" rule is gone and with it the case where a row
// silently landed on a nearest neighbour.
//
// A product with no axes takes no pairs and resolves to the default SKU
// sp_create_product_with_variants gives every product. The three ways a row can
// miss are told apart rather than collapsed: an unknown product is
// sku-unknown, a row naming fewer axes than the product has is axes-incomplete,
// and an option set the product does not carry is combination-unknown.
//
// It returns the row code to report, empty when the SKU resolved.
func resolveSkuByAxes(
	lookups inventoryInterfaces.ImportLookupRepository,
	ctx importModels.ImportContext,
	sku string,
	pairs []axisPair,
) (skuCandidate, string) {
	sku = strings.TrimSpace(sku)
	unknown := inventoryErrors.ImportSkuUnknown + errorParamSeparator + sku
	if lookups == nil {
		return skuCandidate{}, unknown
	}

	axisNames := make([]string, 0, len(pairs))
	optionNames := make([]string, 0, len(pairs))
	for _, pair := range pairs {
		axisNames = append(axisNames, pair.Axis)
		optionNames = append(optionNames, pair.Option)
	}

	resolved, err := lookups.ResolveSkuByAxes(ctx.Context(), ctx.TenantID, sku, axisNames, optionNames)
	if err != nil || resolved.ProductID == nil {
		return skuCandidate{}, unknown
	}

	if len(pairs) < resolved.AxisCount {
		return skuCandidate{}, inventoryErrors.ImportAxesIncomplete + errorParamSeparator + sku
	}

	if len(pairs) == 0 {
		if resolved.DefaultSkuID == nil || resolved.DefaultSku == nil {
			return skuCandidate{}, unknown
		}
		return skuCandidate{ProductID: *resolved.ProductID, SkuID: *resolved.DefaultSkuID, SKU: *resolved.DefaultSku}, ""
	}

	if resolved.MatchedSkuID == nil || resolved.MatchedSku == nil {
		return skuCandidate{}, inventoryErrors.ImportCombinationUnknown + errorParamSeparator + formatAxisPairs(pairs)
	}

	return skuCandidate{ProductID: *resolved.ProductID, SkuID: *resolved.MatchedSkuID, SKU: *resolved.MatchedSku}, ""
}

// axisSampleColumns is the pair of `variant[…]` columns every product template
// publishes (AC-14). One list, three descriptors: the catalogue file, the stock
// file and the lots file name a combination the same way (D6), so the operator
// meets the marker once.
func axisSampleColumns() []importModels.ColumnSpec {
	return []importModels.ColumnSpec{
		{Key: axisColumnPrefix + sampleFirstAxis + axisColumnSuffix, Type: importModels.ColumnTypeText},
		{Key: axisColumnPrefix + sampleSecondAxis + axisColumnSuffix, Type: importModels.ColumnTypeText},
	}
}

// findVariantOptionID resolves one option of one axis of the tenant's product to
// the id product_media.variant_option_id references (D7). The axis and the
// option are matched case-insensitively, the way every other cell of these files
// is.
func findVariantOptionID(
	lookups inventoryInterfaces.ImportLookupRepository,
	ctx importModels.ImportContext,
	productID, axis, option string,
) (string, bool) {
	if lookups == nil || productID == "" {
		return "", false
	}

	optionID, err := lookups.FindVariantOptionID(ctx.Context(), ctx.TenantID, productID, axis, option)
	if err != nil || optionID == nil {
		return "", false
	}
	return *optionID, true
}

// slugify turns a category name into the slug sp_create_category requires. A
// name with nothing alphanumeric in it still needs a slug, so it falls back to a
// fixed stem that findFreeSlug then makes unique.
func slugify(name string) string {
	slug := slugSeparatorPattern.ReplaceAllString(strings.ToLower(strings.TrimSpace(name)), "-")
	slug = strings.Trim(slug, "-")
	if slug == "" {
		return "category"
	}
	return slug
}

// findFreeSlug returns the first slug of the form base, base-2, base-3 … this
// tenant is not already using. product_categories.slug is UNIQUE per tenant, so
// two categories legitimately named the same under different parents would
// otherwise collide (D7).
func findFreeSlug(lookups inventoryInterfaces.ImportLookupRepository, ctx importModels.ImportContext, base string) string {
	if lookups == nil {
		return base
	}

	slug, err := lookups.FreeCategorySlug(ctx.Context(), ctx.TenantID, base, slugCollisionLimit)
	if err != nil || slug == "" {
		return base
	}
	return slug
}

// findCategoryID returns the id of the tenant category with this name, matched
// case-insensitively. parentID narrows the search to the children of one
// category, which is exactly what a `subcategory` cell means.
func findCategoryID(
	lookups inventoryInterfaces.ImportLookupRepository,
	ctx importModels.ImportContext,
	name string,
	parentID *string,
) (string, bool) {
	name = strings.TrimSpace(name)
	if name == "" || lookups == nil {
		return "", false
	}

	categoryID, err := lookups.FindCategoryID(ctx.Context(), ctx.TenantID, name, parentID)
	if err != nil || categoryID == nil {
		return "", false
	}
	return *categoryID, true
}

// pendingCategories is the dry run's stand-in for the categories the real run
// will have created by the time it reaches a row (INV-017). Nothing is written
// on a dry run, so without it every row naming a new category announced it
// again, and the child of a category created two rows up looked orphaned.
//
// A category is keyed by its path: the parent key, '/', the name lowercased. The
// parent key is "" at the top, the id of a parent that exists, or the path of a
// parent that is itself pending.
type pendingCategories struct {
	paths map[string]bool
	// names maps a name to the path of the first pending category carrying it,
	// which is what findCategoryID with no parent answers: the oldest one of
	// that name, under any parent.
	names map[string]string
}

// pendingCategoriesOf returns the run's ledger, creating it on first use.
func pendingCategoriesOf(ctx importModels.ImportContext) *pendingCategories {
	if existing, ok := ctx.Scratch[pendingCategoriesKey].(*pendingCategories); ok {
		return existing
	}

	pending := &pendingCategories{paths: map[string]bool{}, names: map[string]string{}}
	if ctx.Scratch != nil {
		ctx.Scratch[pendingCategoriesKey] = pending
	}
	return pending
}

func categoryPath(parentKey, name string) string {
	return parentKey + "/" + strings.ToLower(strings.TrimSpace(name))
}

// find mirrors findCategoryID against the ledger: under one parent, or under any
// parent when parentKey is "". It returns the path that stands in for the id.
func (p *pendingCategories) find(parentKey, name string) (string, bool) {
	if parentKey == "" {
		path, found := p.names[strings.ToLower(strings.TrimSpace(name))]
		return path, found
	}

	path := categoryPath(parentKey, name)
	return path, p.paths[path]
}

// add records a category the real run will create at this row.
func (p *pendingCategories) add(parentKey, name string) {
	path := categoryPath(parentKey, name)
	p.paths[path] = true

	key := strings.ToLower(strings.TrimSpace(name))
	if _, known := p.names[key]; !known {
		p.names[key] = path
	}
}

// parseOptionalNumber reads a numeric cell. An empty cell is "not given" and
// never an error; anything else has to be a number at or above the caller's
// bound. A decimal comma is accepted, because a spreadsheet in a Spanish locale
// writes one.
func parseOptionalNumber(cell string, minimum float64) (float64, bool, error) {
	cell = strings.TrimSpace(cell)
	if cell == "" {
		return 0, false, nil
	}

	value, err := strconv.ParseFloat(strings.ReplaceAll(cell, ",", "."), 64)
	if err != nil || value < minimum {
		return 0, false, fmt.Errorf("%q is not a number of at least %v", cell, minimum)
	}
	return value, true, nil
}

// seedCombination books an opening count on one SKU and sets its reorder level.
// The product_stock file and a continuation row of a products file mean exactly
// the same thing by it (Amendment A1), so both go through here rather than
// through two copies that could drift.
//
// It returns the row error code — empty when the quantity went in — and the
// warnings to attach. The reorder level is a warning and never an error: the
// quantity is already in by the time it runs, so a failure there degrades the
// row instead of losing the stock it just booked.
func seedCombination(
	movementService inventoryInterfaces.MovementService,
	stockService inventoryInterfaces.StockService,
	ctx importModels.ImportContext,
	target skuCandidate,
	quantityCell, reorderCell string,
) (string, []string) {
	var warnings []string
	skuID := target.SkuID

	quantity, given, err := parseOptionalNumber(quantityCell, 0)
	if err == nil && given && quantity > 0 && movementService != nil {
		direction := movementDirectionIn
		notes := importMovementNote
		dto := inventoryModels.RecordMovementDto{
			ProductID:    target.ProductID,
			SkuID:        &skuID,
			MovementType: openingCountMovement,
			Quantity:     quantity,
			Direction:    &direction,
			Notes:        &notes,
		}
		if _, err := movementService.RecordMovement(ctx.Context(), ctx.TenantID, dto, performedByString(ctx)); err != nil {
			return inventoryErrors.ImportStockFailed, warnings
		}
	}

	if level, given, err := parseOptionalNumber(reorderCell, 0); err == nil && given && stockService != nil {
		if _, err := stockService.UpdateReorderLevel(
			ctx.Context(), ctx.TenantID, target.ProductID, level, performedByString(ctx), &skuID,
		); err != nil {
			warnings = append(warnings, inventoryErrors.ImportStockFailed)
		}
	}

	return "", warnings
}
