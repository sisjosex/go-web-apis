//go:build integration
// +build integration

package inventory_test

import (
	"encoding/json"
	"net/http"
	"testing"

	coreTestHelpers "josex/web/modules/core/testhelpers"
	"josex/web/modules/inventory/models"
	inventoryPerms "josex/web/modules/inventory/permissions"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// INV-026: a count compares with the stock when each line is counted, and closes into adjustments.

func countDetail(t *testing.T, code int, body []byte) models.CountDetail {
	t.Helper()
	require.Contains(t, []int{http.StatusOK, http.StatusCreated}, code, string(body))
	var out models.CountDetail
	require.NoError(t, json.Unmarshal(body, &out))
	return out
}

func newCount(t *testing.T, helper *coreTestHelpers.ApiTestHelper, blind bool) models.CountDetail {
	t.Helper()
	w := helper.DoRequest("POST", "/inventory/counts", map[string]interface{}{"blind": blind}, nil)
	return countDetail(t, w.Code, w.Body.Bytes())
}

func TestCounts_ASaleDuringACountIsNotMissing(t *testing.T) {
	helper := ownerOfAnotherTenant(t)
	id := newUnitsProduct(t, helper)
	sku := defaultSku(t, helper, id)
	buyInto(t, helper, id, "", 6, 5)

	count := newCount(t, helper, false)
	assert.Contains(t, count.Number, "C-")
	require.Len(t, count.Lines, 1)
	assert.Nil(t, count.Lines[0].Counted)
	base := "/inventory/counts/" + count.ID

	w := helper.DoRequest("POST", base+"/close", nil, nil)
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code, "not all counted: %s", w.Body.String())

	w = helper.DoRequest("PATCH", base+"/lines", map[string]interface{}{"lines": []map[string]interface{}{{"sku_id": sku, "counted": 5}}}, nil)
	count = countDetail(t, w.Code, w.Body.Bytes())
	assert.Equal(t, 6.0, *count.Lines[0].Expected, "the stock when it was counted")
	assert.Equal(t, -1.0, *count.Lines[0].Difference)
	assert.Equal(t, -5.0, *count.Progress.DifferenceValue, "at cost")

	w = helper.DoRequest("POST", "/inventory/movements", map[string]interface{}{"product_id": id, "movement_type": "SALE", "quantity": 1}, nil)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	w = helper.DoRequest("POST", base+"/close", nil, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "A-")
	assert.Equal(t, 4.0, stockRow(t, helper, id, "").Total)

	w = helper.DoRequest("POST", base+"/close", nil, nil)
	assert.Equal(t, http.StatusConflict, w.Code, "a second close: %s", w.Body.String())
	assert.Contains(t, w.Body.String(), "inventory.count.invalid-state")

	w = helper.DoRequest("GET", "/inventory/movements?movement_type=ADJUSTMENT&product_id="+id, nil, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var list models.ListMovementsResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &list))
	require.Len(t, list.Movements, 1)
	assert.Equal(t, -1.0, list.Movements[0].Quantity)
	assert.Contains(t, *list.Movements[0].ReferenceLabel, "Conteo #"+count.Number)

	// The list and its chips.
	w = helper.DoRequest("GET", "/inventory/counts?status=closed", nil, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var counts models.ListCountsResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &counts))
	require.Len(t, counts.Counts, 1)
	assert.EqualValues(t, 1, counts.Chips.Closed)
	assert.EqualValues(t, 1, counts.Counts[0].WithDifference)
}

func TestCounts_OneOpenPerScopeAndItsSteps(t *testing.T) {
	helper := ownerOfAnotherTenant(t)
	id := newUnitsProduct(t, helper)
	sku := defaultSku(t, helper, id)
	buyInto(t, helper, id, "", 3, 5)
	count := newCount(t, helper, false)
	base := "/inventory/counts/" + count.ID

	w := helper.DoRequest("POST", "/inventory/counts", map[string]interface{}{}, nil)
	assert.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "inventory.count.already-open")

	w = helper.DoRequest("POST", base+"/pause", nil, nil)
	assert.Equal(t, "paused", countDetail(t, w.Code, w.Body.Bytes()).Status)
	w = helper.DoRequest("PATCH", base+"/lines", map[string]interface{}{"lines": []map[string]interface{}{{"sku_id": sku, "counted": 1}}}, nil)
	assert.Equal(t, http.StatusConflict, w.Code, "paused takes no count: %s", w.Body.String())
	w = helper.DoRequest("POST", base+"/resume", nil, nil)
	assert.Equal(t, "open", countDetail(t, w.Code, w.Body.Bytes()).Status)

	w = helper.DoRequest("POST", base+"/submit", nil, nil)
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code, "nothing counted yet: %s", w.Body.String())
	w = helper.DoRequest("PATCH", base+"/lines", map[string]interface{}{"lines": []map[string]interface{}{{"sku_id": sku, "counted": 3}}}, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	w = helper.DoRequest("POST", base+"/submit", nil, nil)
	assert.Equal(t, "review", countDetail(t, w.Code, w.Body.Bytes()).Status)

	w = helper.DoRequest("POST", base+"/cancel", nil, nil)
	assert.Equal(t, "cancelled", countDetail(t, w.Code, w.Body.Bytes()).Status)
	newCount(t, helper, false)
}

func TestCounts_ScanAddsAndCounts(t *testing.T) {
	helper := ownerOfAnotherTenant(t)
	id := newUnitsProduct(t, helper)
	sku := defaultSku(t, helper, id)
	category := newCategoryUnder(t, helper, "Otra", nil)

	w := helper.DoRequest("POST", "/inventory/counts", map[string]interface{}{"category_id": category}, nil)
	count := countDetail(t, w.Code, w.Body.Bytes())
	assert.Empty(t, count.Lines, "the product is outside the category")
	base := "/inventory/counts/" + count.ID

	detail := productDetail(t, helper, id)
	w = helper.DoRequest("POST", base+"/scan", map[string]interface{}{"code": detail.SKU}, nil)
	count = countDetail(t, w.Code, w.Body.Bytes())
	require.Len(t, count.Lines, 1, "a combination of the warehouse joins the count")
	assert.Equal(t, sku, count.Lines[0].SkuID)
	assert.Equal(t, 1.0, *count.Lines[0].Counted)

	w = helper.DoRequest("POST", base+"/scan", map[string]interface{}{"code": detail.SKU}, nil)
	assert.Equal(t, 2.0, *countDetail(t, w.Code, w.Body.Bytes()).Lines[0].Counted)

	w = helper.DoRequest("POST", base+"/scan", map[string]interface{}{"code": "NO-EXISTE"}, nil)
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "inventory.code.not-found")

	stranger := ownerOfAnotherTenant(t)
	w = stranger.DoRequest("GET", base, nil, nil)
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
}

func TestCounts_BlindHidesWhatWasExpectedFromWhoOnlyCounts(t *testing.T) {
	helper := SetupInventoryTest(t)
	id := newUnitsProduct(t, helper)
	sku := defaultSku(t, helper, id)
	buyInto(t, helper, id, "", 6, 5)

	w := helper.DoRequest("POST", "/inventory/counts", map[string]interface{}{"blind": true}, nil)
	if w.Code == http.StatusConflict {
		t.Skip("the shared tenant already has an open count")
	}
	count := countDetail(t, w.Code, w.Body.Bytes())
	base := "/inventory/counts/" + count.ID
	member := asMember(t, helper, inventoryPerms.StockRead, inventoryPerms.StockCount)

	w = member.DoRequest("PATCH", base+"/lines", map[string]interface{}{"lines": []map[string]interface{}{{"sku_id": sku, "counted": 5}}}, nil)
	seen := countDetail(t, w.Code, w.Body.Bytes())
	for _, line := range seen.Lines {
		assert.Nil(t, line.Expected)
		assert.Nil(t, line.Difference)
	}
	assert.Nil(t, seen.Progress.DifferenceValue)

	w = member.DoRequest("POST", base+"/close", nil, nil)
	assert.Equal(t, http.StatusForbidden, w.Code, "closing asks for stock:adjust: %s", w.Body.String())

	w = helper.DoRequest("GET", base, nil, nil)
	found := false
	for _, line := range countDetail(t, w.Code, w.Body.Bytes()).Lines {
		if line.SkuID == sku {
			found = true
			assert.Equal(t, 6.0, *line.Expected, "who closes sees it")
		}
	}
	assert.True(t, found)
	w = helper.DoRequest("POST", base+"/cancel", nil, nil)
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
}
