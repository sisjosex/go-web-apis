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

	"josex/web/modules/inventory/models"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// INV-020: what reaches the SPs unchecked, who changed what, and the tenant boundary per route.

// TestMovement_BadInputIsA400 - a malformed id or an unknown type never reaches the SP as a 500.
func TestMovement_BadInputIsA400(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()
	productID := blockerProduct(t, helper, "BADINPUT")

	for _, body := range []map[string]interface{}{
		{"product_id": "x", "movement_type": "SALE", "quantity": 1},
		{"product_id": productID, "movement_type": "GIFT", "quantity": 1},
		{"product_id": productID, "sku_id": "x", "movement_type": "SALE", "quantity": 1},
	} {
		w := helper.DoRequest("POST", "/inventory/movements", body, nil)
		assert.Equal(t, http.StatusBadRequest, w.Code, "%v: %s", body, w.Body.String())
		assert.Contains(t, w.Body.String(), "validation.invalid")
	}

	w := helper.DoRequest("POST", "/inventory/reserve", map[string]interface{}{"product_id": "x", "quantity": 1}, nil)
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())

	w = helper.DoRequest("POST", "/inventory/products/"+productID+"/media", map[string]interface{}{"media_type": "image", "url": "not a url"}, nil)
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
}

// TestCategoryLists_CapTheLimit - a huge limit answers at most 100 rows on all three lists.
func TestCategoryLists_CapTheLimit(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()
	stamp := time.Now().UnixNano()
	var categoryID string
	for i := 0; i < 102; i++ {
		w := helper.DoRequest("POST", "/inventory/categories", map[string]interface{}{"name": fmt.Sprintf("Cap %d %d", stamp, i), "slug": fmt.Sprintf("cap-%d-%d", stamp, i)}, nil)
		require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
		var category struct {
			ID string `json:"id"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &category))
		categoryID = category.ID
	}

	w := helper.DoRequest("GET", "/inventory/categories?limit=5000", nil, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var list struct {
		Categories []json.RawMessage `json:"categories"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &list))
	assert.Len(t, list.Categories, 100)

	w = helper.DoRequest("GET", fmt.Sprintf("/inventory/categories/search?search_term=Cap+%d&limit=5000", stamp), nil, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var found []json.RawMessage
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &found))
	assert.Len(t, found, 100)

	w = helper.DoRequest("GET", "/inventory/categories/"+categoryID+"/products?limit=5000", nil, nil)
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
}

// TestUpdatedBy_NamesTheCaller - PUT then GET: the row says who changed it, on all three resources.
func TestUpdatedBy_NamesTheCaller(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()
	productID := blockerProduct(t, helper, "AUDIT")

	w := helper.DoRequest("PUT", "/inventory/products/"+productID, map[string]interface{}{"name": "Audited", "base_price": 12}, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	w = helper.DoRequest("GET", "/inventory/products/"+productID, nil, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var product models.ProductDetail
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &product))
	require.NotNil(t, product.UpdatedByName, w.Body.String())
	assert.NotEmpty(t, *product.UpdatedByName)
	assert.NotNil(t, product.UpdatedAt)

	batch := blockerBatch(t, helper, productID, 3)
	w = helper.DoRequest("PATCH", "/inventory/batches/"+batch.ID.String(), map[string]interface{}{"lot_number": "RENAMED"}, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	w = helper.DoRequest("GET", "/inventory/batches/"+batch.ID.String(), nil, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var got models.BatchResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.NotNil(t, got.UpdatedByName, w.Body.String())
	assert.Equal(t, *product.UpdatedByName, *got.UpdatedByName)

	stamp := time.Now().UnixNano()
	w = helper.DoRequest("POST", "/inventory/categories", map[string]interface{}{"name": fmt.Sprintf("Who %d", stamp), "slug": fmt.Sprintf("who-%d", stamp)}, nil)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var category models.CategoryResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &category))
	w = helper.DoRequest("PUT", "/inventory/categories/"+category.ID, map[string]interface{}{"description": "changed"}, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	w = helper.DoRequest("GET", "/inventory/categories/"+category.ID, nil, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	category = models.CategoryResponse{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &category))
	require.NotNil(t, category.UpdatedByName, w.Body.String())
	assert.Equal(t, *product.UpdatedByName, *category.UpdatedByName)
}

// TestOtherTenant_NotFoundOnEveryID - every :id route answers 404 for another tenant's id.
func TestOtherTenant_NotFoundOnEveryID(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()
	other, otherProduct := createOtherTenantProduct(t)
	defer other.Close()
	w := other.DoRequest("POST", "/inventory/batches", map[string]interface{}{
		"product_id": otherProduct, "lot_number": "OTHER", "unit_cost": 1.0, "initial_quantity": 2.0,
		"purchase_date": tenantToday().AddDate(0, 0, -1).Format("2006-01-02"),
		"expiry_date":   tenantToday().AddDate(0, 0, 30).Format("2006-01-02"),
	}, nil)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var batch models.BatchResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &batch))
	stamp := time.Now().UnixNano()
	w = other.DoRequest("POST", "/inventory/categories", map[string]interface{}{"name": fmt.Sprintf("Their %d", stamp), "slug": fmt.Sprintf("their-%d", stamp)}, nil)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var category models.CategoryResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &category))
	w = other.DoRequest("GET", "/inventory/movements?product_id="+otherProduct, nil, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var movements models.ListMovementsResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &movements))
	require.NotEmpty(t, movements.Movements)
	movementID := movements.Movements[0].ID

	for _, route := range []struct {
		method, path string
		body         interface{}
	}{
		{"GET", "/inventory/products/" + otherProduct, nil},
		{"PUT", "/inventory/products/" + otherProduct, map[string]interface{}{"name": "Mine", "base_price": 1}},
		{"GET", "/inventory/products/" + otherProduct + "/skus", nil},
		{"GET", "/inventory/categories/" + category.ID, nil},
		{"PUT", "/inventory/categories/" + category.ID, map[string]interface{}{"description": "mine"}},
		{"DELETE", "/inventory/categories/" + category.ID, nil},
		{"GET", "/inventory/batches/" + batch.ID.String(), nil},
		{"PATCH", "/inventory/batches/" + batch.ID.String(), map[string]interface{}{"lot_number": "MINE"}},
		{"DELETE", "/inventory/batches/" + batch.ID.String(), nil},
		{"GET", "/inventory/batches/product/" + otherProduct + "/oldest", nil},
		{"GET", fmt.Sprintf("/inventory/movements/%v", movementID), nil},
		{"GET", "/inventory/stock/" + otherProduct, nil},
	} {
		w := helper.DoRequest(route.method, route.path, route.body, nil)
		assert.Equal(t, http.StatusNotFound, w.Code, "%s %s: %s", route.method, route.path, w.Body.String())
	}

	w = helper.DoRequest("GET", "/inventory/batches/product/"+otherProduct, nil, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var lots models.ListBatchesResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &lots))
	assert.Empty(t, lots.Batches, "a list by another tenant's product answers nothing")

	w = helper.DoRequest("GET", "/inventory/products/"+uuid.NewString(), nil, nil)
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
}

// TestConcurrentSales_OneWinsTheLastUnit - two SALEs of the last unit: one 2xx, one insufficient-stock.
func TestConcurrentSales_OneWinsTheLastUnit(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()
	productID := blockerProduct(t, helper, "LASTUNIT")
	blockerBatch(t, helper, productID, 1)

	codes := make([]int, 2)
	bodies := make([]string, 2)
	var wg sync.WaitGroup
	for i := range codes {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			w := helper.DoRequest("POST", "/inventory/movements", map[string]interface{}{"product_id": productID, "movement_type": "SALE", "quantity": 1}, nil)
			codes[i], bodies[i] = w.Code, w.Body.String()
		}(i)
	}
	wg.Wait()

	assert.ElementsMatch(t, []int{http.StatusCreated, http.StatusConflict}, codes, "%v", bodies)
	loser := bodies[0]
	if codes[0] == http.StatusCreated {
		loser = bodies[1]
	}
	assert.Contains(t, loser, "inventory.insufficient-stock")
}
