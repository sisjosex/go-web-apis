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

// ============================================================================
// INV-010 — selling by SKU
// ============================================================================

// newSalesProduct creates a product with no variant axes and returns its ID.
func newSalesProduct(t *testing.T, helper *coreTestHelpers.ApiTestHelper, label string) string {
	t.Helper()

	body := map[string]interface{}{
		"sku":        fmt.Sprintf("INV010-%s-%d", label, uniqueTimestamp()),
		"name":       fmt.Sprintf("INV010 %s", label),
		"base_price": 10.0,
	}
	w := helper.DoRequest("POST", "/inventory/products", body, map[string]string{})
	if w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Skipf("could not create product (status %d): %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	id, _ := resp["product_id"].(string)
	if id == "" {
		id, _ = resp["id"].(string)
	}
	if id == "" {
		t.Skipf("product response carried no id: %s", w.Body.String())
	}
	return id
}

// addAxisAndGenerateSkus turns the product into a stock_by_variant one with two
// combinations. Batches created through POST /inventory/batches still land on
// the default SKU — that is what makes the per-combination stock case below
// reachable without touching the DB directly.
func addAxisAndGenerateSkus(t *testing.T, helper *coreTestHelpers.ApiTestHelper, productID string) {
	t.Helper()

	body := map[string]interface{}{
		"name":       "INV010 Polera",
		"base_price": 10.0,
		"variants": map[string]interface{}{
			"groups": []map[string]interface{}{
				{
					"group_type":        "Talla",
					"is_required":       true,
					"max_selections":    1,
					"affects_inventory": true,
					"options": []map[string]interface{}{
						{"name": "M", "modifier": 0},
						{"name": "XL", "modifier": 0},
					},
				},
			},
		},
	}
	w := helper.DoRequest("PUT", fmt.Sprintf("/inventory/products/%s", productID), body, map[string]string{})
	if w.Code != http.StatusOK && w.Code != http.StatusCreated {
		t.Skipf("could not add an axis (status %d): %s", w.Code, w.Body.String())
	}

	w = helper.DoRequest("POST", fmt.Sprintf("/inventory/products/%s/skus/generate", productID), nil, map[string]string{})
	if w.Code != http.StatusOK && w.Code != http.StatusCreated {
		t.Skipf("could not generate SKUs (status %d): %s", w.Code, w.Body.String())
	}
}

// productSkus lists a product's combinations, split into the default one and
// the generated ones.
func productSkus(t *testing.T, helper *coreTestHelpers.ApiTestHelper, productID string) (defaultSku string, generated []string) {
	t.Helper()

	w := helper.DoRequest("GET", fmt.Sprintf("/inventory/products/%s/skus", productID), nil, map[string]string{})
	if w.Code != http.StatusOK {
		t.Skipf("could not list SKUs (status %d): %s", w.Code, w.Body.String())
	}

	var resp struct {
		Skus []struct {
			SkuID     string `json:"sku_id"`
			IsDefault bool   `json:"is_default"`
		} `json:"skus"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)

	for _, s := range resp.Skus {
		if s.IsDefault {
			defaultSku = s.SkuID
		} else {
			generated = append(generated, s.SkuID)
		}
	}
	return defaultSku, generated
}

// stockBatchFor gives the product a batch. It lands on the product's default
// SKU: CreateBatchDto carries no sku_id, so fn_resolve_sku resolves NULL.
func stockBatchFor(t *testing.T, helper *coreTestHelpers.ApiTestHelper, productID string, qty float64) {
	t.Helper()

	body := map[string]interface{}{
		"product_id":       productID,
		"lot_number":       fmt.Sprintf("LOT-%d", uniqueTimestamp()),
		"purchase_date":    time.Now().Format("2006-01-02"),
		"expiry_date":      time.Now().AddDate(1, 0, 0).Format("2006-01-02"),
		"unit_cost":        5.0,
		"initial_quantity": qty,
	}
	w := helper.DoRequest("POST", "/inventory/batches", body, map[string]string{})
	if w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Skipf("could not create batch (status %d): %s", w.Code, w.Body.String())
	}
}

// newSalesOrder creates a customer and an empty order, returning the order ID.
func newSalesOrder(t *testing.T, helper *coreTestHelpers.ApiTestHelper) string {
	t.Helper()

	customerBody := map[string]interface{}{
		"name":         "INV010 Cliente",
		"email":        fmt.Sprintf("inv010-%d@example.com", uniqueTimestamp()),
		"phone_number": "1234567890",
		"address":      "Calle Uno 123",
		"city":         "Santiago",
		"state":        "RM",
		"postal_code":  "8320000",
		"country":      "Chile",
	}
	w := helper.DoRequest("POST", "/sales/customers", customerBody, map[string]string{})
	if w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Skipf("could not create customer (status %d): %s", w.Code, w.Body.String())
	}
	customerID, _ := orderItemFromResponse(t, w.Body.Bytes())["id"].(string)

	// Items is required by the DTO binding but nothing downstream reads it:
	// sp_create_sales_order takes no items and the repository passes none. It is
	// sent here only to satisfy the binder, and the order comes back empty.
	orderBody := map[string]interface{}{
		"customer_id":      customerID,
		"shipping_address": "Calle Uno 123",
		"items": []map[string]interface{}{
			{"product_id": "550e8400-e29b-41d4-a716-446655440001", "quantity": 1},
		},
	}
	w = helper.DoRequest("POST", "/sales/orders", orderBody, map[string]string{})
	if w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Skipf("could not create order (status %d): %s", w.Code, w.Body.String())
	}
	orderID, _ := orderItemFromResponse(t, w.Body.Bytes())["id"].(string)
	return orderID
}

// orderItemFromResponse pulls the {"data": {...}} order item out of a response.
func orderItemFromResponse(t *testing.T, body []byte) map[string]interface{} {
	t.Helper()

	var resp struct {
		Data map[string]interface{} `json:"data"`
	}
	json.Unmarshal(body, &resp)
	return resp.Data
}

// TestAddOrderItem_WithoutSku_Success — AC-2. A product that never had axes,
// posted exactly as every pre-INV-010 client posts it, still works, and the
// response now names the combination it sold.
func TestAddOrderItem_WithoutSku_Success(t *testing.T) {
	helper := SetupSalesAuthTest(t)
	defer helper.Close()

	productID := newSalesProduct(t, helper, "PLAIN")
	stockBatchFor(t, helper, productID, 10)
	orderID := newSalesOrder(t, helper)

	body := map[string]interface{}{
		"product_id": productID,
		"quantity":   2,
		"unit_price": 10.0,
	}
	w := helper.DoRequest("POST", fmt.Sprintf("/sales/orders/%s/items", orderID), body, map[string]string{})

	assert.Equal(t, http.StatusCreated, w.Code,
		"adding an item without sku_id should still return 201, got %d: %s", w.Code, w.Body.String())
	item := orderItemFromResponse(t, w.Body.Bytes())
	assert.NotEmpty(t, item["sku_id"], "the response should carry the resolved sku_id")
	assert.NotEmpty(t, item["sku"], "the response should carry the SKU code")
	t.Logf("✅ INV-010: add item without sku_id → 201, sku_id=%v sku=%v", item["sku_id"], item["sku"])
}

// TestAddOrderItem_WithSku_Success — naming the combination explicitly resolves
// to the same line as omitting it does on a product with a single combination.
func TestAddOrderItem_WithSku_Success(t *testing.T) {
	helper := SetupSalesAuthTest(t)
	defer helper.Close()

	productID := newSalesProduct(t, helper, "EXPLICIT")
	stockBatchFor(t, helper, productID, 10)
	defaultSku, _ := productSkus(t, helper, productID)
	orderID := newSalesOrder(t, helper)

	body := map[string]interface{}{
		"product_id": productID,
		"quantity":   1,
		"unit_price": 10.0,
		"sku_id":     defaultSku,
	}
	w := helper.DoRequest("POST", fmt.Sprintf("/sales/orders/%s/items", orderID), body, map[string]string{})

	assert.Equal(t, http.StatusCreated, w.Code,
		"adding an item with an explicit sku_id should return 201, got %d: %s", w.Code, w.Body.String())
	item := orderItemFromResponse(t, w.Body.Bytes())
	assert.Equal(t, defaultSku, item["sku_id"], "the line should sit on the SKU the client named")
	t.Logf("✅ INV-010: add item with sku_id → 201 on %v", item["sku_id"])
}

// TestAddOrderItem_SkuRequired — AC-3. On a stock_by_variant product, omitting
// the combination is a 400 and writes nothing.
func TestAddOrderItem_SkuRequired(t *testing.T) {
	helper := SetupSalesAuthTest(t)
	defer helper.Close()

	productID := newSalesProduct(t, helper, "VARIANT")
	addAxisAndGenerateSkus(t, helper, productID)
	stockBatchFor(t, helper, productID, 10)
	orderID := newSalesOrder(t, helper)

	body := map[string]interface{}{
		"product_id": productID,
		"quantity":   1,
		"unit_price": 10.0,
	}
	w := helper.DoRequest("POST", fmt.Sprintf("/sales/orders/%s/items", orderID), body, map[string]string{})

	assert.Equal(t, http.StatusBadRequest, w.Code,
		"a variant product without sku_id should return 400, got %d: %s", w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "sku-required")

	// and nothing was written
	w = helper.DoRequest("GET", fmt.Sprintf("/sales/orders/%s/with-batches", orderID), nil, map[string]string{})
	assert.NotContains(t, w.Body.String(), `"item_count":1`,
		"the rejected item should not have been created")
	t.Logf("✅ INV-010: variant product without sku_id → 400 sku-required")
}

// TestAddOrderItem_SkuOfAnotherProduct — a combination that belongs to a
// different product is a 404, not a 500.
func TestAddOrderItem_SkuOfAnotherProduct(t *testing.T) {
	helper := SetupSalesAuthTest(t)
	defer helper.Close()

	productA := newSalesProduct(t, helper, "OWNER")
	stockBatchFor(t, helper, productA, 10)

	productB := newSalesProduct(t, helper, "STRANGER")
	foreignSku, _ := productSkus(t, helper, productB)

	orderID := newSalesOrder(t, helper)
	body := map[string]interface{}{
		"product_id": productA,
		"quantity":   1,
		"unit_price": 10.0,
		"sku_id":     foreignSku,
	}
	w := helper.DoRequest("POST", fmt.Sprintf("/sales/orders/%s/items", orderID), body, map[string]string{})

	assert.Equal(t, http.StatusNotFound, w.Code,
		"a SKU of another product should return 404, got %d: %s", w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "sku-not-found")
	t.Logf("✅ INV-010: SKU of another product → 404 sku-not-found")
}

// TestAddOrderItem_InsufficientStockPerSku — AC-4, the point of the spec. The
// generated combination holds no batch while its sibling, the default SKU, holds
// ten. Before INV-010 the check answered for the product as a whole and this
// order went through.
func TestAddOrderItem_InsufficientStockPerSku(t *testing.T) {
	helper := SetupSalesAuthTest(t)
	defer helper.Close()

	productID := newSalesProduct(t, helper, "PERSKU")
	addAxisAndGenerateSkus(t, helper, productID)
	stockBatchFor(t, helper, productID, 10) // lands on the default SKU
	defaultSku, generated := productSkus(t, helper, productID)
	if len(generated) == 0 {
		t.Skip("no combination was generated; nothing to assert per SKU")
	}
	orderID := newSalesOrder(t, helper)

	// The sibling that does hold stock is sellable...
	body := map[string]interface{}{
		"product_id": productID,
		"quantity":   1,
		"unit_price": 10.0,
		"sku_id":     defaultSku,
	}
	w := helper.DoRequest("POST", fmt.Sprintf("/sales/orders/%s/items", orderID), body, map[string]string{})
	assert.Equal(t, http.StatusCreated, w.Code,
		"the SKU holding the batch should still sell, got %d: %s", w.Code, w.Body.String())

	// ...and the one that holds nothing is not, even though the product does.
	body["sku_id"] = generated[0]
	w = helper.DoRequest("POST", fmt.Sprintf("/sales/orders/%s/items", orderID), body, map[string]string{})

	assert.Equal(t, http.StatusConflict, w.Code,
		"a combination with no batches should fail even though a sibling has stock, got %d: %s",
		w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "insufficient-inventory")
	t.Logf("✅ INV-010: empty combination → 409 insufficient-inventory while its sibling sells")
}
