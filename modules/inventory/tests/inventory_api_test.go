//go:build integration
// +build integration

package inventory_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

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
// Helper Functions
// ============================================

func ptrFloat64(v float64) *float64 {
	return &v
}

func ptrString(v string) *string {
	return &v
}
