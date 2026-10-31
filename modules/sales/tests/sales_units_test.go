//go:build integration
// +build integration

package sales_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	coreTestHelpers "josex/web/modules/core/testhelpers"
)

// INV-023: a line keeps the presentation it was sold in; lots and stock move in base units.

type unitRow struct {
	ID     string `json:"id"`
	UnitID string `json:"unit_id"`
}

// boxedProduct gives a product a base presentation and a "Caja x 12" with no price of its own, and returns both ids.
func boxedProduct(t *testing.T, helper *coreTestHelpers.ApiTestHelper, label string) (productID, baseID, boxID string) {
	t.Helper()
	productID = newSalesProduct(t, helper, label)
	w := helper.DoRequest("GET", "/inventory/units", nil, map[string]string{})
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
	w = helper.DoRequest("GET", "/inventory/products/"+productID, nil, map[string]string{})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var product struct {
		Units []unitRow `json:"units"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &product))
	body := map[string]interface{}{"units": []map[string]interface{}{
		{"id": product.Units[0].ID, "unit_id": product.Units[0].UnitID, "factor": 1, "is_base": true, "is_sale_default": true},
		{"unit_id": box, "factor": 12},
	}}
	w = helper.DoRequest("PUT", "/inventory/products/"+productID+"/units", body, map[string]string{})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var saved struct {
		Units []unitRow `json:"units"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &saved))
	return productID, saved.Units[0].ID, saved.Units[1].ID
}

func addLine(helper *coreTestHelpers.ApiTestHelper, orderID string, body map[string]interface{}) (int, string) {
	w := helper.DoRequest("POST", fmt.Sprintf("/sales/orders/%s/items", orderID), body, map[string]string{})
	return w.Code, w.Body.String()
}

func TestOrderLines_SellInPresentations(t *testing.T) {
	helper := SetupSalesAuthTest(t)
	defer helper.Close()
	productID, baseID, boxID := boxedProduct(t, helper, "UNITS")
	stockBatchFor(t, helper, productID, 24)
	orderID := newSalesOrder(t, helper)

	code, body := addLine(helper, orderID, map[string]interface{}{"product_id": productID, "quantity": 1, "product_unit_id": boxID})
	require.Contains(t, []int{http.StatusOK, http.StatusCreated}, code, body)
	var line struct {
		Data struct {
			Quantity  float64 `json:"quantity"`
			UnitPrice float64 `json:"unit_price"`
			LineTotal float64 `json:"line_total"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal([]byte(body), &line))
	assert.Equal(t, 1.0, line.Data.Quantity, "the line keeps what was sold")
	assert.Equal(t, 120.0, line.Data.UnitPrice, "D3: base × factor")

	code, body = addLine(helper, orderID, map[string]interface{}{"product_id": productID, "quantity": 3, "product_unit_id": baseID})
	require.Contains(t, []int{http.StatusOK, http.StatusCreated}, code, body)

	w := helper.DoRequest("PATCH", fmt.Sprintf("/sales/orders/%s/complete", orderID), nil, map[string]string{})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	w = helper.DoRequest("GET", "/inventory/stock/"+productID, nil, map[string]string{})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var stock struct {
		Quantity float64 `json:"current_quantity"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &stock))
	assert.Equal(t, 9.0, stock.Quantity, "24 − 12 − 3, in base units")

	w = helper.DoRequest("GET", "/inventory/movements?movement_type=SALE&product_id="+productID, nil, map[string]string{})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var list struct {
		Movements []struct {
			Quantity float64 `json:"quantity"`
		} `json:"movements"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &list))
	require.Len(t, list.Movements, 2, "two SALE movements in base units")
	assert.ElementsMatch(t, []float64{-12, -3}, []float64{list.Movements[0].Quantity, list.Movements[1].Quantity})
}

func TestOrderLines_FractionOnlyWhereThePresentationAllowsIt(t *testing.T) {
	helper := SetupSalesAuthTest(t)
	defer helper.Close()
	productID, _, boxID := boxedProduct(t, helper, "FRACTION")
	stockBatchFor(t, helper, productID, 24)
	orderID := newSalesOrder(t, helper)

	code, body := addLine(helper, orderID, map[string]interface{}{"product_id": productID, "quantity": 0.5, "product_unit_id": boxID})
	assert.Equal(t, http.StatusUnprocessableEntity, code, body)
	assert.Contains(t, body, "inventory.unit.no-fraction")
}

func TestOrderLines_ReservationAcrossPresentationsIsInBaseUnits(t *testing.T) {
	helper := SetupSalesAuthTest(t)
	defer helper.Close()
	productID, _, boxID := boxedProduct(t, helper, "RESERVE")
	stockBatchFor(t, helper, productID, 13)
	orderID := newSalesOrder(t, helper)

	code, body := addLine(helper, orderID, map[string]interface{}{"product_id": productID, "quantity": 2, "product_unit_id": boxID})
	assert.Contains(t, body, "insufficient", "two boxes are 24 of the 13 there are: %d", code)
	code, body = addLine(helper, orderID, map[string]interface{}{"product_id": productID, "quantity": 1, "product_unit_id": boxID})
	assert.Contains(t, []int{http.StatusOK, http.StatusCreated}, code, body)
}

func TestOrderLines_ARemovedPresentationStaysOnTheOldOrder(t *testing.T) {
	helper := SetupSalesAuthTest(t)
	defer helper.Close()
	productID, baseID, boxID := boxedProduct(t, helper, "REMOVED")
	stockBatchFor(t, helper, productID, 24)
	orderID := newSalesOrder(t, helper)
	code, body := addLine(helper, orderID, map[string]interface{}{"product_id": productID, "quantity": 1, "product_unit_id": boxID})
	require.Contains(t, []int{http.StatusOK, http.StatusCreated}, code, body)

	w := helper.DoRequest("GET", "/inventory/products/"+productID, nil, map[string]string{})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var product struct {
		Units []unitRow `json:"units"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &product))
	w = helper.DoRequest("PUT", "/inventory/products/"+productID+"/units", map[string]interface{}{"units": []map[string]interface{}{
		{"id": baseID, "unit_id": product.Units[0].UnitID, "factor": 1, "is_base": true},
	}}, map[string]string{})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var saved struct {
		Units []unitRow `json:"units"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &saved))
	assert.Len(t, saved.Units, 1, "gone from the product")

	w = helper.DoRequest("GET", fmt.Sprintf("/sales/orders/%s", orderID), nil, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), productID)
	w = helper.DoRequest("PATCH", fmt.Sprintf("/sales/orders/%s/complete", orderID), nil, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code, "the old line still completes: %s", w.Body.String())
}
