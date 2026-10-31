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
	"josex/web/modules/inventory/utils"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// INV-018: the four defects the audit called unsafe to sell.

func blockerProduct(t *testing.T, helper *testhelpers.ApiTestHelper, prefix string) string {
	t.Helper()
	w := helper.DoRequest("POST", "/inventory/products", models.CreateProductDto{
		SKU: fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano()), Name: prefix, BasePrice: 10,
	}, nil)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var created models.CreateProductResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	return created.ProductID
}

func blockerBatch(t *testing.T, helper *testhelpers.ApiTestHelper, productID string, quantity float64) models.BatchResponse {
	t.Helper()
	w := helper.DoRequest("POST", "/inventory/batches", map[string]interface{}{
		"product_id":       productID,
		"lot_number":       fmt.Sprintf("LOT_%d", time.Now().UnixNano()),
		"purchase_date":    tenantToday().AddDate(0, 0, -30).Format("2006-01-02"),
		"expiry_date":      tenantToday().AddDate(0, 0, 30).Format("2006-01-02"),
		"unit_cost":        5.0,
		"initial_quantity": quantity,
	}, nil)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var batch models.BatchResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &batch))
	return batch
}

// TestExpiredBatch_LeavesFifoAndReadsExpired - a lot that expired yesterday is not offered for sale and
// lists as expired, with nothing having rewritten its row (D2).
func TestExpiredBatch_LeavesFifoAndReadsExpired(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()
	productID := blockerProduct(t, helper, "EXPIRED")
	batch := blockerBatch(t, helper, productID, 10)

	expireBatch(t, helper, batch.ID.String(), tenantToday().AddDate(0, 0, -1))

	w := helper.DoRequest("GET", "/inventory/batches/product/"+productID+"/oldest", nil, nil)
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())

	w = helper.DoRequest("GET", "/inventory/batches/product/"+productID+"?onlyActive=false", nil, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var list models.ListBatchesResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &list))
	require.Len(t, list.Batches, 1)
	assert.Equal(t, "expired", list.Batches[0].Status)

	w = helper.DoRequest("GET", "/inventory/batches/product/"+productID, nil, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	list = models.ListBatchesResponse{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &list))
	assert.Empty(t, list.Batches, "the default list leaves an expired lot out")
}

// TestBatchOnItsExpiryDate_StillSells - the expiry date is the last good day (D2).
func TestBatchOnItsExpiryDate_StillSells(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()
	productID := blockerProduct(t, helper, "LASTDAY")
	batch := blockerBatch(t, helper, productID, 10)

	expireBatch(t, helper, batch.ID.String(), tenantToday())

	w := helper.DoRequest("GET", "/inventory/batches/product/"+productID+"/oldest", nil, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var oldest models.BatchResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &oldest))
	assert.Equal(t, "expiring_soon", oldest.Status)
}

// TestBatchDates_FollowTheTenantZone - a tenant on another zone counts days from its own date (D1).
func TestBatchDates_FollowTheTenantZone(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()
	const zone = "Europe/Madrid"
	w := helper.DoRequest("PATCH", "/tenants/test-company/settings", map[string]interface{}{"general.timezone": zone}, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	defer func() {
		helper.DoRequest("PATCH", "/tenants/test-company/settings", map[string]interface{}{"general.timezone": "America/La_Paz"}, nil)
	}()
	productID := blockerProduct(t, helper, "ZONE")
	batch := blockerBatch(t, helper, productID, 3)
	expiry := utils.DateIn(time.Now(), zone).AddDate(0, 0, 20)
	expireBatch(t, helper, batch.ID.String(), expiry)

	w = helper.DoRequest("GET", "/inventory/batches/"+batch.ID.String(), nil, nil)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var got models.BatchResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.NotNil(t, got.DaysToExpiry)
	assert.Equal(t, 20, *got.DaysToExpiry)
}

// TestSaleMovement_CannotConsumeReserved - 5 of 5 reserved: a sale of 1 is refused, as is waste, and
// freeing the reservation lets it through (D3).
func TestSaleMovement_CannotConsumeReserved(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()
	productID := blockerProduct(t, helper, "RESERVED")
	blockerBatch(t, helper, productID, 5)
	w := helper.DoRequest("POST", "/inventory/reserve", map[string]interface{}{"product_id": productID, "quantity": 5}, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	for _, kind := range []string{"SALE", "WASTE"} {
		w = helper.DoRequest("POST", "/inventory/movements", map[string]interface{}{"product_id": productID, "movement_type": kind, "quantity": 1}, nil)
		assert.Equal(t, http.StatusConflict, w.Code, w.Body.String())
		assert.Contains(t, w.Body.String(), "inventory.insufficient-stock")
	}

	w = helper.DoRequest("POST", "/inventory/release-reserved", map[string]interface{}{"product_id": productID, "quantity": 1}, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	w = helper.DoRequest("POST", "/inventory/movements", map[string]interface{}{"product_id": productID, "movement_type": "SALE", "quantity": 1}, nil)
	assert.Equal(t, http.StatusCreated, w.Code, w.Body.String())
}

// TestMovement_WithoutStockRowIsRefused - a SKU with no product_stock row answers 404, not a logged
// movement against NULL.
func TestMovement_WithoutStockRowIsRefused(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()
	productID := blockerProduct(t, helper, "NOSTOCK")
	removeStockRow(t, helper, productID)

	w := helper.DoRequest("POST", "/inventory/movements", map[string]interface{}{"product_id": productID, "movement_type": "ADJUSTMENT", "direction": "OUT", "quantity": 1}, nil)

	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "inventory.stock-not-found")
}

// TestStockCheck_RefusesNegativeQuantity - the database itself refuses what the SPs promise never to write.
func TestStockCheck_RefusesNegativeQuantity(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()
	productID := blockerProduct(t, helper, "CHECK")

	assert.ErrorContains(t, setStockQuantity(t, helper, productID, -1), "chk_product_stock_quantities")
}

// TestRemoveProductFromCategory_OtherTenant - another tenant's ids delete nothing and answer 404.
func TestRemoveProductFromCategory_OtherTenant(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()
	productID := blockerProduct(t, helper, "MAPPED")
	w := helper.DoRequest("POST", "/inventory/categories", map[string]interface{}{"name": fmt.Sprintf("Cat %d", time.Now().UnixNano()), "slug": fmt.Sprintf("cat-%d", time.Now().UnixNano())}, nil)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var category struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &category))
	path := "/inventory/products/" + productID + "/categories/" + category.ID
	w = helper.DoRequest("POST", path, nil, nil)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	other := ownerOfAnotherTenant(t)
	defer other.Close()
	w = other.DoRequest("DELETE", path, nil, nil)
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "inventory.category-mapping.not-found")

	w = helper.DoRequest("GET", "/inventory/products/"+productID+"/categories", nil, nil)
	assert.Contains(t, w.Body.String(), category.ID, "the owner's mapping is still there")
	w = helper.DoRequest("DELETE", path, nil, nil)
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
}

// TestInventoryReads_NeedAPermission - one 403 per resource, and the read permission opens each (D4).
func TestInventoryReads_NeedAPermission(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()
	cases := []struct{ path, perm string }{
		{"/inventory/products", "inventory:products:read"},
		{"/inventory/categories", "inventory:categories:read"},
		{"/inventory/movements", "inventory:stock:read"},
		{"/inventory/batches/expiring", "inventory:batches:read"},
	}
	none := asMember(t, helper)
	defer none.Close()

	for _, tc := range cases {
		w := none.DoRequest("GET", tc.path, nil, nil)
		assert.Equal(t, http.StatusForbidden, w.Code, "%s without a permission: %s", tc.path, w.Body.String())

		member := asMember(t, helper, tc.perm)
		w = member.DoRequest("GET", tc.path, nil, nil)
		assert.Equal(t, http.StatusOK, w.Code, "%s with %s: %s", tc.path, tc.perm, w.Body.String())
		member.Close()
	}
}
