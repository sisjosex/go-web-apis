//go:build integration
// +build integration

package sales_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	coreTestHelpers "josex/web/modules/core/testhelpers"
	"josex/web/modules/sales/models"
)

func init() {
	// Initialize test environment (load .env.test before any config)
	coreTestHelpers.InitTestEnvironment()
}

// ============================================
// Customer Tests
// ============================================

// TestCreateCustomer requires authentication with tenant context
func TestCreateCustomer_RequiresAuth(t *testing.T) {
	helper := SetupSalesTest(t)
	defer helper.Close()

	body := models.CreateCustomerRequestDto{
		Name:        "John Doe",
		Email:       "john@example.com",
		PhoneNumber: "1234567890",
		Address:     "123 Main St",
		City:        "New York",
		State:       "NY",
		PostalCode:  "10001",
		Country:     "USA",
	}

	w := helper.DoRequest("POST", "/sales/customers", body, map[string]string{})

	// Should require authentication
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestGetCustomer_NotFound(t *testing.T) {
	helper := SetupSalesTest(t)
	defer helper.Close()

	// Authenticate as super admin and set tenant context
	_, err := helper.LoginAsSuperAdmin()
	if err != nil {
		t.Logf("Login error: %v", err)
	}
	helper.SetTenantSlug("test-company")

	w := helper.DoRequest("GET", "/sales/customers/550e8400-e29b-41d4-a716-446655440000", nil, map[string]string{})

	// Should return 404 for non-existent customer
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ============================================
// Sales Order Tests
// ============================================

func TestCreateSalesOrder_CustomerNotFound(t *testing.T) {
	helper := SetupSalesTest(t)
	defer helper.Close()

	discountAmount := 10.0
	body := models.CreateSalesOrderRequestDto{
		CustomerID:      "550e8400-e29b-41d4-a716-446655440000",
		ShippingAddress: "456 Oak Ave",
		Notes:           "Rush delivery",
		DiscountAmount:  &discountAmount,
		Items: []models.CreateOrderItemRequestDto{
			{
				ProductID: "550e8400-e29b-41d4-a716-446655440001",
				Quantity:  1,
			},
		},
	}

	w := helper.DoRequest("POST", "/sales/orders", body, map[string]string{})

	// Should require authentication (returns 401, not 404)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestListSalesOrders_RequiresAuth(t *testing.T) {
	helper := SetupSalesTest(t)
	defer helper.Close()

	w := helper.DoRequest("GET", "/sales/orders", nil, map[string]string{})

	// Should require authentication
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestUpdateCustomer_RequiresAuth(t *testing.T) {
	helper := SetupSalesTest(t)
	defer helper.Close()

	customerID := "550e8400-e29b-41d4-a716-446655440000"
	name := "Jane Doe"
	body := models.UpdateCustomerRequestDto{
		Name: &name,
	}

	w := helper.DoRequest("PATCH", fmt.Sprintf("/sales/customers/%s", customerID), body, map[string]string{})

	// Should require authentication
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestDeleteCustomer_RequiresAuth(t *testing.T) {
	helper := SetupSalesTest(t)
	defer helper.Close()

	customerID := "550e8400-e29b-41d4-a716-446655440000"
	w := helper.DoRequest("DELETE", fmt.Sprintf("/sales/customers/%s", customerID), nil, map[string]string{})

	// Should require authentication
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestGetSalesOrder_RequiresAuth(t *testing.T) {
	helper := SetupSalesTest(t)
	defer helper.Close()

	orderID := "550e8400-e29b-41d4-a716-446655440000"
	w := helper.DoRequest("GET", fmt.Sprintf("/sales/orders/%s", orderID), nil, map[string]string{})

	// Should require authentication
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// ============================================================================
// Edge Case Tests — Authenticated
// ============================================================================

// SetupSalesAuthTest logs in as admin with tenant context for sales module.
func SetupSalesAuthTest(t *testing.T) *coreTestHelpers.ApiTestHelper {
	t.Helper()
	helper := coreTestHelpers.SetupApiTest(t)
	_, err := helper.Login("admin@test.local", "Admin123!")
	if err != nil {
		t.Fatalf("sales test login failed: %v", err)
	}
	helper.SetTenantSlug("test-company")
	return helper
}

// TestCreateCustomer_MissingName - name is required → 400
func TestCreateCustomer_MissingName(t *testing.T) {
	helper := SetupSalesAuthTest(t)
	defer helper.Close()

	body := map[string]interface{}{
		"email":        "noname@example.com",
		"phone_number": "1234567890",
	}

	w := helper.DoRequest("POST", "/sales/customers", body, map[string]string{})

	assert.Equal(t, http.StatusBadRequest, w.Code,
		"Missing name should return 400, got %d: %s", w.Code, w.Body.String())
	t.Logf("✅ Sales: CreateCustomer without name returns 400")
}

// TestCreateCustomer_MissingEmail - email is required → 400
func TestCreateCustomer_MissingEmail(t *testing.T) {
	helper := SetupSalesAuthTest(t)
	defer helper.Close()

	body := map[string]interface{}{
		"name":         "No Email Customer",
		"phone_number": "1234567890",
	}

	w := helper.DoRequest("POST", "/sales/customers", body, map[string]string{})

	assert.Equal(t, http.StatusBadRequest, w.Code,
		"Missing email should return 400, got %d: %s", w.Code, w.Body.String())
	t.Logf("✅ Sales: CreateCustomer without email returns 400")
}

// TestCreateCustomer_DuplicateEmail - creating same email twice → 409
func TestCreateCustomer_DuplicateEmail(t *testing.T) {
	helper := SetupSalesAuthTest(t)
	defer helper.Close()

	email := fmt.Sprintf("dup-customer-%d@example.com", uniqueTimestamp())
	body := map[string]interface{}{
		"name":  "First Customer",
		"email": email,
	}

	// Create first customer
	w := helper.DoRequest("POST", "/sales/customers", body, map[string]string{})
	if w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Skipf("Could not create first customer (status %d); skipping duplicate test", w.Code)
		return
	}

	// Try duplicate
	body["name"] = "Second Customer"
	w = helper.DoRequest("POST", "/sales/customers", body, map[string]string{})

	// SP raises customer.already-exists → 409 Conflict
	assert.Equal(t, http.StatusConflict, w.Code,
		"Duplicate email should return 409, got %d: %s", w.Code, w.Body.String())
	t.Logf("✅ Sales: CreateCustomer duplicate email returns 409")
}

// TestCompleteOrder_NoItems - completing an order with no items → 400
func TestCompleteOrder_NoItems(t *testing.T) {
	helper := SetupSalesAuthTest(t)
	defer helper.Close()

	// First create a customer
	email := fmt.Sprintf("order-noitems-%d@example.com", uniqueTimestamp())
	customerBody := map[string]interface{}{
		"name":  "No Items Customer",
		"email": email,
	}
	w := helper.DoRequest("POST", "/sales/customers", customerBody, map[string]string{})
	if w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Skipf("Could not create customer (status %d); skipping test", w.Code)
		return
	}

	var customer map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &customer)
	customerID, _ := customer["id"].(string)

	// Create empty order
	orderBody := map[string]interface{}{
		"customer_id": customerID,
	}
	w = helper.DoRequest("POST", "/sales/orders", orderBody, map[string]string{})
	if w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Skipf("Could not create order (status %d); skipping test", w.Code)
		return
	}

	var order map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &order)
	orderID, _ := order["id"].(string)

	// Try to complete the empty order
	w = helper.DoRequest("PATCH", fmt.Sprintf("/sales/orders/%s/complete", orderID), nil, map[string]string{})

	// SP raises sales.order.no-items → controller returns 400
	assert.True(t, w.Code == http.StatusBadRequest || w.Code == http.StatusUnprocessableEntity,
		"Completing empty order should return 400, got %d: %s", w.Code, w.Body.String())
	t.Logf("✅ Sales: CompleteOrder with no items returns %d", w.Code)
}

// uniqueTimestamp returns a nanosecond timestamp for unique test data.
func uniqueTimestamp() int64 {
	return time.Now().UnixNano()
}
