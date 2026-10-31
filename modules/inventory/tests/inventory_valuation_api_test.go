//go:build integration
// +build integration

package inventory_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	coreTestHelpers "josex/web/modules/core/testhelpers"
	"josex/web/modules/inventory/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// INV-022: stock × cost: the active lots' weighted average, else the entries', else no cost.

func valuationOf(t *testing.T, helper *coreTestHelpers.ApiTestHelper, query string) models.ValuationResponse {
	t.Helper()
	w := helper.DoRequest("GET", "/inventory/reports/valuation?page_size=100&"+query, nil, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var out models.ValuationResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	return out
}

func valuationRow(t *testing.T, out models.ValuationResponse, productID string) models.ValuationItem {
	t.Helper()
	for _, row := range out.Items {
		if row.ProductID == productID {
			return row
		}
	}
	t.Fatalf("no row for %s", productID)
	return models.ValuationItem{}
}

func moveStock(t *testing.T, helper *coreTestHelpers.ApiTestHelper, productID, kind string, quantity float64, cost interface{}) {
	t.Helper()
	body := map[string]interface{}{"product_id": productID, "movement_type": kind, "quantity": quantity}
	if kind == "ADJUSTMENT" {
		body["direction"] = "IN"
		if quantity < 0 {
			body["direction"], body["quantity"] = "OUT", -quantity
		}
	}
	if cost != nil {
		body["unit_cost"] = cost
	}
	w := helper.DoRequest("POST", "/inventory/movements", body, nil)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
}

func TestValuation_WeightedAverageOfEntries(t *testing.T) {
	helper := SetupInventoryTest(t)
	id := newUnitsProduct(t, helper)
	moveStock(t, helper, id, "PURCHASE", 10, 5)
	moveStock(t, helper, id, "PURCHASE", 10, 7)
	moveStock(t, helper, id, "SALE", 5, nil)

	row := valuationRow(t, valuationOf(t, helper, ""), id)
	assert.Equal(t, 15.0, row.Quantity)
	require.NotNil(t, row.Value)
	assert.Equal(t, 90.0, *row.Value)
	assert.Equal(t, 6.0, *row.UnitCost)
}

func TestValuation_LotsOverEntries(t *testing.T) {
	helper := SetupInventoryTest(t)
	id := newUnitsProduct(t, helper)
	for i, cost := range []float64{5, 7} {
		w := helper.DoRequest("POST", "/inventory/batches", map[string]interface{}{
			"product_id": id, "lot_number": fmt.Sprintf("VAL-%d-%d", i, time.Now().UnixNano()), "purchase_date": time.Now().Format("2006-01-02"),
			"expiry_date": time.Now().AddDate(0, 0, 30+i).Format("2006-01-02"), "unit_cost": cost, "initial_quantity": 10,
		}, nil)
		require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	}
	// The movement comes off the lot that expires first (INV-027): 5 at 5 and 10 at 7.
	moveStock(t, helper, id, "ADJUSTMENT", -5, nil)
	row := valuationRow(t, valuationOf(t, helper, ""), id)
	assert.Equal(t, 15.0, row.Quantity)
	require.NotNil(t, row.Value)
	assert.Equal(t, 95.0, *row.Value)
}

func TestValuation_NoCostIsMarkedAndOutOfTheTotal(t *testing.T) {
	helper := SetupInventoryTest(t)
	before := valuationOf(t, helper, "")
	costed := newUnitsProduct(t, helper)
	moveStock(t, helper, costed, "PURCHASE", 2, 10)
	free := newUnitsProduct(t, helper)
	moveStock(t, helper, free, "ADJUSTMENT", 4, nil)

	out := valuationOf(t, helper, "")
	row := valuationRow(t, out, free)
	assert.Nil(t, row.UnitCost)
	assert.Nil(t, row.Value)
	assert.Equal(t, before.UncostedCount+1, out.UncostedCount)
	assert.Equal(t, before.TotalValue+20, out.TotalValue)

	// Highest value first, what has no cost last.
	assert.NotNil(t, out.Items[0].Value)
	assert.Nil(t, out.Items[len(out.Items)-1].Value)

	empty := newUnitsProduct(t, helper)
	for _, item := range valuationOf(t, helper, "").Items {
		assert.NotEqual(t, empty, item.ProductID, "no stock, no row")
	}
}

func TestValuation_ByCategoryAndAcrossBusinesses(t *testing.T) {
	helper := SetupInventoryTest(t)
	id := newUnitsProduct(t, helper)
	moveStock(t, helper, id, "PURCHASE", 3, 4)
	category := newCategoryUnder(t, helper, "Valorizada", nil)
	w := helper.DoRequest("POST", "/inventory/products/"+id+"/categories/"+category, nil, nil)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	out := valuationOf(t, helper, "category_id="+category)
	require.Len(t, out.Items, 1)
	assert.Equal(t, 12.0, out.TotalValue)

	stranger := ownerOfAnotherTenant(t)
	foreign := valuationOf(t, stranger, "")
	assert.Empty(t, foreign.Items)
	assert.Zero(t, foreign.TotalValue)
	assert.Zero(t, foreign.TotalCount)

	w = helper.DoRequest("GET", "/inventory/export/valuation?category_id="+category, nil, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.True(t, strings.Contains(w.Body.String(), "12"), w.Body.String())
}
