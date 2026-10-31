//go:build integration
// +build integration

package inventory_test

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"josex/web/modules/inventory/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// INV-021: the ledger folds a document into one row, the kardex and the file never do.

func ledger(t *testing.T, get func(string) (int, []byte), query string) models.ListMovementsResponse {
	t.Helper()
	code, body := get("/inventory/movements?" + query)
	require.Equal(t, http.StatusOK, code, string(body))
	var list models.ListMovementsResponse
	require.NoError(t, json.Unmarshal(body, &list))
	return list
}

// TestLedger_NamesAndKardex - a lot writes its label; rows carry names; a product's kardex is per movement.
func TestLedger_NamesAndKardex(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()
	get := func(path string) (int, []byte) {
		w := helper.DoRequest("GET", path, nil, nil)
		return w.Code, w.Body.Bytes()
	}
	productID := blockerProduct(t, helper, "LEDGER")
	batch := blockerBatch(t, helper, productID, 4)
	w := helper.DoRequest("POST", "/inventory/movements", map[string]interface{}{"product_id": productID, "movement_type": "WASTE", "quantity": 1, "notes": "roto"}, nil)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	list := ledger(t, get, "product_id="+productID)
	require.Len(t, list.Movements, 2)
	assert.Equal(t, "WASTE", list.Movements[0].MovementType)
	assert.NotEmpty(t, list.Movements[0].ProductName)
	require.NotNil(t, list.Movements[0].CreatedByName)
	require.NotNil(t, list.Movements[1].ReferenceLabel)
	assert.Equal(t, "Lote "+batch.LotNumber, *list.Movements[1].ReferenceLabel)

	found := ledger(t, get, "search=LEDGER&movement_type=PURCHASE,WASTE&page_size=100")
	assert.GreaterOrEqual(t, found.TotalCount, int64(2))
	none := ledger(t, get, "search=LEDGER&movement_type=SALE&page_size=100")
	assert.Empty(t, none.Movements)
}

// TestLedger_FoldsADocument - one transfer of a lot onto two combinations is one row with what moved.
func TestLedger_FoldsADocument(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()
	get := func(path string) (int, []byte) {
		w := helper.DoRequest("GET", path, nil, nil)
		return w.Code, w.Body.Bytes()
	}
	productID := blockerProduct(t, helper, "FOLD")
	batch := blockerBatch(t, helper, productID, 6)
	w := helper.DoRequest("PUT", "/inventory/products/"+productID, map[string]interface{}{
		"name": "Fold", "base_price": 10,
		"variants": map[string]interface{}{"groups": []map[string]interface{}{{
			"group_type": "Talla", "is_required": true, "max_selections": 1, "affects_inventory": true,
			"options": []map[string]interface{}{{"name": "S", "modifier": 0}, {"name": "M", "modifier": 0}},
		}}},
	}, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	w = helper.DoRequest("POST", "/inventory/products/"+productID+"/skus/generate", nil, nil)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	w = helper.DoRequest("GET", "/inventory/products/"+productID+"/skus", nil, nil)
	var skus models.ListProductSkusResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &skus))
	var targets []map[string]interface{}
	for _, sku := range skus.Skus {
		if !sku.IsDefault {
			targets = append(targets, map[string]interface{}{"sku_id": sku.SkuID, "quantity": 3})
		}
	}
	require.Len(t, targets, 2)
	w = helper.DoRequest("POST", "/inventory/products/"+productID+"/batches/redistribute", map[string]interface{}{"batch_id": batch.ID.String(), "targets": targets}, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	list := ledger(t, get, fmt.Sprintf("search=%s&movement_type=TRANSFER&page_size=100", "FOLD"))

	var rows []models.InventoryMovement
	for _, row := range list.Movements {
		if row.ProductID == productID {
			rows = append(rows, row)
		}
	}
	require.Len(t, rows, 1)
	assert.Equal(t, 2, rows[0].MoreCount)
	assert.Equal(t, 6.0, rows[0].Quantity)
	kardex := ledger(t, get, "product_id="+productID+"&movement_type=TRANSFER")
	assert.Len(t, kardex.Movements, 3)
}

func readCSV(t *testing.T, body []byte) [][]string {
	t.Helper()
	require.True(t, bytes.HasPrefix(body, []byte{0xEF, 0xBB, 0xBF}), "the file starts with a BOM")
	rows, err := csv.NewReader(bytes.NewReader(body[3:])).ReadAll()
	require.NoError(t, err)
	return rows
}

// TestExport_FilesFollowTheFilters - each download answers CSV with the screen's filters.
func TestExport_FilesFollowTheFilters(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()
	productID := blockerProduct(t, helper, "EXPORTME")
	blockerBatch(t, helper, productID, 2)
	w := helper.DoRequest("POST", "/inventory/movements", map[string]interface{}{"product_id": productID, "movement_type": "SALE", "quantity": 1}, nil)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	w = helper.DoRequest("GET", "/inventory/export/products?search=EXPORTME", nil, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Header().Get("Content-Disposition"), `filename="products-`)
	assert.Equal(t, "text/csv; charset=utf-8", w.Header().Get("Content-Type"))
	rows := readCSV(t, w.Body.Bytes())
	require.Len(t, rows, 2, "a header and the one product")
	assert.Contains(t, rows[1][0], "EXPORTME")

	w = helper.DoRequest("GET", "/inventory/export/movements?product_id="+productID, nil, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Len(t, readCSV(t, w.Body.Bytes()), 3, "a header and one line per movement")

	w = helper.DoRequest("GET", "/inventory/export/batches?product_id="+productID, nil, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Len(t, readCSV(t, w.Body.Bytes()), 2)

	w = helper.DoRequest("GET", "/inventory/export/batches?warning_days=60", nil, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	w = helper.DoRequest("GET", "/inventory/export/batches?warning_days=x", nil, nil)
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())

	member := asMember(t, helper)
	defer member.Close()
	w = member.DoRequest("GET", "/inventory/export/products", nil, nil)
	assert.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
}
