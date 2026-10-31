//go:build integration
// +build integration

package purchasing_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A purchase in boxes enters stock in base units, at the cost per base unit (INV-023).
func TestPurchaseOrders_ReceiveInBoxes(t *testing.T) {
	helper := setupPurchasing(t)
	productID := createProduct(t, helper)

	w := helper.DoRequest("GET", "/inventory/units", nil, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var units struct {
		Units []struct {
			ID   string `json:"id"`
			Code string `json:"code"`
		} `json:"units"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &units))
	var box string
	for _, unit := range units.Units {
		if unit.Code == "CAJ" {
			box = unit.ID
		}
	}
	w = helper.DoRequest("GET", "/inventory/products/"+productID, nil, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	type row struct {
		ID     string `json:"id"`
		UnitID string `json:"unit_id"`
	}
	var product struct {
		Units []row `json:"units"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &product))
	w = helper.DoRequest("PUT", "/inventory/products/"+productID+"/units", map[string]interface{}{"units": []map[string]interface{}{
		{"id": product.Units[0].ID, "unit_id": product.Units[0].UnitID, "factor": 1, "is_base": true},
		{"unit_id": box, "factor": 12, "is_purchase_default": true},
	}}, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var saved struct {
		Units []row `json:"units"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &saved))

	poID := createOrder(t, helper, createSupplier(t, helper))
	base := "/purchasing/purchase-orders/" + poID
	line := map[string]interface{}{"product_id": productID, "quantity": 0.5, "unit_cost": 120, "product_unit_id": saved.Units[1].ID}
	w = helper.DoRequest("POST", base+"/items", line, nil)
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code, "half a box: %s", w.Body.String())

	// The purchase default is the box, named or not.
	w = helper.DoRequest("POST", base+"/items", map[string]interface{}{"product_id": productID, "quantity": 2, "unit_cost": 120}, nil)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	assert.Equal(t, 240.0, decode(t, w.Body.Bytes())["line_total"])

	require.Equal(t, http.StatusOK, helper.DoRequest("PATCH", base+"/approve", nil, nil).Code)
	w = helper.DoRequest("PATCH", base+"/receive", map[string]interface{}{}, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, 24.0, stockOf(t, helper, productID))

	w = helper.DoRequest("GET", "/inventory/movements?movement_type=PURCHASE&product_id="+productID, nil, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var list struct {
		Movements []struct {
			Quantity float64  `json:"quantity"`
			UnitCost *float64 `json:"unit_cost"`
		} `json:"movements"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &list))
	require.Len(t, list.Movements, 1)
	assert.Equal(t, 24.0, list.Movements[0].Quantity)
	if assert.NotNil(t, list.Movements[0].UnitCost) {
		assert.Equal(t, 10.0, *list.Movements[0].UnitCost, "120 per box of 12")
	}
}
