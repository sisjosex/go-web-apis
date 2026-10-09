//go:build integration
// +build integration

package sales_test

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

// INFRA-014: every sales route reached with a success — customers, the order lifecycle, returns,
// payments and the report — plus their refusals. These replace tests that never signed in.

func init() {
	// Initialize test environment (load .env.test before any config)
	coreTestHelpers.InitTestEnvironment()
}

func data(t *testing.T, body []byte) map[string]interface{} {
	t.Helper()
	var resp struct {
		Data map[string]interface{} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(body, &resp), string(body))
	return resp.Data
}

func newCustomer(t *testing.T, helper *coreTestHelpers.ApiTestHelper) string {
	t.Helper()
	w := helper.DoRequest("POST", "/sales/customers", map[string]interface{}{
		"name": "Flow Cliente", "email": fmt.Sprintf("flow-%d@example.com", uniqueTimestamp()),
		"phone_number": "1234567890", "address": "Calle Uno 123", "city": "Santiago",
		"state": "RM", "postal_code": "8320000", "country": "Chile",
	}, nil)
	require.Equal(t, http.StatusCreated, w.Code, "create customer: %s", w.Body.String())
	id, _ := data(t, w.Body.Bytes())["id"].(string)
	require.NotEmpty(t, id)
	return id
}

// orderWithItem is an order holding one stocked line of 2 × 10.
func orderWithItem(t *testing.T, helper *coreTestHelpers.ApiTestHelper) string {
	t.Helper()
	productID := newSalesProduct(t, helper, "FLOW")
	stockBatchFor(t, helper, productID, 10)
	orderID := newSalesOrder(t, helper)
	w := helper.DoRequest("POST", "/sales/orders/"+orderID+"/items", map[string]interface{}{
		"product_id": productID, "quantity": 2, "unit_price": 10.0,
	}, nil)
	require.Equal(t, http.StatusCreated, w.Code, "add item: %s", w.Body.String())
	return orderID
}

// TestCustomers_GetUpdateDelete - a customer is read, edited and deleted, which deactivates it.
func TestCustomers_GetUpdateDelete(t *testing.T) {
	helper := SetupSalesAuthTest(t)
	defer helper.Close()
	id := newCustomer(t, helper)

	w := helper.DoRequest("GET", "/sales/customers/"+id, nil, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	w = helper.DoRequest("PATCH", "/sales/customers/"+id, map[string]interface{}{"city": "Cochabamba"}, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	w = helper.DoRequest("GET", "/sales/customers/"+id, nil, nil)
	assert.Contains(t, w.Body.String(), "Cochabamba", "the edit is stored")

	w = helper.DoRequest("PATCH", "/sales/customers/"+id, map[string]interface{}{"city": "X"}, nil)
	assert.Equal(t, http.StatusBadRequest, w.Code, "a city under 2 characters: %s", w.Body.String())
	w = helper.DoRequest("PATCH", "/sales/customers/"+uuid.NewString(), map[string]interface{}{"city": "Sucre"}, nil)
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())

	w = helper.DoRequest("DELETE", "/sales/customers/"+id, nil, nil)
	require.Contains(t, []int{http.StatusOK, http.StatusNoContent}, w.Code, w.Body.String())
	w = helper.DoRequest("GET", "/sales/customers/"+id, nil, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, "inactive", data(t, w.Body.Bytes())["status"], "a delete deactivates: the customer's orders keep it")
	w = helper.DoRequest("DELETE", "/sales/customers/not-a-uuid", nil, nil)
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
}

// TestOrders_ListGetUpdate - an order shows in the list, reads back and takes an edit.
func TestOrders_ListGetUpdate(t *testing.T) {
	helper := SetupSalesAuthTest(t)
	defer helper.Close()
	orderID := newSalesOrder(t, helper)

	w := helper.DoRequest("GET", "/sales/orders?limit=100", nil, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), orderID)

	w = helper.DoRequest("GET", "/sales/orders/"+orderID, nil, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, orderID, data(t, w.Body.Bytes())["id"])

	w = helper.DoRequest("PATCH", "/sales/orders/"+orderID, map[string]interface{}{"notes": "Leave at the door"}, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	w = helper.DoRequest("GET", "/sales/orders/"+orderID, nil, nil)
	assert.Contains(t, w.Body.String(), "Leave at the door", "the edit is stored")

	w = helper.DoRequest("PATCH", "/sales/orders/"+orderID, map[string]interface{}{"shipping_address": "x"}, nil)
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	w = helper.DoRequest("GET", "/sales/orders/"+uuid.NewString(), nil, nil)
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	w = helper.DoRequest("PATCH", "/sales/orders/"+uuid.NewString(), map[string]interface{}{"notes": "ghost"}, nil)
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	w = helper.DoRequest("POST", "/sales/orders/not-a-uuid/items", map[string]interface{}{"product_id": uuid.NewString(), "quantity": 1, "unit_price": 1}, nil)
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
}

// TestOrders_Cancel - a pending order is cancelled; a cancelled one cannot be completed.
func TestOrders_Cancel(t *testing.T) {
	helper := SetupSalesAuthTest(t)
	defer helper.Close()
	orderID := orderWithItem(t, helper)

	w := helper.DoRequest("PATCH", "/sales/orders/"+orderID+"/cancel", nil, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, "cancelled", data(t, w.Body.Bytes())["status"])

	w = helper.DoRequest("PATCH", "/sales/orders/"+orderID+"/complete", nil, nil)
	assert.Equal(t, http.StatusConflict, w.Code, "a cancelled order is not completed: %s", w.Body.String())
	w = helper.DoRequest("PATCH", "/sales/orders/"+uuid.NewString()+"/cancel", nil, nil)
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	w = helper.DoRequest("PATCH", "/sales/orders/not-a-uuid/cancel", nil, nil)
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
}

// TestPaymentsAndReturns - a completed order is paid and returned; both list by order.
func TestPaymentsAndReturns(t *testing.T) {
	helper := SetupSalesAuthTest(t)
	defer helper.Close()
	orderID := orderWithItem(t, helper)
	w := helper.DoRequest("PATCH", "/sales/orders/"+orderID+"/complete", nil, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	w = helper.DoRequest("POST", "/sales/payments", map[string]interface{}{"order_id": orderID, "amount": 20.0, "payment_method": "cash"}, nil)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	w = helper.DoRequest("GET", "/sales/payments?order_id="+orderID, nil, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "cash")

	w = helper.DoRequest("POST", "/sales/payments", map[string]interface{}{"order_id": orderID, "amount": 5.0, "payment_method": "barter"}, nil)
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	w = helper.DoRequest("POST", "/sales/payments", map[string]interface{}{"order_id": uuid.NewString(), "amount": 5.0, "payment_method": "cash"}, nil)
	assert.Equal(t, http.StatusNotFound, w.Code, "an unknown order: %s", w.Body.String())
	w = helper.DoRequest("GET", "/sales/payments", nil, nil)
	assert.Equal(t, http.StatusBadRequest, w.Code, "order_id is required: %s", w.Body.String())

	w = helper.DoRequest("POST", "/sales/returns", map[string]interface{}{"order_id": orderID, "reason": "Wrong size"}, nil)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	returnID, _ := data(t, w.Body.Bytes())["id"].(string)
	require.NotEmpty(t, returnID)

	w = helper.DoRequest("GET", "/sales/returns?order_id="+orderID, nil, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), returnID)

	w = helper.DoRequest("PATCH", "/sales/returns/"+returnID+"/approve", map[string]interface{}{"approved": true}, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	w = helper.DoRequest("PATCH", "/sales/returns/"+uuid.NewString()+"/approve", map[string]interface{}{"approved": true}, nil)
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	w = helper.DoRequest("PATCH", "/sales/returns/not-a-uuid/approve", map[string]interface{}{"approved": true}, nil)
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	w = helper.DoRequest("POST", "/sales/returns", map[string]interface{}{"order_id": uuid.NewString(), "reason": "Ghost"}, nil)
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
}

// TestSalesReport - the report covers a date range; both ends are required and must be dates.
func TestSalesReport(t *testing.T) {
	helper := SetupSalesAuthTest(t)
	defer helper.Close()
	today := time.Now().Format("2006-01-02")

	w := helper.DoRequest("GET", "/sales/reports/sales?start_date=2026-01-01&end_date="+today, nil, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "metrics")

	w = helper.DoRequest("GET", "/sales/reports/sales", nil, nil)
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	w = helper.DoRequest("GET", "/sales/reports/sales?start_date=yesterday&end_date="+today, nil, nil)
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
}

// TestSales_CrossTenantIsNotFound - another workspace with sales on reads none of test-company's.
func TestSales_CrossTenantIsNotFound(t *testing.T) {
	helper := SetupSalesAuthTest(t)
	defer helper.Close()
	customerID := newCustomer(t, helper)
	orderID := newSalesOrder(t, helper)

	other := coreTestHelpers.SetupApiTest(t)
	stamp := uniqueTimestamp()
	email := fmt.Sprintf("sales-other-%d@test.com", stamp)
	slug := fmt.Sprintf("sales-%d", stamp)
	other.Register(email, "$Password2025", "Other", "Seller")
	_, err := other.Login(email, "$Password2025")
	require.NoError(t, err)
	w := other.DoRequest("POST", "/tenants/self-service", map[string]interface{}{"slug": slug, "name": "Other"}, nil)
	require.Contains(t, []int{http.StatusOK, http.StatusCreated}, w.Code, w.Body.String())
	w = other.DoRequest("PUT", "/tenants/"+slug+"/modules/sales", map[string]interface{}{"is_enabled": true}, map[string]string{"X-Tenant-Slug": slug})
	require.Contains(t, []int{http.StatusOK, http.StatusNoContent}, w.Code, "enable sales: %s", w.Body.String())
	other.SetTenantSlug(slug)

	w = other.DoRequest("GET", "/sales/customers/"+customerID, nil, nil)
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	w = other.DoRequest("GET", "/sales/orders/"+orderID, nil, nil)
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	w = other.DoRequest("PATCH", "/sales/orders/"+orderID+"/cancel", nil, nil)
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	w = other.DoRequest("DELETE", "/sales/customers/"+customerID, nil, nil)
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
}
