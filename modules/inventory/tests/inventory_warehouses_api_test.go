//go:build integration
// +build integration

package inventory_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	coreTestHelpers "josex/web/modules/core/testhelpers"
	"josex/web/modules/inventory/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// INV-025: stock per warehouse, what names none uses the default, and transfers move it in three steps.

func listWarehouses(t *testing.T, helper *coreTestHelpers.ApiTestHelper) []models.Warehouse {
	t.Helper()
	w := helper.DoRequest("GET", "/inventory/warehouses", nil, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var list models.ListWarehousesResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &list))
	return list.Warehouses
}

// newWarehouse creates one on a fresh tenant, whose plan's limit the shared test tenant would soon reach.
func newWarehouse(t *testing.T, helper *coreTestHelpers.ApiTestHelper, name string) string {
	t.Helper()
	w := helper.DoRequest("POST", "/inventory/warehouses", map[string]interface{}{"name": name, "kind": "store", "city": "Cochabamba"}, nil)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var created models.Warehouse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	return created.ID
}

func defaultWarehouse(t *testing.T, rows []models.Warehouse) models.Warehouse {
	t.Helper()
	found := []models.Warehouse{}
	for _, row := range rows {
		if row.IsDefault {
			found = append(found, row)
		}
	}
	require.Len(t, found, 1, "one default")
	return found[0]
}

func buyInto(t *testing.T, helper *coreTestHelpers.ApiTestHelper, productID, warehouseID string, quantity, cost float64) {
	t.Helper()
	body := map[string]interface{}{"product_id": productID, "movement_type": "PURCHASE", "quantity": quantity, "unit_cost": cost}
	if warehouseID != "" {
		body["warehouse_id"] = warehouseID
	}
	w := helper.DoRequest("POST", "/inventory/movements", body, nil)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
}

func stockRow(t *testing.T, helper *coreTestHelpers.ApiTestHelper, productID, query string) models.StockItem {
	t.Helper()
	w := helper.DoRequest("GET", "/inventory/stock?page_size=100&"+query, nil, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var list models.ListStockResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &list))
	for _, row := range list.Items {
		if row.ProductID == productID {
			return row
		}
	}
	t.Fatalf("no stock row for %s: %s", productID, w.Body.String())
	return models.StockItem{}
}

func quantityIn(row models.StockItem, warehouseID string) float64 {
	for _, cell := range row.ByWarehouse {
		if cell.WarehouseID == warehouseID {
			return cell.Quantity
		}
	}
	return 0
}

func TestWarehouses_StockLivesPerWarehouse(t *testing.T) {
	helper := ownerOfAnotherTenant(t)
	id := newUnitsProduct(t, helper)
	rows := listWarehouses(t, helper)
	require.Len(t, rows, 1, "Principal, created with the first product")
	principal := defaultWarehouse(t, rows)
	assert.Equal(t, "Principal", principal.Name)

	buyInto(t, helper, id, "", 10, 5)
	south := newWarehouse(t, helper, "Sur")
	buyInto(t, helper, id, south, 4, 5)

	row := stockRow(t, helper, id, "")
	assert.Equal(t, 10.0, quantityIn(row, principal.ID), "a purchase naming Sur leaves Principal as it was")
	assert.Equal(t, 4.0, quantityIn(row, south))
	assert.Equal(t, 14.0, row.Total)
	assert.Equal(t, 4.0, stockRow(t, helper, id, "warehouse_id="+south).Total)

	rows = listWarehouses(t, helper)
	assert.Len(t, rows, 2)
	defaultWarehouse(t, rows)
	for _, warehouse := range rows {
		if warehouse.ID == south {
			assert.Equal(t, 4.0, warehouse.Units)
			assert.Equal(t, 20.0, warehouse.Value)
		}
	}

	w := helper.DoRequest("PATCH", "/inventory/warehouses/"+south+"/status", map[string]interface{}{"status": "archived"}, nil)
	assert.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "inventory.warehouse.has-stock")
	w = helper.DoRequest("PATCH", "/inventory/warehouses/"+principal.ID+"/status", map[string]interface{}{"status": "archived"}, nil)
	assert.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "inventory.warehouse.default")

	w = helper.DoRequest("PUT", "/inventory/warehouses/"+south, map[string]interface{}{"name": "Sucursal Sur", "kind": "storage"}, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "Sucursal Sur")

	// Stock out of it leaves it free to archive, once the default moved elsewhere nothing changes for it.
	w = helper.DoRequest("POST", "/inventory/movements", map[string]interface{}{
		"product_id": id, "movement_type": "WASTE", "quantity": 4, "warehouse_id": south,
	}, nil)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	w = helper.DoRequest("PATCH", "/inventory/warehouses/"+south+"/default", nil, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, south, defaultWarehouse(t, listWarehouses(t, helper)).ID)
	w = helper.DoRequest("PATCH", "/inventory/warehouses/"+principal.ID+"/default", nil, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	w = helper.DoRequest("PATCH", "/inventory/warehouses/"+south+"/status", map[string]interface{}{"status": "archived"}, nil)
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	w = helper.DoRequest("POST", "/inventory/movements", map[string]interface{}{
		"product_id": id, "movement_type": "PURCHASE", "quantity": 1, "unit_cost": 1, "warehouse_id": south,
	}, nil)
	assert.Equal(t, http.StatusNotFound, w.Code, "an archived warehouse takes nothing: %s", w.Body.String())

	stranger := ownerOfAnotherTenant(t)
	w = stranger.DoRequest("PUT", "/inventory/warehouses/"+south, map[string]interface{}{"name": "x"}, nil)
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
}

func TestWarehouses_PlanLimit(t *testing.T) {
	helper := ownerOfAnotherTenant(t)
	newUnitsProduct(t, helper)
	newWarehouse(t, helper, "Segundo")
	w := helper.DoRequest("POST", "/inventory/warehouses", map[string]interface{}{"name": "Tercero"}, nil)
	assert.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "billing.limit-reached")
}

func TestStock_ChipsAndHome(t *testing.T) {
	helper := ownerOfAnotherTenant(t)
	id := newUnitsProduct(t, helper)
	principal := defaultWarehouse(t, listWarehouses(t, helper)).ID
	south := newWarehouse(t, helper, "Sur")
	buyInto(t, helper, id, principal, 30, 5)
	buyInto(t, helper, id, south, 2, 5)
	w := helper.DoRequest("PATCH", "/inventory/products/"+id+"/reorder-level", map[string]interface{}{"reorder_level": 3, "warehouse_id": south}, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	w = helper.DoRequest("GET", "/inventory/summary", nil, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var summary models.InventorySummary
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &summary))
	assert.EqualValues(t, 2, summary.WarehousesCount)
	assert.Equal(t, 160.0, summary.Value)
	require.NotEmpty(t, summary.Reorder.Items, w.Body.String())
	item := summary.Reorder.Items[0]
	assert.Equal(t, south, item.WarehouseID)
	assert.Equal(t, 1.0, item.Suggested, "up to the minimum, nothing sold")
	require.NotNil(t, item.TransferFrom, "Principal can spare it")
	assert.Equal(t, principal, item.TransferFrom.WarehouseID)

	// The counts behind a chip are the rows under it.
	lacking := newUnitsProduct(t, helper)
	w = helper.DoRequest("GET", "/inventory/stock?page_size=100", nil, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var all models.ListStockResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &all))
	w = helper.DoRequest("GET", "/inventory/stock?page_size=100&filter=out_of_stock", nil, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var out models.ListStockResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	assert.EqualValues(t, all.Counts.OutOfStock, len(out.Items))
	assert.Contains(t, w.Body.String(), lacking)
	w = helper.DoRequest("GET", "/inventory/stock?filter=nada", nil, nil)
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())

	w = helper.DoRequest("GET", "/inventory/stock/"+id+"/skus", nil, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var skus models.ListStockSkusResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &skus))
	require.Len(t, skus.Skus, 1)
	assert.Len(t, skus.Skus[0].ByWarehouse, 2)

	w = helper.DoRequest("GET", "/inventory/nav-badges", nil, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var badges models.InventoryNavBadges
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &badges))
	assert.GreaterOrEqual(t, badges.StockLow, int64(2))
}

func TestTransfers_PreparedShippedReceived(t *testing.T) {
	helper := ownerOfAnotherTenant(t)
	id := newUnitsProduct(t, helper)
	sku := defaultSku(t, helper, id)
	origin := defaultWarehouse(t, listWarehouses(t, helper)).ID
	destination := newWarehouse(t, helper, "Sur")
	buyInto(t, helper, id, origin, 10, 5)

	w := helper.DoRequest("POST", "/inventory/transfers", map[string]interface{}{
		"from_warehouse_id": origin, "to_warehouse_id": origin, "lines": []map[string]interface{}{{"sku_id": sku, "quantity": 6}},
	}, nil)
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code, w.Body.String())

	w = helper.DoRequest("POST", "/inventory/transfers", map[string]interface{}{
		"from_warehouse_id": origin, "to_warehouse_id": destination, "lines": []map[string]interface{}{{"sku_id": sku, "quantity": 60}},
	}, nil)
	assert.Equal(t, http.StatusConflict, w.Code, "more than there is: %s", w.Body.String())

	w = helper.DoRequest("POST", "/inventory/transfers", map[string]interface{}{
		"from_warehouse_id": origin, "to_warehouse_id": destination, "lines": []map[string]interface{}{{"sku_id": sku, "quantity": 6}},
	}, nil)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var transfer models.Transfer
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &transfer))
	assert.Equal(t, "prepared", transfer.Status)
	assert.Contains(t, transfer.Number, "T-")
	assert.Equal(t, 10.0, quantityIn(stockRow(t, helper, id, ""), origin), "prepared only reserves")

	w = helper.DoRequest("POST", "/inventory/movements", map[string]interface{}{"product_id": id, "movement_type": "SALE", "quantity": 5, "warehouse_id": origin}, nil)
	assert.Equal(t, http.StatusConflict, w.Code, "6 are set aside, 4 can leave: %s", w.Body.String())

	base := "/inventory/transfers/" + transfer.ID
	w = helper.DoRequest("POST", base+"/receive", map[string]interface{}{}, nil)
	assert.Equal(t, http.StatusConflict, w.Code, "not shipped yet: %s", w.Body.String())

	w = helper.DoRequest("POST", base+"/ship", map[string]interface{}{"carrier_note": "con Juan"}, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, 4.0, quantityIn(stockRow(t, helper, id, ""), origin), "origin −6")
	assert.Equal(t, 0.0, quantityIn(stockRow(t, helper, id, ""), destination))

	w = helper.DoRequest("POST", base+"/cancel", nil, nil)
	assert.Equal(t, http.StatusConflict, w.Code, "only while prepared: %s", w.Body.String())

	w = helper.DoRequest("POST", base+"/receive", map[string]interface{}{"lines": []map[string]interface{}{{"sku_id": sku, "received_quantity": 7}}}, nil)
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code, "more than was sent: %s", w.Body.String())

	w = helper.DoRequest("POST", base+"/receive", map[string]interface{}{"lines": []map[string]interface{}{{"sku_id": sku, "received_quantity": 5}}}, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &transfer))
	assert.Equal(t, "received", transfer.Status)
	assert.Len(t, transfer.Timeline, 3)
	assert.Equal(t, 5.0, quantityIn(stockRow(t, helper, id, ""), destination), "destination +5")

	w = helper.DoRequest("POST", base+"/receive", map[string]interface{}{}, nil)
	assert.Equal(t, http.StatusConflict, w.Code, "a second receive: %s", w.Body.String())

	w = helper.DoRequest("GET", fmt.Sprintf("/inventory/movements?movement_type=WASTE&product_id=%s", id), nil, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var waste models.ListMovementsResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &waste))
	require.Len(t, waste.Movements, 1)
	assert.Equal(t, -1.0, waste.Movements[0].Quantity)
	require.NotNil(t, waste.Movements[0].ReferenceLabel)
	assert.Contains(t, *waste.Movements[0].ReferenceLabel, "Traspaso #"+transfer.Number)

	w = helper.DoRequest("GET", "/inventory/transfers?status=received", nil, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), transfer.ID)
	w = helper.DoRequest("GET", base, nil, nil)
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())

	// A prepared one cancelled gives its reservation back.
	w = helper.DoRequest("POST", "/inventory/transfers", map[string]interface{}{
		"from_warehouse_id": origin, "to_warehouse_id": destination, "lines": []map[string]interface{}{{"sku_id": sku, "quantity": 4}},
	}, nil)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var second models.Transfer
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &second))
	w = helper.DoRequest("POST", "/inventory/transfers/"+second.ID+"/cancel", nil, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	w = helper.DoRequest("POST", "/inventory/movements", map[string]interface{}{"product_id": id, "movement_type": "SALE", "quantity": 4, "warehouse_id": origin}, nil)
	assert.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	stranger := ownerOfAnotherTenant(t)
	w = stranger.DoRequest("GET", base, nil, nil)
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
}
