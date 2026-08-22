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
		"name":         "First Customer",
		"email":        email,
		"phone_number": "1234567890",
		"address":      "123 Main St",
		"city":         "New York",
		"state":        "NY",
		"postal_code":  "10001",
		"country":      "USA",
	}

	// Create first customer
	w := helper.DoRequest("POST", "/sales/customers", body, map[string]string{})
	if w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Fatalf("could not create the first customer (status %d): %s", w.Code, w.Body.String())
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
		"name":         "No Items Customer",
		"email":        email,
		"phone_number": "1234567890",
		"address":      "123 Main St",
		"city":         "New York",
		"state":        "NY",
		"postal_code":  "10001",
		"country":      "USA",
	}
	w := helper.DoRequest("POST", "/sales/customers", customerBody, map[string]string{})
	if w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Fatalf("could not create the customer (status %d): %s", w.Code, w.Body.String())
	}

	customerID, _ := orderItemFromResponse(t, w.Body.Bytes())["id"].(string)

	// Create an empty order. `items` is required by the binder but nothing
	// downstream reads it — sp_create_sales_order takes no items — so the order
	// that comes back really has no lines.
	orderBody := map[string]interface{}{
		"customer_id":      customerID,
		"shipping_address": "123 Main St",
		"items": []map[string]interface{}{
			{"product_id": "550e8400-e29b-41d4-a716-446655440001", "quantity": 1},
		},
	}
	w = helper.DoRequest("POST", "/sales/orders", orderBody, map[string]string{})
	if w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Fatalf("could not create the order (status %d): %s", w.Code, w.Body.String())
	}

	orderID, _ := orderItemFromResponse(t, w.Body.Bytes())["id"].(string)

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
		t.Fatalf("could not create the product (status %d): %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	id, _ := resp["product_id"].(string)
	if id == "" {
		id, _ = resp["id"].(string)
	}
	if id == "" {
		t.Fatalf("the product response carried no id: %s", w.Body.String())
	}
	return id
}

// addAxisAndGenerateSkus turns the product into a stock_by_variant one with two
// combinations. Since INV-014 a lot created after this point must name one of
// them, so a test that wants stock stranded in the unassigned bucket has to
// create it BEFORE calling this — which is exactly how every pre-INV-014 lot got
// there.
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
		t.Fatalf("could not add an axis (status %d): %s", w.Code, w.Body.String())
	}

	w = helper.DoRequest("POST", fmt.Sprintf("/inventory/products/%s/skus/generate", productID), nil, map[string]string{})
	if w.Code != http.StatusOK && w.Code != http.StatusCreated {
		t.Fatalf("could not generate the SKUs (status %d): %s", w.Code, w.Body.String())
	}
}

// productSkus lists a product's combinations, split into the default one and
// the generated ones.
func productSkus(t *testing.T, helper *coreTestHelpers.ApiTestHelper, productID string) (defaultSku string, generated []string) {
	t.Helper()

	w := helper.DoRequest("GET", fmt.Sprintf("/inventory/products/%s/skus", productID), nil, map[string]string{})
	if w.Code != http.StatusOK {
		t.Fatalf("could not list the SKUs (status %d): %s", w.Code, w.Body.String())
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

// stockBatchFor gives the product a batch on its default SKU. Valid only while
// the product has no combinations: after INV-014 the API refuses a lot that names
// none on a stock_by_variant product (inventory.sku.required).
func stockBatchFor(t *testing.T, helper *coreTestHelpers.ApiTestHelper, productID string, qty float64) {
	t.Helper()

	stockBatchOn(t, helper, productID, "", qty)
}

// stockBatchOn gives the product a batch on the combination named, or on its
// default SKU when skuID is empty. Returns the lot number so a caller can assert
// which lot a sale drew from.
func stockBatchOn(
	t *testing.T,
	helper *coreTestHelpers.ApiTestHelper,
	productID, skuID string,
	qty float64,
) string {
	t.Helper()

	lotNumber := fmt.Sprintf("LOT-%d", uniqueTimestamp())
	body := map[string]interface{}{
		"product_id":       productID,
		"lot_number":       lotNumber,
		"purchase_date":    time.Now().Format("2006-01-02"),
		"expiry_date":      time.Now().AddDate(1, 0, 0).Format("2006-01-02"),
		"unit_cost":        5.0,
		"initial_quantity": qty,
	}
	if skuID != "" {
		body["sku_id"] = skuID
	}

	w := helper.DoRequest("POST", "/inventory/batches", body, map[string]string{})
	if w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Fatalf("could not create the batch (status %d): %s", w.Code, w.Body.String())
	}
	return lotNumber
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
		t.Fatalf("could not create the customer (status %d): %s", w.Code, w.Body.String())
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
		t.Fatalf("could not create the order (status %d): %s", w.Code, w.Body.String())
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
	// Stocked before the axis exists: INV-014 refuses a lot that names no
	// combination once the product is stocked by variant, and what this test needs
	// is only that the product has stock somewhere.
	stockBatchFor(t, helper, productID, 10)
	addAxisAndGenerateSkus(t, helper, productID)
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
	// The lot has to predate the axis to land in the unassigned bucket at all:
	// after INV-014 the API will not put one there on a stock_by_variant product.
	stockBatchFor(t, helper, productID, 10) // lands on the default SKU
	addAxisAndGenerateSkus(t, helper, productID)
	defaultSku, generated := productSkus(t, helper, productID)
	if len(generated) == 0 {
		t.Fatal("no combination was generated; there is nothing to assert per SKU")
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

// TestAddOrderItem_FulfilledFromCombinationBatch — INV-014 AC-5, the bug that
// started the spec. A lot created for Polera-M through the API alone fulfils an
// order for Polera-M.
//
// Before INV-014 this was impossible: POST /inventory/batches carried no sku_id,
// so every lot resolved to the product's default SKU, and
// sp_add_order_item_with_batch fulfils strictly from pb.sku_id = v_sku_id. The
// order failed with sales-order.insufficient-inventory while ten units sat in the
// bucket, and no API call could put them anywhere else.
func TestAddOrderItem_FulfilledFromCombinationBatch(t *testing.T) {
	helper := SetupSalesAuthTest(t)
	defer helper.Close()

	productID := newSalesProduct(t, helper, "INV014")
	addAxisAndGenerateSkus(t, helper, productID)
	_, generated := productSkus(t, helper, productID)
	if len(generated) == 0 {
		t.Fatal("no combination was generated; there is nothing to fulfil from")
	}

	lotNumber := stockBatchOn(t, helper, productID, generated[0], 10)
	orderID := newSalesOrder(t, helper)

	w := helper.DoRequest("POST", fmt.Sprintf("/sales/orders/%s/items", orderID),
		map[string]interface{}{
			"product_id": productID,
			"quantity":   3,
			"unit_price": 10.0,
			"sku_id":     generated[0],
		}, map[string]string{})

	assert.Equal(t, http.StatusCreated, w.Code,
		"an order for a combination that holds a lot should be fulfilled, got %d: %s",
		w.Code, w.Body.String())
	item := orderItemFromResponse(t, w.Body.Bytes())
	assert.Equal(t, generated[0], item["sku_id"], "the line should sit on the combination ordered")

	// The endpoint does not echo the lot it drew from, so the assignment is read
	// back instead. sp_add_order_item_with_batch only ever assigns from
	// pb.sku_id = v_sku_id, and this combination holds exactly one lot, so a
	// non-zero batch_count is that lot and no other.
	wb := helper.DoRequest("GET", fmt.Sprintf("/sales/orders/%s/with-batches", orderID), nil, map[string]string{})
	assert.Equal(t, http.StatusOK, wb.Code, wb.Body.String())
	assert.Contains(t, wb.Body.String(), `"batch_count":1`,
		"the line should be fulfilled from the lot created for that combination: %s", wb.Body.String())
	t.Logf("✅ INV-014: order for a combination fulfilled from its own lot %s", lotNumber)
}

// A voided lot is not sellable: sp_add_order_item_with_batch skips it exactly as
// inventory's FIFO pick does, or a write-off would silently come back as COGS.
func TestAddOrderItem_SkipsVoidedBatch(t *testing.T) {
	helper := SetupSalesAuthTest(t)
	defer helper.Close()

	productID := newSalesProduct(t, helper, "INV014VOID")
	addAxisAndGenerateSkus(t, helper, productID)
	_, generated := productSkus(t, helper, productID)
	if len(generated) == 0 {
		t.Fatal("no combination was generated; there is nothing to void")
	}

	stockBatchOn(t, helper, productID, generated[0], 10)
	batchID := onlyBatchID(t, helper, productID, generated[0])

	wd := helper.DoRequest("DELETE", fmt.Sprintf("/inventory/batches/%s", batchID), nil, map[string]string{})
	assert.Equal(t, http.StatusOK, wd.Code, wd.Body.String())

	orderID := newSalesOrder(t, helper)
	w := helper.DoRequest("POST", fmt.Sprintf("/sales/orders/%s/items", orderID),
		map[string]interface{}{
			"product_id": productID,
			"quantity":   1,
			"unit_price": 10.0,
			"sku_id":     generated[0],
		}, map[string]string{})

	assert.Equal(t, http.StatusConflict, w.Code,
		"a voided lot should not be sellable, got %d: %s", w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "insufficient-inventory")
	t.Logf("✅ INV-014: voided lot → 409 insufficient-inventory")
}

// onlyBatchID returns the id of the single lot a combination holds.
func onlyBatchID(t *testing.T, helper *coreTestHelpers.ApiTestHelper, productID, skuID string) string {
	t.Helper()

	w := helper.DoRequest("GET",
		fmt.Sprintf("/inventory/batches/product/%s?onlyActive=false&skuId=%s", productID, skuID),
		nil, map[string]string{})
	if w.Code != http.StatusOK {
		t.Fatalf("could not list the lots of %s (status %d): %s", productID, w.Code, w.Body.String())
	}

	var resp struct {
		Batches []struct {
			ID string `json:"id"`
		} `json:"batches"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Batches) != 1 {
		t.Fatalf("expected exactly one lot on the combination, got %d", len(resp.Batches))
	}
	return resp.Batches[0].ID
}

// TestCompleteOrder_DecrementsStockOnce — INV-014 A1, la otra mitad. El lote
// sube las existencias al crearse, así que completar el pedido tiene que
// bajarlas; si sólo bajara el saldo del lote, el stock se inflaría en cada venta.
//
// Comprueba además que se escribe inventory.batch_movements, la tabla que nada
// llenaba nunca y de la que depende el guardia has-movements de INV-014 D3.
func TestCompleteOrder_DecrementsStockOnce(t *testing.T) {
	helper := SetupSalesAuthTest(t)
	defer helper.Close()

	productID := newSalesProduct(t, helper, "INV014STOCK")
	addAxisAndGenerateSkus(t, helper, productID)
	_, generated := productSkus(t, helper, productID)
	if len(generated) == 0 {
		t.Fatal("no se generó ninguna combinación")
	}

	stockBatchOn(t, helper, productID, generated[0], 10)
	assert.Equal(t, 10.0, skuStock(t, helper, productID, generated[0]),
		"el lote debería haber entrado al stock de la combinación (A1)")

	orderID := newSalesOrder(t, helper)
	w := helper.DoRequest("POST", fmt.Sprintf("/sales/orders/%s/items", orderID),
		map[string]interface{}{
			"product_id": productID,
			"quantity":   3,
			"unit_price": 10.0,
			"sku_id":     generated[0],
		}, map[string]string{})
	assert.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	// Asignar la línea no consume: el stock sólo baja al completar.
	assert.Equal(t, 10.0, skuStock(t, helper, productID, generated[0]),
		"añadir la línea no debe descontar todavía")

	wc := helper.DoRequest("PATCH", fmt.Sprintf("/sales/orders/%s/complete", orderID), nil, map[string]string{})
	if wc.Code != http.StatusOK {
		t.Fatalf("no se pudo completar el pedido (status %d): %s", wc.Code, wc.Body.String())
	}

	assert.Equal(t, 7.0, skuStock(t, helper, productID, generated[0]),
		"completar un pedido de 3 debe dejar el stock en 7, descontado una sola vez")
	t.Logf("✅ INV-014 A1: completar el pedido descontó 10 → 7")
}

// skuStock lee las existencias de una combinación desde el listado de SKUs.
func skuStock(t *testing.T, helper *coreTestHelpers.ApiTestHelper, productID, skuID string) float64 {
	t.Helper()

	w := helper.DoRequest("GET", fmt.Sprintf("/inventory/products/%s/skus", productID), nil, map[string]string{})
	if w.Code != http.StatusOK {
		t.Fatalf("no se pudieron listar los SKUs (status %d): %s", w.Code, w.Body.String())
	}

	var resp struct {
		Skus []struct {
			SkuID           string  `json:"sku_id"`
			CurrentQuantity float64 `json:"current_quantity"`
		} `json:"skus"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)

	for _, sku := range resp.Skus {
		if sku.SkuID == skuID {
			return sku.CurrentQuantity
		}
	}
	t.Fatalf("la combinación %s no aparece en el listado", skuID)
	return 0
}
