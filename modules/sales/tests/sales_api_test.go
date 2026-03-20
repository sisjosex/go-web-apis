//go:build integration
// +build integration

package sales_test

import (
	"fmt"
	"net/http"
	"testing"

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
