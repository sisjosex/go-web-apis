//go:build integration
// +build integration

package inventory_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	coreTestHelpers "josex/web/modules/core/testhelpers"
	"josex/web/modules/inventory/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// INV-027: a lot belongs to a warehouse, and what leaves a warehouse comes off its lots.

func newLot(t *testing.T, helper *coreTestHelpers.ApiTestHelper, productID, warehouseID, number string, quantity, cost float64, days int) {
	t.Helper()
	body := map[string]interface{}{
		"product_id": productID, "lot_number": number, "purchase_date": time.Now().AddDate(0, 0, -1).Format("2006-01-02"),
		"expiry_date": time.Now().AddDate(0, 0, days).Format("2006-01-02"), "unit_cost": cost, "initial_quantity": quantity,
	}
	if warehouseID != "" {
		body["warehouse_id"] = warehouseID
	}
	w := helper.DoRequest("POST", "/inventory/batches", body, nil)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
}

func lotsOf(t *testing.T, helper *coreTestHelpers.ApiTestHelper, productID, warehouseID string) map[string]float64 {
	t.Helper()
	path := "/inventory/batches/product/" + productID + "?page_size=100&onlyActive=false"
	if warehouseID != "" {
		path += "&warehouse_id=" + warehouseID
	}
	w := helper.DoRequest("GET", path, nil, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var list models.ListBatchesResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &list))
	out := map[string]float64{}
	for _, lot := range list.Batches {
		out[lot.LotNumber] = lot.CurrentQuantity
	}
	return out
}

func TestLots_BelongToAWarehouse(t *testing.T) {
	helper := ownerOfAnotherTenant(t)
	id := newUnitsProduct(t, helper)
	principal := defaultWarehouse(t, listWarehouses(t, helper)).ID
	south := newWarehouse(t, helper, "Sur")

	newLot(t, helper, id, "", "EN-PRINCIPAL", 2, 5, 30)
	newLot(t, helper, id, south, "EN-SUR", 3, 5, 10)
	assert.Equal(t, map[string]float64{"EN-PRINCIPAL": 2}, lotsOf(t, helper, id, principal), "without a warehouse it lands in the default")
	assert.Equal(t, map[string]float64{"EN-SUR": 3}, lotsOf(t, helper, id, south))
	assert.Equal(t, 3.0, quantityIn(stockRow(t, helper, id, ""), south), "the lot's units are that warehouse's stock")

	// The one that expires first is in Sur, and a sale from the default never picks it.
	w := helper.DoRequest("GET", "/inventory/batches/product/"+id+"/oldest", nil, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "EN-PRINCIPAL")
	w = helper.DoRequest("GET", "/inventory/batches/product/"+id+"/oldest?warehouse_id="+south, nil, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "EN-SUR")

	w = helper.DoRequest("GET", "/inventory/batches/expiring?warningDays=60&warehouse_id="+south, nil, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var expiring models.ListBatchesResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &expiring))
	require.Len(t, expiring.Batches, 1)
	require.NotNil(t, expiring.Batches[0].WarehouseName)
	assert.Equal(t, "Sur", *expiring.Batches[0].WarehouseName)
}

func TestLots_OutboundComesOffTheFirstToExpire(t *testing.T) {
	helper := ownerOfAnotherTenant(t)
	id := newUnitsProduct(t, helper)
	newLot(t, helper, id, "", "PRONTO", 2, 5, 10)
	newLot(t, helper, id, "", "TARDE", 5, 5, 40)

	w := helper.DoRequest("POST", "/inventory/movements", map[string]interface{}{"product_id": id, "movement_type": "WASTE", "quantity": 3}, nil)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	assert.Equal(t, map[string]float64{"PRONTO": 0, "TARDE": 4}, lotsOf(t, helper, id, ""))

	// Units beyond the lots come off stock only.
	moveStock(t, helper, id, "ADJUSTMENT", 10, nil)
	w = helper.DoRequest("POST", "/inventory/movements", map[string]interface{}{"product_id": id, "movement_type": "WASTE", "quantity": 9}, nil)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	assert.Equal(t, map[string]float64{"PRONTO": 0, "TARDE": 0}, lotsOf(t, helper, id, ""))
	assert.Equal(t, 5.0, stockRow(t, helper, id, "").Total)
}

func TestLots_NeverAboveStockUnderConcurrency(t *testing.T) {
	helper := ownerOfAnotherTenant(t)
	id := newUnitsProduct(t, helper)
	newLot(t, helper, id, "", "A", 5, 5, 10)
	newLot(t, helper, id, "", "B", 5, 5, 20)

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			helper.DoRequest("POST", "/inventory/movements", map[string]interface{}{"product_id": id, "movement_type": "WASTE", "quantity": 2}, nil)
		}()
	}
	wg.Wait()

	total := 0.0
	for _, quantity := range lotsOf(t, helper, id, "") {
		total += quantity
	}
	assert.Equal(t, stockRow(t, helper, id, "").Total, total, "lots and stock moved together")
}

func TestTransfers_CarryTheirLots(t *testing.T) {
	helper := ownerOfAnotherTenant(t)
	id := newUnitsProduct(t, helper)
	sku := defaultSku(t, helper, id)
	origin := defaultWarehouse(t, listWarehouses(t, helper)).ID
	destination := newWarehouse(t, helper, "Sur")
	newLot(t, helper, id, origin, "L1", 4, 5, 10)
	newLot(t, helper, id, origin, "L2", 5, 7, 40)

	transfer := func(quantity float64) string {
		w := helper.DoRequest("POST", "/inventory/transfers", map[string]interface{}{
			"from_warehouse_id": origin, "to_warehouse_id": destination, "lines": []map[string]interface{}{{"sku_id": sku, "quantity": quantity}},
		}, nil)
		require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
		var out models.Transfer
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
		return out.ID
	}
	base := "/inventory/transfers/" + transfer(6)
	w := helper.DoRequest("POST", base+"/ship", map[string]interface{}{}, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, map[string]float64{"L1": 0, "L2": 3}, lotsOf(t, helper, id, origin))

	var shipped models.Transfer
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &shipped))
	require.Len(t, shipped.Lines[0].Lots, 2)
	assert.Equal(t, "L1", shipped.Lines[0].Lots[0].LotNumber)
	assert.Equal(t, 4.0, shipped.Lines[0].Lots[0].Quantity)

	w = helper.DoRequest("POST", base+"/receive", map[string]interface{}{"lines": []map[string]interface{}{{"sku_id": sku, "received_quantity": 5}}}, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, map[string]float64{"L1": 4, "L2": 1}, lotsOf(t, helper, id, destination), "the shortfall comes off the last lot taken")
	assert.Equal(t, 5.0, quantityIn(stockRow(t, helper, id, ""), destination))

	// A second one lands on the same lots at the destination: same number, expiry and cost.
	second := "/inventory/transfers/" + transfer(1)
	w = helper.DoRequest("POST", second+"/ship", map[string]interface{}{}, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	w = helper.DoRequest("POST", second+"/receive", map[string]interface{}{}, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, map[string]float64{"L1": 4, "L2": 2}, lotsOf(t, helper, id, destination))

	// With no lots at the origin it moves stock only.
	plain := newUnitsProduct(t, helper)
	buyInto(t, helper, plain, origin, 3, 5)
	w = helper.DoRequest("POST", "/inventory/transfers", map[string]interface{}{
		"from_warehouse_id": origin, "to_warehouse_id": destination,
		"lines": []map[string]interface{}{{"sku_id": defaultSku(t, helper, plain), "quantity": 2}},
	}, nil)
	require.Equal(t, http.StatusCreated, w.Code, fmt.Sprint(w.Body.String()))
	var noLots models.Transfer
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &noLots))
	w = helper.DoRequest("POST", "/inventory/transfers/"+noLots.ID+"/ship", map[string]interface{}{}, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Equal(t, http.StatusOK, helper.DoRequest("POST", "/inventory/transfers/"+noLots.ID+"/receive", map[string]interface{}{}, nil).Code)
	assert.Empty(t, lotsOf(t, helper, plain, destination))
	assert.Equal(t, 2.0, quantityIn(stockRow(t, helper, plain, ""), destination))
}
