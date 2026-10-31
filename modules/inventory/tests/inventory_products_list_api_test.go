//go:build integration
// +build integration

package inventory_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"josex/web/modules/core/testhelpers"
	"josex/web/modules/inventory/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// INV-019: the list's chips and rows, discontinue and reactivate, the bulk bar.

func queryProducts(t *testing.T, helper *testhelpers.ApiTestHelper, query string) models.ListProductsResponse {
	t.Helper()
	w := helper.DoRequest("GET", "/inventory/products?"+query, nil, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var list models.ListProductsResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &list))
	return list
}

func productIDs(list models.ListProductsResponse) []string {
	ids := make([]string, 0, len(list.Products))
	for _, p := range list.Products {
		ids = append(ids, p.ID)
	}
	return ids
}

// TestBulkUpdate_HidesInPosAndMovesTheChip - two rows hidden: updated 2, and the chip grows by 2.
func TestBulkUpdate_HidesInPosAndMovesTheChip(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()
	a, b := blockerProduct(t, helper, "BULKA"), blockerProduct(t, helper, "BULKB")
	before := queryProducts(t, helper, "page=1").Counts.PosHidden

	w := helper.DoRequest("PATCH", "/inventory/products/bulk", map[string]interface{}{"ids": []string{a, b}, "set": map[string]interface{}{"pos_visible": false}}, nil)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.JSONEq(t, `{"updated":2}`, w.Body.String())
	after := queryProducts(t, helper, "page=1&filter=pos_hidden")
	assert.Equal(t, before+2, after.Counts.PosHidden)
	assert.Contains(t, productIDs(after), a)
}

// TestBulkUpdate_PriceAndCategory - a percent change and the one category that replaces theirs.
func TestBulkUpdate_PriceAndCategory(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()
	id := blockerProduct(t, helper, "BULKPRICE")
	stamp := time.Now().UnixNano()
	w := helper.DoRequest("POST", "/inventory/categories", map[string]interface{}{"name": fmt.Sprintf("Bulk %d", stamp), "slug": fmt.Sprintf("bulk-%d", stamp)}, nil)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var category models.CategoryResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &category))

	w = helper.DoRequest("PATCH", "/inventory/products/bulk", map[string]interface{}{"ids": []string{id}, "set": map[string]interface{}{"price_change_pct": 10, "category_id": category.ID}}, nil)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	list := queryProducts(t, helper, "category_id="+category.ID)
	require.Len(t, list.Products, 1)
	assert.Equal(t, 11.0, list.Products[0].BasePrice)
	require.Len(t, list.Products[0].Categories, 1)
	assert.Equal(t, category.ID, list.Products[0].Categories[0].ID)

	w = helper.DoRequest("PATCH", "/inventory/products/bulk", map[string]interface{}{"ids": []string{id}, "set": map[string]interface{}{"category_id": "00000000-0000-4000-8000-000000000000"}}, nil)
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
}

// TestBulkUpdate_StatusNeedsDelete - a writer without the delete permission cannot discontinue in bulk.
func TestBulkUpdate_StatusNeedsDelete(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()
	id := blockerProduct(t, helper, "BULKPERM")
	writer := asMember(t, helper, "inventory:products:write")
	defer writer.Close()

	w := writer.DoRequest("PATCH", "/inventory/products/bulk", map[string]interface{}{"ids": []string{id}, "set": map[string]interface{}{"status": "discontinued"}}, nil)
	assert.Equal(t, http.StatusForbidden, w.Code, w.Body.String())

	w = writer.DoRequest("PATCH", "/inventory/products/bulk", map[string]interface{}{"ids": []string{id}, "set": map[string]interface{}{"pos_visible": false}}, nil)
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
}

// TestDiscontinue_LeavesTheDefaultListAndComesBack - D1: reversible, still readable.
func TestDiscontinue_LeavesTheDefaultListAndComesBack(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()
	id := blockerProduct(t, helper, "DISCONT")

	w := helper.DoRequest("PATCH", "/inventory/products/"+id+"/status", map[string]interface{}{"status": "discontinued"}, nil)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.JSONEq(t, fmt.Sprintf(`{"id":%q,"status":"discontinued"}`, id), w.Body.String())
	assert.NotContains(t, productIDs(queryProducts(t, helper, "page_size=100")), id)
	assert.Contains(t, productIDs(queryProducts(t, helper, "status=discontinued&page_size=100")), id)
	w = helper.DoRequest("GET", "/inventory/products/"+id, nil, nil)
	assert.Equal(t, http.StatusOK, w.Code, "a discontinued product still opens: %s", w.Body.String())

	w = helper.DoRequest("PATCH", "/inventory/products/"+id+"/status", map[string]interface{}{"status": "active"}, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, productIDs(queryProducts(t, helper, "page_size=100")), id)

	w = helper.DoRequest("PATCH", "/inventory/products/00000000-0000-4000-8000-000000000000/status", map[string]interface{}{"status": "active"}, nil)
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	w = helper.DoRequest("PATCH", "/inventory/products/"+id+"/status", map[string]interface{}{"status": "lost"}, nil)
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
}

// TestProductsList_RowCarriesStockAndAxes - a product with an axis: stock, axes, variants and flags.
func TestProductsList_RowCarriesStockAndAxes(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()
	id := blockerProduct(t, helper, "ROWSTOCK")
	blockerBatch(t, helper, id, 7)

	list := queryProducts(t, helper, "search=ROWSTOCK&page_size=100")

	var row *models.Product
	for i := range list.Products {
		if list.Products[i].ID == id {
			row = &list.Products[i]
		}
	}
	require.NotNil(t, row)
	assert.Equal(t, 7.0, row.AvailableQuantity)
	assert.NotEmpty(t, row.StockStatus)
	assert.Equal(t, 0, row.VariantsCount)
	assert.Equal(t, row.BasePrice, row.MinPrice)
	assert.True(t, row.PosVisible)
	assert.NotNil(t, row.Axes)
}

// TestProductsList_EmptyPageKeepsTheCounts - a chip with no rows still answers every count.
func TestProductsList_EmptyPageKeepsTheCounts(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()
	blockerProduct(t, helper, "EMPTYPAGE")

	list := queryProducts(t, helper, "search=EMPTYPAGE&filter=pos_hidden")

	assert.Empty(t, list.Products)
	assert.Equal(t, int64(0), list.TotalCount)
	assert.GreaterOrEqual(t, list.Counts.All, int64(1))

	w := helper.DoRequest("GET", "/inventory/products?filter=nope", nil, nil)
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
}

// TestReorderLevel_PerCombination - "Aplicar a todas: Mínimo" sets one combination's level by its id.
func TestReorderLevel_PerCombination(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()
	w := helper.DoRequest("POST", "/inventory/products", map[string]interface{}{
		"sku": fmt.Sprintf("MINSKU_%d", time.Now().UnixNano()), "name": "Min", "base_price": 10,
		"variants": map[string]interface{}{"groups": []map[string]interface{}{{
			"group_type": "Talla", "is_required": true, "max_selections": 1, "affects_inventory": true,
			"options": []map[string]interface{}{{"name": "S", "modifier": 0}, {"name": "M", "modifier": 0}},
		}}},
	}, nil)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var created models.CreateProductResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	w = helper.DoRequest("GET", "/inventory/products/"+created.ProductID+"/skus", nil, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var skus models.ListProductSkusResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &skus))
	var target models.ProductSku
	for _, sku := range skus.Skus {
		if !sku.IsDefault {
			target = sku
		}
	}
	require.NotEmpty(t, target.SkuID)

	w = helper.DoRequest("PATCH", "/inventory/products/"+created.ProductID+"/reorder-level", map[string]interface{}{"reorder_level": 3, "sku_id": target.SkuID}, nil)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	w = helper.DoRequest("GET", "/inventory/products/"+created.ProductID+"/skus", nil, nil)
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &skus))
	for _, sku := range skus.Skus {
		if sku.SkuID == target.SkuID {
			assert.Equal(t, 3.0, sku.ReorderLevel)
		}
	}
}
