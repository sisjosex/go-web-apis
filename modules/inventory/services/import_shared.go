package services

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/google/uuid"

	coreServices "josex/web/modules/core/services"
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
	dbService coreServices.DatabaseService,
	tenantID uuid.UUID,
	sku string,
	pairs []axisPair,
) (skuCandidate, string) {
	sku = strings.TrimSpace(sku)
	if dbService == nil {
		return skuCandidate{}, inventoryErrors.ImportSkuUnknown + errorParamSeparator + sku
	}

	axisNames := make([]string, 0, len(pairs))
	optionNames := make([]string, 0, len(pairs))
	for _, pair := range pairs {
		axisNames = append(axisNames, pair.Axis)
		optionNames = append(optionNames, pair.Option)
	}

	// One round trip answers all of it: which product the cell names, how many
	// axes it has, the SKU whose option set equals the row's, and the default
	// bucket for the no-axis case.
	query := `
        WITH target AS (
            SELECT p.id AS product_id
            FROM inventory.products p
            WHERE p.tenant_id = $1
              AND LOWER(p.sku) = LOWER($2)
            UNION
            SELECT s.product_id
            FROM inventory.product_skus s
            WHERE s.tenant_id = $1
              AND LOWER(s.sku) = LOWER($2)
        ),
        wanted AS (
            SELECT ARRAY(
                SELECT LOWER(TRIM(w.axis)) || '=' || LOWER(TRIM(w.option))
                FROM unnest($3::TEXT[], $4::TEXT[]) AS w(axis, option)
                ORDER BY 1
            ) AS options
        ),
        matched AS (
            SELECT s.id::TEXT AS sku_id, s.sku
            FROM inventory.product_skus s
            JOIN target t ON t.product_id = s.product_id
            WHERE NOT s.is_default
              AND ARRAY(
                      SELECT LOWER(g.group_type) || '=' || LOWER(vo.option_name)
                      FROM inventory.product_sku_options so
                      JOIN inventory.product_variant_groups g
                        ON g.id = so.variant_group_id
                      JOIN inventory.product_variant_options vo
                        ON vo.id = so.option_id
                      WHERE so.sku_id = s.id
                      ORDER BY 1
                  ) = (SELECT w.options FROM wanted w)
        )
        SELECT (SELECT t.product_id::TEXT FROM target t LIMIT 1),
               (SELECT COUNT(*)
                FROM inventory.product_variant_groups vg
                JOIN target t ON t.product_id = vg.product_id
                WHERE vg.affects_inventory),
               (SELECT m.sku_id FROM matched m LIMIT 1),
               (SELECT m.sku FROM matched m LIMIT 1),
               (SELECT s.id::TEXT
                FROM inventory.product_skus s
                JOIN target t ON t.product_id = s.product_id
                WHERE s.is_default
                LIMIT 1),
               (SELECT s.sku
                FROM inventory.product_skus s
                JOIN target t ON t.product_id = s.product_id
                WHERE s.is_default
                LIMIT 1)
    `

	var productID, matchedID, matchedSku, defaultID, defaultSku *string
	var axisCount int

	row := dbService.QueryRow(context.Background(), query, tenantID, sku, axisNames, optionNames)
	if err := row.Scan(&productID, &axisCount, &matchedID, &matchedSku, &defaultID, &defaultSku); err != nil {
		return skuCandidate{}, inventoryErrors.ImportSkuUnknown + errorParamSeparator + sku
	}

	if productID == nil {
		return skuCandidate{}, inventoryErrors.ImportSkuUnknown + errorParamSeparator + sku
	}

	if len(pairs) < axisCount {
		return skuCandidate{}, inventoryErrors.ImportAxesIncomplete + errorParamSeparator + sku
	}

	if len(pairs) == 0 {
		if defaultID == nil {
			return skuCandidate{}, inventoryErrors.ImportSkuUnknown + errorParamSeparator + sku
		}
		return skuCandidate{ProductID: *productID, SkuID: *defaultID, SKU: *defaultSku}, ""
	}

	if matchedID == nil {
		return skuCandidate{}, inventoryErrors.ImportCombinationUnknown + errorParamSeparator + formatAxisPairs(pairs)
	}

	return skuCandidate{ProductID: *productID, SkuID: *matchedID, SKU: *matchedSku}, ""
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

// findVariantOptionID resolves one option of one axis of a product to the id
// product_media.variant_option_id references (D7). The axis and the option are
// matched case-insensitively, the way every other cell of these files is.
func findVariantOptionID(dbService coreServices.DatabaseService, productID, axis, option string) (string, bool) {
	if dbService == nil || productID == "" {
		return "", false
	}

	query := `
        SELECT vo.id::TEXT
        FROM inventory.product_variant_options vo
        JOIN inventory.product_variant_groups vg ON vg.id = vo.variant_group_id
        WHERE vg.product_id = $1::UUID
          AND vg.affects_inventory
          AND LOWER(vg.group_type)  = LOWER($2)
          AND LOWER(vo.option_name) = LOWER($3)
        LIMIT 1
    `

	optionID := ""
	row := dbService.QueryRow(context.Background(), query, productID, strings.TrimSpace(axis), strings.TrimSpace(option))
	if err := row.Scan(&optionID); err != nil {
		return "", false
	}
	return optionID, optionID != ""
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
func findFreeSlug(dbService coreServices.DatabaseService, tenantID uuid.UUID, base string) string {
	query := `
        SELECT c.slug
        FROM inventory.product_categories c
        WHERE c.tenant_id = $1
          AND (c.slug = $2 OR c.slug LIKE $2 || '-%')
    `

	rows, err := dbService.Query(context.Background(), query, tenantID, base)
	if err != nil {
		return base
	}
	defer rows.Close()

	taken := make(map[string]bool)
	for rows.Next() {
		var slug string
		if err := rows.Scan(&slug); err != nil {
			return base
		}
		taken[slug] = true
	}
	if rows.Err() != nil {
		return base
	}

	if !taken[base] {
		return base
	}
	for suffix := 2; suffix <= slugCollisionLimit; suffix++ {
		candidate := base + "-" + strconv.Itoa(suffix)
		if !taken[candidate] {
			return candidate
		}
	}
	return base
}

// findCategoryID returns the id of the tenant category with this name, matched
// case-insensitively. parentID narrows the search to the children of one
// category, which is exactly what a `subcategory` cell means.
func findCategoryID(dbService coreServices.DatabaseService, tenantID uuid.UUID, name string, parentID *string) (string, bool) {
	name = strings.TrimSpace(name)
	if name == "" || dbService == nil {
		return "", false
	}

	query := `
        SELECT c.id::TEXT
        FROM inventory.product_categories c
        WHERE c.tenant_id = $1
          AND LOWER(c.name) = LOWER($2)
          AND c.deleted_at IS NULL
          AND ($3::UUID IS NULL OR c.parent_id = $3::UUID)
        ORDER BY c.created_at
        LIMIT 1
    `

	categoryID := ""
	row := dbService.QueryRow(context.Background(), query, tenantID, name, parentID)
	if err := row.Scan(&categoryID); err != nil {
		return "", false
	}
	return categoryID, categoryID != ""
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
		if _, err := movementService.RecordMovement(context.Background(), ctx.TenantID, dto, performedByString(ctx)); err != nil {
			return inventoryErrors.ImportStockFailed, warnings
		}
	}

	if level, given, err := parseOptionalNumber(reorderCell, 0); err == nil && given && stockService != nil {
		if _, err := stockService.UpdateReorderLevel(
			context.Background(), ctx.TenantID, target.ProductID, level, performedByString(ctx), &skuID,
		); err != nil {
			warnings = append(warnings, inventoryErrors.ImportStockFailed)
		}
	}

	return "", warnings
}
