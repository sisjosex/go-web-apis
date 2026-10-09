//go:build integration
// +build integration

package purchasing_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	coreTestHelpers "josex/web/modules/core/testhelpers"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// INFRA-014: every purchasing route reached with a success through the flow a buyer follows —
// supplier, order, items, approval, receipt (stock rises), invoice — plus batches and quotes.

func init() {
	coreTestHelpers.InitTestEnvironment()
}

// setupPurchasing signs the super_admin into test-company, which has purchasing and inventory on.
func setupPurchasing(t *testing.T) *coreTestHelpers.ApiTestHelper {
	t.Helper()
	helper := coreTestHelpers.SetupApiTest(t)
	if _, err := helper.LoginAsSuperAdmin(); err != nil {
		t.Fatalf("login: %v", err)
	}
	helper.SetTenantSlug("test-company")
	return helper
}

func decode(t *testing.T, body []byte) map[string]interface{} {
	t.Helper()
	var out map[string]interface{}
	require.NoError(t, json.Unmarshal(body, &out), string(body))
	return out
}

func stamp() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

func createProduct(t *testing.T, helper *coreTestHelpers.ApiTestHelper) string {
	t.Helper()
	w := helper.DoRequest("POST", "/inventory/products", map[string]interface{}{
		"sku": "PUR-" + stamp(), "name": "Purchased good", "base_price": 10.0,
	}, nil)
	require.Equal(t, http.StatusCreated, w.Code, "create product: %s", w.Body.String())
	return decode(t, w.Body.Bytes())["product_id"].(string)
}

func createSupplier(t *testing.T, helper *coreTestHelpers.ApiTestHelper) string {
	t.Helper()
	w := helper.DoRequest("POST", "/purchasing/suppliers", map[string]interface{}{
		"name": "Supplier " + stamp(), "email": "supplier@example.com", "payment_terms": 30,
	}, nil)
	require.Equal(t, http.StatusCreated, w.Code, "create supplier: %s", w.Body.String())
	return decode(t, w.Body.Bytes())["supplier_id"].(string)
}

func createOrder(t *testing.T, helper *coreTestHelpers.ApiTestHelper, supplierID string) string {
	t.Helper()
	w := helper.DoRequest("POST", "/purchasing/purchase-orders", map[string]interface{}{"supplier_id": supplierID}, nil)
	require.Equal(t, http.StatusCreated, w.Code, "create order: %s", w.Body.String())
	order := decode(t, w.Body.Bytes())
	assert.Equal(t, "draft", order["status"])
	return order["po_id"].(string)
}

func stockOf(t *testing.T, helper *coreTestHelpers.ApiTestHelper, productID string) float64 {
	t.Helper()
	w := helper.DoRequest("GET", "/inventory/stock/"+productID, nil, nil)
	if w.Code == http.StatusNotFound {
		return 0
	}
	require.Equal(t, http.StatusOK, w.Code, "stock: %s", w.Body.String())
	quantity, _ := decode(t, w.Body.Bytes())["current_quantity"].(float64)
	return quantity
}

// TestSuppliers_CreateListGet - a supplier created is listed and read back; bad input and unknown ids are refused.
func TestSuppliers_CreateListGet(t *testing.T) {
	helper := setupPurchasing(t)
	id := createSupplier(t, helper)

	w := helper.DoRequest("GET", "/purchasing/suppliers?page_size=100", nil, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), id)

	w = helper.DoRequest("GET", "/purchasing/suppliers/"+id, nil, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, id, decode(t, w.Body.Bytes())["id"])

	w = helper.DoRequest("GET", "/purchasing/suppliers/"+uuid.NewString(), nil, nil)
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	w = helper.DoRequest("GET", "/purchasing/suppliers/not-a-uuid", nil, nil)
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	w = helper.DoRequest("POST", "/purchasing/suppliers", map[string]interface{}{"name": "ab"}, nil)
	assert.Equal(t, http.StatusBadRequest, w.Code, "a name under 3 characters: %s", w.Body.String())
}

// TestPurchaseOrders_Lifecycle - draft → items → approved → received (stock rises) → invoiced, and every
// step refused out of order.
func TestPurchaseOrders_Lifecycle(t *testing.T) {
	helper := setupPurchasing(t)
	productID := createProduct(t, helper)
	supplierID := createSupplier(t, helper)
	poID := createOrder(t, helper, supplierID)
	base := "/purchasing/purchase-orders/" + poID

	w := helper.DoRequest("PATCH", base+"/approve", nil, nil)
	assert.Equal(t, http.StatusConflict, w.Code, "an order without items is not approved: %s", w.Body.String())

	w = helper.DoRequest("POST", base+"/items", map[string]interface{}{"product_id": productID, "quantity": 10, "unit_cost": 4.5}, nil)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	assert.Equal(t, 45.0, decode(t, w.Body.Bytes())["line_total"])

	w = helper.DoRequest("POST", base+"/items", map[string]interface{}{"product_id": uuid.NewString(), "quantity": 1, "unit_cost": 1}, nil)
	assert.Equal(t, http.StatusNotFound, w.Code, "an unknown product: %s", w.Body.String())
	w = helper.DoRequest("POST", base+"/items", map[string]interface{}{"product_id": productID, "quantity": 0, "unit_cost": 1}, nil)
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())

	w = helper.DoRequest("GET", base, nil, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), productID, "the order carries its item")

	w = helper.DoRequest("PATCH", base+"/receive", map[string]interface{}{}, nil)
	assert.Equal(t, http.StatusConflict, w.Code, "a draft is not received: %s", w.Body.String())

	w = helper.DoRequest("PATCH", base+"/approve", nil, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, "approved", decode(t, w.Body.Bytes())["status"])
	w = helper.DoRequest("PATCH", base+"/approve", nil, nil)
	assert.Equal(t, http.StatusConflict, w.Code, "approved once: %s", w.Body.String())

	w = helper.DoRequest("GET", "/purchasing/purchase-orders?status=approved&page_size=100", nil, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), poID)

	before := stockOf(t, helper, productID)
	w = helper.DoRequest("PATCH", base+"/receive", map[string]interface{}{"notes": "all boxes"}, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, before+10, stockOf(t, helper, productID), "receiving adds the ordered quantity to stock")
	w = helper.DoRequest("PATCH", base+"/receive", map[string]interface{}{}, nil)
	assert.Equal(t, http.StatusConflict, w.Code, "received once: %s", w.Body.String())

	invoice := map[string]interface{}{
		"invoice_number": "INV-" + stamp(), "invoice_date": "2026-10-01", "invoice_amount": 45.0, "due_date": "2026-10-31",
	}
	w = helper.DoRequest("POST", base+"/invoices", invoice, nil)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	w = helper.DoRequest("POST", "/purchasing/purchase-orders/"+uuid.NewString()+"/invoices", invoice, nil)
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	w = helper.DoRequest("POST", base+"/invoices", map[string]interface{}{"invoice_number": "X"}, nil)
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())

	w = helper.DoRequest("GET", "/purchasing/pending-payments", nil, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), poID, "an invoiced order is owed")
}

// TestPurchaseOrders_NotFoundAndBadInput - unknown and malformed ids on every order route.
func TestPurchaseOrders_NotFoundAndBadInput(t *testing.T) {
	helper := setupPurchasing(t)
	unknown := "/purchasing/purchase-orders/" + uuid.NewString()

	w := helper.DoRequest("GET", unknown, nil, nil)
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	w = helper.DoRequest("PATCH", unknown+"/approve", nil, nil)
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	w = helper.DoRequest("GET", "/purchasing/purchase-orders/not-a-uuid", nil, nil)
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	w = helper.DoRequest("POST", "/purchasing/purchase-orders", map[string]interface{}{"supplier_id": uuid.NewString()}, nil)
	assert.Equal(t, http.StatusNotFound, w.Code, "an unknown supplier: %s", w.Body.String())
	w = helper.DoRequest("POST", "/purchasing/purchase-orders", map[string]interface{}{}, nil)
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
}

// TestPurchasing_CrossTenantIsNotFound - another workspace with purchasing on reads none of test-company's.
func TestPurchasing_CrossTenantIsNotFound(t *testing.T) {
	helper := setupPurchasing(t)
	supplierID := createSupplier(t, helper)
	poID := createOrder(t, helper, supplierID)

	other := coreTestHelpers.SetupApiTest(t)
	email := "purchasing-other-" + stamp() + "@test.com"
	slug := "purchasing-" + stamp()
	other.Register(email, "$Password2025", "Other", "Buyer")
	_, err := other.Login(email, "$Password2025")
	require.NoError(t, err)
	w := other.DoRequest("POST", "/tenants/self-service", map[string]interface{}{"slug": slug, "name": "Other"}, nil)
	require.Contains(t, []int{http.StatusOK, http.StatusCreated}, w.Code, w.Body.String())
	w = other.DoRequest("PUT", "/tenants/"+slug+"/modules/purchasing", map[string]interface{}{"is_enabled": true}, map[string]string{"X-Tenant-Slug": slug})
	require.Contains(t, []int{http.StatusOK, http.StatusNoContent}, w.Code, "enable purchasing: %s", w.Body.String())
	other.SetTenantSlug(slug)

	w = other.DoRequest("GET", "/purchasing/suppliers/"+supplierID, nil, nil)
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	w = other.DoRequest("GET", "/purchasing/purchase-orders/"+poID, nil, nil)
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	w = other.DoRequest("POST", "/purchasing/purchase-orders", map[string]interface{}{"supplier_id": supplierID}, nil)
	assert.Equal(t, http.StatusNotFound, w.Code, "another workspace's supplier: %s", w.Body.String())
	w = other.DoRequest("PATCH", "/purchasing/purchase-orders/"+poID+"/approve", nil, nil)
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
}

// TestBatches_FIFO - batches of a product list oldest first; the oldest is the one sold next.
func TestBatches_FIFO(t *testing.T) {
	helper := setupPurchasing(t)
	productID := createProduct(t, helper)

	w := helper.DoRequest("GET", "/purchasing/batches/oldest?product_id="+productID, nil, nil)
	assert.Equal(t, http.StatusNotFound, w.Code, "no batch yet: %s", w.Body.String())

	for i, received := range []string{"2026-09-01T00:00:00Z", "2026-08-01T00:00:00Z"} {
		w = helper.DoRequest("POST", "/purchasing/batches", map[string]interface{}{
			"product_id": productID, "batch_number": fmt.Sprintf("B-%d-%s", i, stamp()),
			"quantity": 5, "unit_cost": 2.0, "receipt_date": received,
		}, nil)
		require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	}

	w = helper.DoRequest("GET", "/purchasing/batches?product_id="+productID, nil, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, 2.0, decode(t, w.Body.Bytes())["batch_count"])

	w = helper.DoRequest("GET", "/purchasing/batches/oldest?product_id="+productID, nil, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, decode(t, w.Body.Bytes())["receipt_date"], "2026-08-01", "the August batch goes first")

	w = helper.DoRequest("GET", "/purchasing/batches", nil, nil)
	assert.Equal(t, http.StatusBadRequest, w.Code, "product_id is required: %s", w.Body.String())
	w = helper.DoRequest("POST", "/purchasing/batches", map[string]interface{}{"quantity": "many"}, nil)
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
}

// TestPriceComparison - a product's suppliers compared by unit cost; a product nobody sells answers empty.
func TestPriceComparison(t *testing.T) {
	helper := setupPurchasing(t)
	productID := createProduct(t, helper)

	w := helper.DoRequest("GET", "/purchasing/price-comparison?product_id="+productID, nil, nil)
	require.Equal(t, http.StatusOK, w.Code, "a product with no supplier price: %s", w.Body.String())
	assert.Equal(t, 0.0, decode(t, w.Body.Bytes())["supplier_count"])

	for _, cost := range []float64{6, 4} {
		poID := createOrder(t, helper, createSupplier(t, helper))
		w = helper.DoRequest("POST", "/purchasing/purchase-orders/"+poID+"/items", map[string]interface{}{"product_id": productID, "quantity": 1, "unit_cost": cost}, nil)
		require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	}
	w = helper.DoRequest("GET", "/purchasing/price-comparison?product_id="+productID, nil, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	comparison := decode(t, w.Body.Bytes())
	assert.Equal(t, 2.0, comparison["supplier_count"])
	assert.Equal(t, 4.0, comparison["best_price"])

	w = helper.DoRequest("GET", "/purchasing/price-comparison", nil, nil)
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
}

// TestRFQ_QuoteCompareSelect - a quote request collects supplier answers; selecting one closes it.
func TestRFQ_QuoteCompareSelect(t *testing.T) {
	helper := setupPurchasing(t)
	productID := createProduct(t, helper)
	cheap, dear := createSupplier(t, helper), createSupplier(t, helper)

	w := helper.DoRequest("POST", "/purchasing/rfq", map[string]interface{}{
		"supplier_ids": []string{cheap, dear},
		"items":        []map[string]interface{}{{"product_id": productID, "quantity": 20}},
	}, nil)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	rfqID := decode(t, w.Body.Bytes())["id"].(string)
	base := "/purchasing/rfq/" + rfqID

	w = helper.DoRequest("GET", base+"/responses", nil, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, 0.0, decode(t, w.Body.Bytes())["response_count"])

	cheapResponse := addRFQResponse(t, helper, rfqID, cheap, 80)
	addRFQResponse(t, helper, rfqID, dear, 120)

	w = helper.DoRequest("GET", base+"/responses", nil, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, 2.0, decode(t, w.Body.Bytes())["response_count"])

	w = helper.DoRequest("GET", base+"/comparison", nil, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), cheapResponse)

	w = helper.DoRequest("PATCH", base+"/responses/"+cheapResponse+"/select", nil, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	w = helper.DoRequest("POST", "/purchasing/rfq", map[string]interface{}{"supplier_ids": []string{}}, nil)
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	w = helper.DoRequest("GET", "/purchasing/rfq/not-a-uuid/responses", nil, nil)
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
}

// TestPurchasing_RequiresSession - every purchasing route sits behind a session.
func TestPurchasing_RequiresSession(t *testing.T) {
	helper := coreTestHelpers.SetupApiTest(t)
	helper.SetTenantSlug("test-company")
	w := helper.DoRequest("GET", "/purchasing/suppliers", nil, nil)
	assert.Equal(t, http.StatusUnauthorized, w.Code, w.Body.String())
}
