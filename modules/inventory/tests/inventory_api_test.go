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

	"github.com/stretchr/testify/assert"
)

func init() {
	// Initialize test environment (load .env.test before any config)
	testhelpers.InitTestEnvironment()
}

// ============================================
// Product Creation Tests
// ============================================

func TestCreateProduct_Success(t *testing.T) {
	helper := testhelpers.SetupApiTest(t)
	defer helper.Close()

	body := models.CreateProductDto{
		SKU:       "TEST001",
		Name:      "Test Product",
		BasePrice: 10.00,
	}

	w := helper.DoRequest("POST", "/inventory/products", body, map[string]string{})
	if w.Code != http.StatusCreated {
		t.Logf("Error response: %s", w.Body.String())
	}
	assert.Equal(t, http.StatusCreated, w.Code)

	var result models.CreateProductResponse
	json.Unmarshal(w.Body.Bytes(), &result)
	assert.NotEmpty(t, result.ProductID)
	assert.Equal(t, "TEST001", result.SKU)
}

func TestCreateProduct_WithVariants(t *testing.T) {
	helper := testhelpers.SetupApiTest(t)
	defer helper.Close()

	variants := map[string]interface{}{
		"groups": []map[string]interface{}{
			{
				"group_type":     "flavor",
				"is_required":    true,
				"max_selections": 1,
				"options": []map[string]interface{}{
					{"name": "Vanilla", "modifier": 0.00},
					{"name": "Chocolate", "modifier": 0.00},
					{"name": "Strawberry", "modifier": 0.00},
				},
			},
			{
				"group_type":     "topping",
				"is_required":    false,
				"max_selections": 3,
				"options": []map[string]interface{}{
					{"name": "Sprinkles", "modifier": 1.00},
					{"name": "Chocolate Chips", "modifier": 1.50},
					{"name": "Nuts", "modifier": 2.00},
				},
			},
		},
	}

	body := models.CreateProductDto{
		SKU:       "ICECREAM001",
		Name:      "Classic Ice Cream",
		BasePrice: 3.00,
		Variants:  variants,
	}

	w := helper.DoRequest("POST", "/inventory/products", body, map[string]string{})
	assert.Equal(t, http.StatusCreated, w.Code)

	var result models.CreateProductResponse
	json.Unmarshal(w.Body.Bytes(), &result)
	assert.NotEmpty(t, result.ProductID)
}

func TestCreateProduct_SKUAlreadyExists(t *testing.T) {
	helper := testhelpers.SetupApiTest(t)
	defer helper.Close()

	// Create first product
	body1 := models.CreateProductDto{
		SKU:       "DUP001",
		Name:      "Product 1",
		BasePrice: 10.00,
	}
	w1 := helper.DoRequest("POST", "/inventory/products", body1, map[string]string{})
	assert.Equal(t, http.StatusCreated, w1.Code)

	// Try to create duplicate
	body2 := models.CreateProductDto{
		SKU:       "DUP001",
		Name:      "Product 2",
		BasePrice: 15.00,
	}
	w2 := helper.DoRequest("POST", "/inventory/products", body2, map[string]string{})
	assert.Equal(t, http.StatusConflict, w2.Code)
}

func TestCreateProduct_InvalidPrice(t *testing.T) {
	helper := testhelpers.SetupApiTest(t)
	defer helper.Close()

	body := models.CreateProductDto{
		SKU:       "INVALID001",
		Name:      "Invalid Product",
		BasePrice: -10.00,
	}

	w := helper.DoRequest("POST", "/inventory/products", body, map[string]string{})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateProduct_MissingRequiredFields(t *testing.T) {
	helper := testhelpers.SetupApiTest(t)
	defer helper.Close()

	body := map[string]interface{}{
		"name": "Missing SKU",
	}

	w := helper.DoRequest("POST", "/inventory/products", body, map[string]string{})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ============================================
// Product Retrieval Tests
// ============================================

func TestGetProduct_Success(t *testing.T) {
	helper := testhelpers.SetupApiTest(t)
	defer helper.Close()

	// Create product first
	body := models.CreateProductDto{
		SKU:       "GET001",
		Name:      "Get Test Product",
		BasePrice: 25.00,
	}
	w1 := helper.DoRequest("POST", "/inventory/products", body, map[string]string{})
	assert.Equal(t, http.StatusCreated, w1.Code)

	var created models.CreateProductResponse
	json.Unmarshal(w1.Body.Bytes(), &created)

	// Now get it
	w2 := helper.DoRequest("GET", fmt.Sprintf("/inventory/products/%s", created.ProductID), nil, map[string]string{})
	assert.Equal(t, http.StatusOK, w2.Code)

	var product models.Product
	json.Unmarshal(w2.Body.Bytes(), &product)
	assert.Equal(t, "GET001", product.SKU)
	assert.Equal(t, "Get Test Product", product.Name)
}

func TestGetProduct_NotFound(t *testing.T) {
	helper := testhelpers.SetupApiTest(t)
	defer helper.Close()

	w := helper.DoRequest("GET", "/inventory/products/00000000-0000-0000-0000-000000000000", nil, map[string]string{})
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetProductBySkU_Success(t *testing.T) {
	helper := testhelpers.SetupApiTest(t)
	defer helper.Close()

	// Create product
	body := models.CreateProductDto{
		SKU:       "SKU001",
		Name:      "SKU Test Product",
		BasePrice: 30.00,
	}
	helper.DoRequest("POST", "/inventory/products", body, map[string]string{})

	// Get by SKU
	w := helper.DoRequest("GET", "/inventory/products/sku/SKU001", nil, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code)

	var product models.Product
	json.Unmarshal(w.Body.Bytes(), &product)
	assert.Equal(t, "SKU001", product.SKU)
}

func TestListProducts_Success(t *testing.T) {
	helper := testhelpers.SetupApiTest(t)
	defer helper.Close()

	// Create a few products
	for i := 1; i <= 3; i++ {
		body := models.CreateProductDto{
			SKU:       fmt.Sprintf("LIST%03d", i),
			Name:      fmt.Sprintf("List Product %d", i),
			BasePrice: float64(10 + i),
		}
		helper.DoRequest("POST", "/inventory/products", body, map[string]string{})
	}

	// List products
	w := helper.DoRequest("GET", "/inventory/products?limit=10&offset=0", nil, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code)

	var result map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &result)
	assert.NotNil(t, result["data"])
}

// ============================================
// Inventory Movement Tests
// ============================================

func TestRecordMovement_Purchase(t *testing.T) {
	helper := testhelpers.SetupApiTest(t)
	defer helper.Close()

	// Create product
	body := models.CreateProductDto{
		SKU:       "MOVE001",
		Name:      "Movement Test",
		BasePrice: 50.00,
	}
	w1 := helper.DoRequest("POST", "/inventory/products", body, map[string]string{})
	var created models.CreateProductResponse
	json.Unmarshal(w1.Body.Bytes(), &created)

	// Record purchase movement
	moveBody := models.RecordMovementDto{
		ProductID:    created.ProductID,
		MovementType: "PURCHASE",
		Quantity:     100.0,
		UnitCost:     ptrFloat64(25.00),
	}

	w2 := helper.DoRequest("POST", "/inventory/movements", moveBody, map[string]string{})
	if w2.Code != http.StatusCreated {
		t.Logf("Error response: %s", w2.Body.String())
	}
	assert.Equal(t, http.StatusCreated, w2.Code)

	var movement models.RecordMovementResponse
	json.Unmarshal(w2.Body.Bytes(), &movement)
	assert.Equal(t, 100.0, movement.NewStockQuantity)
}

func TestRecordMovement_Sale(t *testing.T) {
	helper := testhelpers.SetupApiTest(t)
	defer helper.Close()

	// Create and purchase product
	body := models.CreateProductDto{
		SKU:       "SALE001",
		Name:      "Sale Test",
		BasePrice: 20.00,
	}
	w1 := helper.DoRequest("POST", "/inventory/products", body, map[string]string{})
	var created models.CreateProductResponse
	json.Unmarshal(w1.Body.Bytes(), &created)

	// Purchase
	purchaseBody := models.RecordMovementDto{
		ProductID:    created.ProductID,
		MovementType: "PURCHASE",
		Quantity:     50.0,
	}
	helper.DoRequest("POST", "/inventory/movements", purchaseBody, map[string]string{})

	// Sale
	saleBody := models.RecordMovementDto{
		ProductID:    created.ProductID,
		MovementType: "SALE",
		Quantity:     -20.0,
	}
	w2 := helper.DoRequest("POST", "/inventory/movements", saleBody, map[string]string{})
	assert.Equal(t, http.StatusCreated, w2.Code)

	var movement models.RecordMovementResponse
	json.Unmarshal(w2.Body.Bytes(), &movement)
	assert.Equal(t, 30.0, movement.NewStockQuantity)
}

func TestRecordMovement_InsufficientStock(t *testing.T) {
	helper := testhelpers.SetupApiTest(t)
	defer helper.Close()

	// Create product
	body := models.CreateProductDto{
		SKU:       "INSUF001",
		Name:      "Insufficient Stock Test",
		BasePrice: 15.00,
	}
	w1 := helper.DoRequest("POST", "/inventory/products", body, map[string]string{})
	var created models.CreateProductResponse
	json.Unmarshal(w1.Body.Bytes(), &created)

	// Try to sell without purchase
	saleBody := models.RecordMovementDto{
		ProductID:    created.ProductID,
		MovementType: "SALE",
		Quantity:     -50.0,
	}
	w2 := helper.DoRequest("POST", "/inventory/movements", saleBody, map[string]string{})
	assert.Equal(t, http.StatusConflict, w2.Code)
}

func TestRecordMovement_InvalidType(t *testing.T) {
	helper := testhelpers.SetupApiTest(t)
	defer helper.Close()

	// Create product
	body := models.CreateProductDto{
		SKU:       "INVTYPE001",
		Name:      "Invalid Type Test",
		BasePrice: 12.00,
	}
	w1 := helper.DoRequest("POST", "/inventory/products", body, map[string]string{})
	assert.Equal(t, http.StatusCreated, w1.Code)

	var created models.CreateProductResponse
	json.Unmarshal(w1.Body.Bytes(), &created)
	assert.NotEmpty(t, created.ProductID)

	// Invalid movement type
	moveBody := models.RecordMovementDto{
		ProductID:    created.ProductID,
		MovementType: "INVALID",
		Quantity:     10.0,
	}
	w2 := helper.DoRequest("POST", "/inventory/movements", moveBody, map[string]string{})
	assert.Equal(t, http.StatusBadRequest, w2.Code)
}

func TestRecordMovement_ProductNotFound(t *testing.T) {
	helper := testhelpers.SetupApiTest(t)
	defer helper.Close()

	moveBody := models.RecordMovementDto{
		ProductID:    "00000000-0000-0000-0000-000000000000",
		MovementType: "PURCHASE",
		Quantity:     10.0,
	}
	w := helper.DoRequest("POST", "/inventory/movements", moveBody, map[string]string{})
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ============================================
// Stock Query Tests
// ============================================

func TestGetProductStock_Success(t *testing.T) {
	helper := testhelpers.SetupApiTest(t)
	defer helper.Close()

	// Create and purchase
	body := models.CreateProductDto{
		SKU:       "STOCK001",
		Name:      "Stock Test",
		BasePrice: 18.00,
	}
	w1 := helper.DoRequest("POST", "/inventory/products", body, map[string]string{})
	var created models.CreateProductResponse
	json.Unmarshal(w1.Body.Bytes(), &created)

	moveBody := models.RecordMovementDto{
		ProductID:    created.ProductID,
		MovementType: "PURCHASE",
		Quantity:     75.0,
	}
	helper.DoRequest("POST", "/inventory/movements", moveBody, map[string]string{})

	// Get stock
	w2 := helper.DoRequest("GET", fmt.Sprintf("/inventory/stock/%s", created.ProductID), nil, map[string]string{})
	assert.Equal(t, http.StatusOK, w2.Code)

	var stock models.ProductStock
	json.Unmarshal(w2.Body.Bytes(), &stock)
	assert.Equal(t, 75.0, stock.CurrentQuantity)
}

// ============================================
// Reserved Quantity Tests
// ============================================

func TestReserveStock_Success(t *testing.T) {
	helper := testhelpers.SetupApiTest(t)
	defer helper.Close()

	// Create product with unique SKU
	productBody := models.CreateProductDto{
		SKU:       fmt.Sprintf("RESERVE_%d", time.Now().UnixNano()),
		Name:      "Reserve Test Product",
		BasePrice: 50.00,
	}
	w := helper.DoRequest("POST", "/inventory/products", productBody, map[string]string{})
	t.Logf("Create product response: status=%d, body=%s", w.Code, w.Body.String())

	var created models.CreateProductResponse
	json.Unmarshal(w.Body.Bytes(), &created)
	t.Logf("Created product: ID=%s, SKU=%s", created.ProductID, created.SKU)

	if created.ProductID == "" {
		t.Fatalf("ProductID is empty, cannot continue test")
	}

	// Add initial stock: 100 units
	moveBody := models.RecordMovementDto{
		ProductID:    created.ProductID,
		MovementType: "PURCHASE",
		Quantity:     100.0,
	}
	moveResp := helper.DoRequest("POST", "/inventory/movements", moveBody, map[string]string{})
	t.Logf("Movement response: status=%d, body=%s", moveResp.Code, moveResp.Body.String())

	if moveResp.Code != http.StatusCreated {
		t.Fatalf("Failed to create movement: status=%d", moveResp.Code)
	}

	// Verify initial stock
	w1 := helper.DoRequest("GET", fmt.Sprintf("/inventory/stock/%s", created.ProductID), nil, map[string]string{})
	var stock1 models.ProductStock
	json.Unmarshal(w1.Body.Bytes(), &stock1)
	t.Logf("Initial stock after purchase: current=%v, reserved=%v, available=%v, status=%v", stock1.CurrentQuantity, stock1.ReservedQuantity, stock1.AvailableQuantity, stock1.Status)

	if stock1.CurrentQuantity != 100.0 {
		t.Fatalf("Expected current quantity 100, got %v", stock1.CurrentQuantity)
	}

	// Reserve 25 units (simulating a sales order)
	reserveBody := map[string]interface{}{
		"product_id": created.ProductID,
		"quantity":   25.0,
	}
	w2 := helper.DoRequest("POST", "/inventory/reserve", reserveBody, map[string]string{})
	t.Logf("Reserve response status: %d, body: %s", w2.Code, w2.Body.String())
	assert.Equal(t, http.StatusOK, w2.Code)

	var reservation map[string]interface{}
	json.Unmarshal(w2.Body.Bytes(), &reservation)
	assert.Equal(t, 25.0, reservation["reserved_quantity"])
	assert.Equal(t, 75.0, reservation["available_quantity"]) // 100 - 25
	assert.Equal(t, "ok", reservation["status"])

	// Verify stock is updated
	w3 := helper.DoRequest("GET", fmt.Sprintf("/inventory/stock/%s", created.ProductID), nil, map[string]string{})
	var stock2 models.ProductStock
	json.Unmarshal(w3.Body.Bytes(), &stock2)
	assert.Equal(t, 100.0, stock2.CurrentQuantity)
	assert.Equal(t, 25.0, stock2.ReservedQuantity)
	assert.Equal(t, 75.0, stock2.AvailableQuantity)
	assert.Equal(t, "ok", stock2.Status)
}

func TestReserveStock_InsufficientStock(t *testing.T) {
	helper := testhelpers.SetupApiTest(t)
	defer helper.Close()

	// Create product with 20 units
	productBody := models.CreateProductDto{
		SKU:       "RESERVE002",
		Name:      "Insufficient Stock Test",
		BasePrice: 50.00,
	}
	w := helper.DoRequest("POST", "/inventory/products", productBody, map[string]string{})
	var created models.CreateProductResponse
	json.Unmarshal(w.Body.Bytes(), &created)

	// Add only 20 units
	moveBody := models.RecordMovementDto{
		ProductID:    created.ProductID,
		MovementType: "PURCHASE",
		Quantity:     20.0,
	}
	helper.DoRequest("POST", "/inventory/movements", moveBody, map[string]string{})

	// Try to reserve 25 (more than available)
	reserveBody := map[string]interface{}{
		"product_id": created.ProductID,
		"quantity":   25.0,
	}
	w2 := helper.DoRequest("POST", "/inventory/reserve", reserveBody, map[string]string{})
	assert.Equal(t, http.StatusConflict, w2.Code) // Should fail
}

func TestReleaseReservedStock_Success(t *testing.T) {
	helper := testhelpers.SetupApiTest(t)
	defer helper.Close()

	// Create product with 100 units
	productBody := models.CreateProductDto{
		SKU:       "RELEASE001",
		Name:      "Release Test Product",
		BasePrice: 50.00,
	}
	w := helper.DoRequest("POST", "/inventory/products", productBody, map[string]string{})
	var created models.CreateProductResponse
	json.Unmarshal(w.Body.Bytes(), &created)

	// Add 100 units
	moveBody := models.RecordMovementDto{
		ProductID:    created.ProductID,
		MovementType: "PURCHASE",
		Quantity:     100.0,
	}
	helper.DoRequest("POST", "/inventory/movements", moveBody, map[string]string{})

	// Reserve 40 units
	reserveBody := map[string]interface{}{
		"product_id": created.ProductID,
		"quantity":   40.0,
	}
	helper.DoRequest("POST", "/inventory/reserve", reserveBody, map[string]string{})

	// Verify reserved
	w1 := helper.DoRequest("GET", fmt.Sprintf("/inventory/stock/%s", created.ProductID), nil, map[string]string{})
	var stock1 models.ProductStock
	json.Unmarshal(w1.Body.Bytes(), &stock1)
	assert.Equal(t, 40.0, stock1.ReservedQuantity)

	// Release 20 units (cancel partial order)
	releaseBody := map[string]interface{}{
		"product_id": created.ProductID,
		"quantity":   20.0,
	}
	w2 := helper.DoRequest("POST", "/inventory/release-reserved", releaseBody, map[string]string{})
	assert.Equal(t, http.StatusOK, w2.Code)

	var release map[string]interface{}
	json.Unmarshal(w2.Body.Bytes(), &release)
	assert.Equal(t, 20.0, release["reserved_quantity"])  // 40 - 20
	assert.Equal(t, 80.0, release["available_quantity"]) // 100 - 20

	// Verify stock
	w3 := helper.DoRequest("GET", fmt.Sprintf("/inventory/stock/%s", created.ProductID), nil, map[string]string{})
	var stock2 models.ProductStock
	json.Unmarshal(w3.Body.Bytes(), &stock2)
	assert.Equal(t, 100.0, stock2.CurrentQuantity)
	assert.Equal(t, 20.0, stock2.ReservedQuantity)
	assert.Equal(t, 80.0, stock2.AvailableQuantity)
}

// ============================================
// Reorder Level Tests
// ============================================

func TestUpdateReorderLevel_Success(t *testing.T) {
	helper := testhelpers.SetupApiTest(t)
	defer helper.Close()

	// Create product
	productBody := models.CreateProductDto{
		SKU:       fmt.Sprintf("REORDER_%d", time.Now().UnixNano()),
		Name:      "Reorder Test Product",
		BasePrice: 50.00,
	}
	w := helper.DoRequest("POST", "/inventory/products", productBody, map[string]string{})
	var created models.CreateProductResponse
	json.Unmarshal(w.Body.Bytes(), &created)

	// Add 50 units (reorder_level defaults to 10)
	moveBody := models.RecordMovementDto{
		ProductID:    created.ProductID,
		MovementType: "PURCHASE",
		Quantity:     50.0,
	}
	helper.DoRequest("POST", "/inventory/movements", moveBody, map[string]string{})

	// Verify initial status is "ok"
	w1 := helper.DoRequest("GET", fmt.Sprintf("/inventory/stock/%s", created.ProductID), nil, map[string]string{})
	var stock1 models.ProductStock
	json.Unmarshal(w1.Body.Bytes(), &stock1)
	assert.Equal(t, "ok", stock1.Status)
	assert.Equal(t, 10.0, stock1.ReorderLevel) // default

	// Update reorder level to 40
	updateBody := map[string]interface{}{
		"reorder_level": 40.0,
	}
	w2 := helper.DoRequest("PATCH", fmt.Sprintf("/inventory/products/%s/reorder-level", created.ProductID), updateBody, map[string]string{})
	t.Logf("Update reorder level response: status=%d, body=%s", w2.Code, w2.Body.String())
	assert.Equal(t, http.StatusOK, w2.Code)

	var updated map[string]interface{}
	json.Unmarshal(w2.Body.Bytes(), &updated)
	assert.Equal(t, 40.0, updated["reorder_level"])
	assert.Equal(t, "low", updated["status"]) // 50 < (40*1.5=60) but >= 40

	// Verify stock
	w3 := helper.DoRequest("GET", fmt.Sprintf("/inventory/stock/%s", created.ProductID), nil, map[string]string{})
	var stock2 models.ProductStock
	json.Unmarshal(w3.Body.Bytes(), &stock2)
	assert.Equal(t, 40.0, stock2.ReorderLevel)
	assert.Equal(t, "low", stock2.Status)
}

func TestReorderLevel_StatusTransitions(t *testing.T) {
	helper := testhelpers.SetupApiTest(t)
	defer helper.Close()

	// Create product with reorder_level = 20
	productBody := models.CreateProductDto{
		SKU:       "STATUS001",
		Name:      "Status Transition Test",
		BasePrice: 50.00,
	}
	w := helper.DoRequest("POST", "/inventory/products", productBody, map[string]string{})
	var created models.CreateProductResponse
	json.Unmarshal(w.Body.Bytes(), &created)

	// Set reorder_level to 20
	updateBody := map[string]interface{}{
		"reorder_level": 20.0,
	}
	helper.DoRequest("PATCH", fmt.Sprintf("/inventory/products/%s/reorder-level", created.ProductID), updateBody, map[string]string{})

	// Add 100 units
	moveBody := models.RecordMovementDto{
		ProductID:    created.ProductID,
		MovementType: "PURCHASE",
		Quantity:     100.0,
	}
	helper.DoRequest("POST", "/inventory/movements", moveBody, map[string]string{})

	// Test: 100 >= 30 → should be "ok"
	w1 := helper.DoRequest("GET", fmt.Sprintf("/inventory/stock/%s", created.ProductID), nil, map[string]string{})
	var stock1 models.ProductStock
	json.Unmarshal(w1.Body.Bytes(), &stock1)
	assert.Equal(t, "ok", stock1.Status)

	// Sell 45 units: 100 - 45 = 55, still >= 30 → "ok"
	saleBody := models.RecordMovementDto{
		ProductID:    created.ProductID,
		MovementType: "SALE",
		Quantity:     -45.0,
	}
	helper.DoRequest("POST", "/inventory/movements", saleBody, map[string]string{})

	w2 := helper.DoRequest("GET", fmt.Sprintf("/inventory/stock/%s", created.ProductID), nil, map[string]string{})
	var stock2 models.ProductStock
	json.Unmarshal(w2.Body.Bytes(), &stock2)
	assert.Equal(t, 55.0, stock2.CurrentQuantity)
	assert.Equal(t, "ok", stock2.Status)

	// Sell 20 more: 55 - 20 = 35, but < 20*1.5 = 30 → should be "low"
	saleBody2 := models.RecordMovementDto{
		ProductID:    created.ProductID,
		MovementType: "SALE",
		Quantity:     -25.0,
	}
	helper.DoRequest("POST", "/inventory/movements", saleBody2, map[string]string{})

	w3 := helper.DoRequest("GET", fmt.Sprintf("/inventory/stock/%s", created.ProductID), nil, map[string]string{})
	var stock3 models.ProductStock
	json.Unmarshal(w3.Body.Bytes(), &stock3)
	assert.Equal(t, 30.0, stock3.CurrentQuantity)
	// 30 is NOT < 20*1.5 (30), so status should be "ok"
	assert.Equal(t, "ok", stock3.Status)

	// Sell 5 more: 30 - 5 = 25, still ok but getting close
	saleBody2b := models.RecordMovementDto{
		ProductID:    created.ProductID,
		MovementType: "SALE",
		Quantity:     -5.0,
	}
	helper.DoRequest("POST", "/inventory/movements", saleBody2b, map[string]string{})

	w3b := helper.DoRequest("GET", fmt.Sprintf("/inventory/stock/%s", created.ProductID), nil, map[string]string{})
	var stock3b models.ProductStock
	json.Unmarshal(w3b.Body.Bytes(), &stock3b)
	assert.Equal(t, 25.0, stock3b.CurrentQuantity)
	// 25 < 20*1.5 (30), so status should be "low"
	assert.Equal(t, "low", stock3b.Status)

	// Sell 5 more: 25 - 5 = 20, exactly at reorder level
	saleBody2c := models.RecordMovementDto{
		ProductID:    created.ProductID,
		MovementType: "SALE",
		Quantity:     -5.0,
	}
	helper.DoRequest("POST", "/inventory/movements", saleBody2c, map[string]string{})

	w3c := helper.DoRequest("GET", fmt.Sprintf("/inventory/stock/%s", created.ProductID), nil, map[string]string{})
	var stock3c models.ProductStock
	json.Unmarshal(w3c.Body.Bytes(), &stock3c)
	assert.Equal(t, 20.0, stock3c.CurrentQuantity)
	// 20 is NOT < 20, so status should be "low" (because 20 < 20*1.5 = 30)
	assert.Equal(t, "low", stock3c.Status)

	// Sell 5 more: 20 - 5 = 15 < 20 → "critical"
	saleBody3 := models.RecordMovementDto{
		ProductID:    created.ProductID,
		MovementType: "SALE",
		Quantity:     -5.0,
	}
	helper.DoRequest("POST", "/inventory/movements", saleBody3, map[string]string{})

	w4 := helper.DoRequest("GET", fmt.Sprintf("/inventory/stock/%s", created.ProductID), nil, map[string]string{})
	var stock4 models.ProductStock
	json.Unmarshal(w4.Body.Bytes(), &stock4)
	assert.Equal(t, 15.0, stock4.CurrentQuantity)
	assert.Equal(t, "critical", stock4.Status)

	// Sell all: 15 - 15 = 0 → "out_of_stock"
	saleBody4 := models.RecordMovementDto{
		ProductID:    created.ProductID,
		MovementType: "SALE",
		Quantity:     -15.0,
	}
	helper.DoRequest("POST", "/inventory/movements", saleBody4, map[string]string{})

	w5 := helper.DoRequest("GET", fmt.Sprintf("/inventory/stock/%s", created.ProductID), nil, map[string]string{})
	var stock5 models.ProductStock
	json.Unmarshal(w5.Body.Bytes(), &stock5)
	assert.Equal(t, 0.0, stock5.CurrentQuantity)
	assert.Equal(t, "out_of_stock", stock5.Status)
}

// ============================================
// Data Consistency Tests
// ============================================

func TestDataConsistency_ReserveAndRelease(t *testing.T) {
	helper := testhelpers.SetupApiTest(t)
	defer helper.Close()

	// Create product with 100 units
	productBody := models.CreateProductDto{
		SKU:       "CONSISTENCY001",
		Name:      "Consistency Test",
		BasePrice: 50.00,
	}
	w := helper.DoRequest("POST", "/inventory/products", productBody, map[string]string{})
	var created models.CreateProductResponse
	json.Unmarshal(w.Body.Bytes(), &created)

	moveBody := models.RecordMovementDto{
		ProductID:    created.ProductID,
		MovementType: "PURCHASE",
		Quantity:     100.0,
	}
	helper.DoRequest("POST", "/inventory/movements", moveBody, map[string]string{})

	// Reserve 30 units
	reserveBody := map[string]interface{}{
		"product_id": created.ProductID,
		"quantity":   30.0,
	}
	helper.DoRequest("POST", "/inventory/reserve", reserveBody, map[string]string{})

	// Release 30 units
	releaseBody := map[string]interface{}{
		"product_id": created.ProductID,
		"quantity":   30.0,
	}
	helper.DoRequest("POST", "/inventory/release-reserved", releaseBody, map[string]string{})

	// Verify final state: should be exactly as initial
	w1 := helper.DoRequest("GET", fmt.Sprintf("/inventory/stock/%s", created.ProductID), nil, map[string]string{})
	var finalStock models.ProductStock
	json.Unmarshal(w1.Body.Bytes(), &finalStock)

	assert.Equal(t, 100.0, finalStock.CurrentQuantity, "Current quantity should be 100")
	assert.Equal(t, 0.0, finalStock.ReservedQuantity, "Reserved quantity should be 0")
	assert.Equal(t, 100.0, finalStock.AvailableQuantity, "Available quantity should be 100")
	assert.Equal(t, "ok", finalStock.Status, "Status should be ok")
}

func TestDataConsistency_MultipleReservations(t *testing.T) {
	helper := testhelpers.SetupApiTest(t)
	defer helper.Close()

	// Create product with 100 units
	productBody := models.CreateProductDto{
		SKU:       "CONSISTENCY002",
		Name:      "Multiple Reservations",
		BasePrice: 50.00,
	}
	w := helper.DoRequest("POST", "/inventory/products", productBody, map[string]string{})
	var created models.CreateProductResponse
	json.Unmarshal(w.Body.Bytes(), &created)

	moveBody := models.RecordMovementDto{
		ProductID:    created.ProductID,
		MovementType: "PURCHASE",
		Quantity:     100.0,
	}
	helper.DoRequest("POST", "/inventory/movements", moveBody, map[string]string{})

	// Reserve in steps: 20 + 30 + 25 = 75
	reservations := []float64{20.0, 30.0, 25.0}
	for _, qty := range reservations {
		reserveBody := map[string]interface{}{
			"product_id": created.ProductID,
			"quantity":   qty,
		}
		helper.DoRequest("POST", "/inventory/reserve", reserveBody, map[string]string{})
	}

	// Verify: reserved = 75, available = 25
	w1 := helper.DoRequest("GET", fmt.Sprintf("/inventory/stock/%s", created.ProductID), nil, map[string]string{})
	var stock models.ProductStock
	json.Unmarshal(w1.Body.Bytes(), &stock)

	assert.Equal(t, 100.0, stock.CurrentQuantity)
	assert.Equal(t, 75.0, stock.ReservedQuantity, "Total reserved should be 75")
	assert.Equal(t, 25.0, stock.AvailableQuantity, "Available should be 25")

	// Release partial: release 30
	releaseBody := map[string]interface{}{
		"product_id": created.ProductID,
		"quantity":   30.0,
	}
	helper.DoRequest("POST", "/inventory/release-reserved", releaseBody, map[string]string{})

	// Verify: reserved = 45, available = 55
	w2 := helper.DoRequest("GET", fmt.Sprintf("/inventory/stock/%s", created.ProductID), nil, map[string]string{})
	var stock2 models.ProductStock
	json.Unmarshal(w2.Body.Bytes(), &stock2)

	assert.Equal(t, 100.0, stock2.CurrentQuantity)
	assert.Equal(t, 45.0, stock2.ReservedQuantity, "Reserved after release should be 45")
	assert.Equal(t, 55.0, stock2.AvailableQuantity, "Available after release should be 55")
}

// ============================================
// Helper Functions
// ============================================

func ptrFloat64(v float64) *float64 {
	return &v
}

func ptrString(v string) *string {
	return &v
}
