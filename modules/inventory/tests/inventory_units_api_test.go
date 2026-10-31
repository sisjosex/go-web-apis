//go:build integration
// +build integration

package inventory_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	coreTestHelpers "josex/web/modules/core/testhelpers"
	"josex/web/modules/inventory/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// INV-023: a product sells and buys in presentations of one stock counted in its base unit.

func unitNamed(t *testing.T, helper *coreTestHelpers.ApiTestHelper, code string) string {
	t.Helper()
	w := helper.DoRequest("GET", "/inventory/units", nil, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var list models.ListUnitsResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &list))
	for _, unit := range list.Units {
		if unit.Code == code {
			return unit.ID
		}
	}
	t.Fatalf("no unit %s in %s", code, w.Body.String())
	return ""
}

func productDetail(t *testing.T, helper *coreTestHelpers.ApiTestHelper, productID string) models.ProductDetail {
	t.Helper()
	w := helper.DoRequest("GET", "/inventory/products/"+productID, nil, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var detail models.ProductDetail
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &detail))
	return detail
}

func defaultSku(t *testing.T, helper *coreTestHelpers.ApiTestHelper, productID string) string {
	t.Helper()
	w := helper.DoRequest("GET", "/inventory/products/"+productID+"/skus", nil, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var list models.ListProductSkusResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &list))
	require.NotEmpty(t, list.Skus)
	return list.Skus[0].SkuID
}

func newUnitsProduct(t *testing.T, helper *coreTestHelpers.ApiTestHelper) string {
	t.Helper()
	w := helper.DoRequest("POST", "/inventory/products", map[string]interface{}{
		"sku": fmt.Sprintf("UNITS_%d", time.Now().UnixNano()), "name": "Sold in boxes", "base_price": 10,
	}, nil)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var created struct {
		ProductID string `json:"product_id"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	return created.ProductID
}

func TestUnits_ProductStartsWithItsBasePresentation(t *testing.T) {
	helper := SetupInventoryTest(t)
	detail := productDetail(t, helper, newUnitsProduct(t, helper))
	require.Len(t, detail.Units, 1)
	assert.True(t, detail.Units[0].IsBase)
	assert.Equal(t, 1.0, detail.Units[0].Factor)
	assert.Equal(t, "UND", detail.Units[0].Code)
	assert.True(t, detail.Units[0].IsSaleDefault)
}

func TestUnits_CodeNamesThePresentation(t *testing.T) {
	helper := SetupInventoryTest(t)
	productID := newUnitsProduct(t, helper)
	detail := productDetail(t, helper, productID)
	skuID := defaultSku(t, helper, productID)
	code := fmt.Sprintf("777%d", time.Now().UnixNano())
	body := map[string]interface{}{"units": []map[string]interface{}{
		{"id": detail.Units[0].ID, "unit_id": detail.Units[0].UnitID, "factor": 1, "is_base": true, "is_sale_default": true, "is_purchase_default": true},
		{"unit_id": unitNamed(t, helper, "CAJ"), "factor": 12, "price": 100, "codes": []map[string]interface{}{{"sku_id": skuID, "code": code}}},
	}}
	w := helper.DoRequest("PUT", "/inventory/products/"+productID+"/units", body, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	saved := productDetail(t, helper, productID)
	require.Len(t, saved.Units, 2)
	assert.Equal(t, 12.0, saved.Units[1].Factor)
	require.Len(t, saved.Units[1].Codes, 1)

	w = helper.DoRequest("GET", "/inventory/codes/"+code, nil, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var found models.CodeLookup
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &found))
	assert.Equal(t, 12.0, found.Factor)
	assert.Equal(t, 100.0, found.Price)
	assert.Equal(t, productID, found.Product.ID)
	assert.Equal(t, skuID, found.Sku.ID)

	// The same code on another product of the business is refused and leaves that product as it was.
	other := newUnitsProduct(t, helper)
	otherDetail := productDetail(t, helper, other)
	w = helper.DoRequest("PUT", "/inventory/products/"+other+"/units", map[string]interface{}{"units": []map[string]interface{}{
		{"id": otherDetail.Units[0].ID, "unit_id": otherDetail.Units[0].UnitID, "factor": 1, "is_base": true,
			"codes": []map[string]interface{}{{"sku_id": defaultSku(t, helper, other), "code": code}}},
	}}, nil)
	assert.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "inventory.code.duplicate")
	assert.True(t, productDetail(t, helper, other).Units[0].IsBase, "the refused save rolled back")

	// Another business reads nothing of it.
	stranger := ownerOfAnotherTenant(t)
	w = stranger.DoRequest("GET", "/inventory/codes/"+code, nil, nil)
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "inventory.code.not-found")
}

func TestUnits_PriceDefaultsToBaseTimesFactor(t *testing.T) {
	helper := SetupInventoryTest(t)
	productID := newUnitsProduct(t, helper)
	detail := productDetail(t, helper, productID)
	code := fmt.Sprintf("778%d", time.Now().UnixNano())
	w := helper.DoRequest("PUT", "/inventory/products/"+productID+"/units", map[string]interface{}{"units": []map[string]interface{}{
		{"id": detail.Units[0].ID, "unit_id": detail.Units[0].UnitID, "factor": 1, "is_base": true},
		{"unit_id": unitNamed(t, helper, "PAQ"), "factor": 6, "codes": []map[string]interface{}{{"sku_id": defaultSku(t, helper, productID), "code": code}}},
	}}, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	w = helper.DoRequest("GET", "/inventory/codes/"+code, nil, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var found models.CodeLookup
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &found))
	assert.Equal(t, 60.0, found.Price)
}

func TestUnits_ExactlyOneBase(t *testing.T) {
	helper := SetupInventoryTest(t)
	productID := newUnitsProduct(t, helper)
	box := unitNamed(t, helper, "CAJ")
	cases := map[string][]map[string]interface{}{
		"no base":    {{"unit_id": box, "factor": 12}},
		"base at 12": {{"unit_id": box, "factor": 12, "is_base": true}},
		"two bases":  {{"unit_id": box, "factor": 1, "is_base": true}, {"unit_id": box, "factor": 1, "is_base": true}},
	}
	for name, units := range cases {
		w := helper.DoRequest("PUT", "/inventory/products/"+productID+"/units", map[string]interface{}{"units": units}, nil)
		assert.Equal(t, http.StatusBadRequest, w.Code, "%s: %s", name, w.Body.String())
	}
	assert.Equal(t, "UND", productDetail(t, helper, productID).Units[0].Code)
}

func TestUnits_AProductOfAnotherBusinessIsNotFound(t *testing.T) {
	helper := SetupInventoryTest(t)
	_, foreign := createOtherTenantProduct(t)
	w := helper.DoRequest("PUT", "/inventory/products/"+foreign+"/units", map[string]interface{}{"units": []map[string]interface{}{
		{"unit_id": unitNamed(t, helper, "UND"), "factor": 1, "is_base": true},
	}}, nil)
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
}
