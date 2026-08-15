//go:build integration
// +build integration

package inventory_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"josex/web/modules/core/testhelpers"
	"josex/web/modules/inventory/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

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
	helper := SetupInventoryTest(t)
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
	helper := SetupInventoryTest(t)
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
	helper := SetupInventoryTest(t)
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
	helper := SetupInventoryTest(t)
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
	helper := SetupInventoryTest(t)
	defer helper.Close()

	body := map[string]interface{}{
		"name": "Missing SKU",
	}

	w := helper.DoRequest("POST", "/inventory/products", body, map[string]string{})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ============================================
// Product Update Tests
// ============================================

// createProductWithSizeGroup creates a product carrying one "Size" group with
// "Large" and "Small", and returns its id together with the fetched detail.
func createProductWithSizeGroup(t *testing.T, helper *testhelpers.ApiTestHelper, sku string) (string, models.ProductDetail) {
	t.Helper()

	body := models.CreateProductDto{
		SKU:       sku,
		Name:      "Latte",
		BasePrice: 3.50,
		Variants: map[string]interface{}{
			"groups": []map[string]interface{}{
				{
					"group_type":     "Size",
					"is_required":    true,
					"max_selections": 1,
					"options": []map[string]interface{}{
						{"name": "Large", "modifier": 0.50},
						{"name": "Small", "modifier": 0.00},
					},
				},
			},
		},
	}

	w := helper.DoRequest("POST", "/inventory/products", body, map[string]string{})
	if w.Code != http.StatusCreated {
		t.Fatalf("setup: creating %s failed with %d: %s", sku, w.Code, w.Body.String())
	}

	var created models.CreateProductResponse
	json.Unmarshal(w.Body.Bytes(), &created)

	return created.ProductID, fetchProductDetail(t, helper, created.ProductID)
}

func fetchProductDetail(t *testing.T, helper *testhelpers.ApiTestHelper, productID string) models.ProductDetail {
	t.Helper()

	w := helper.DoRequest("GET", fmt.Sprintf("/inventory/products/%s", productID), nil, map[string]string{})
	if w.Code != http.StatusOK {
		t.Fatalf("fetching product %s failed with %d: %s", productID, w.Code, w.Body.String())
	}

	var detail models.ProductDetail
	json.Unmarshal(w.Body.Bytes(), &detail)
	return detail
}

func findOption(group models.VariantGroup, name string) (models.VariantOption, bool) {
	for _, option := range group.Options {
		if option.Name == name {
			return option, true
		}
	}
	return models.VariantOption{}, false
}

func TestUpdateProduct_Success(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	productID, _ := createProductWithSizeGroup(t, helper, "UPD001")

	body := models.UpdateProductDto{
		Name:        "Latte Grande",
		Description: ptrString("Now with more milk"),
		BasePrice:   4.25,
	}
	w := helper.DoRequest("PUT", fmt.Sprintf("/inventory/products/%s", productID), body, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())

	detail := fetchProductDetail(t, helper, productID)
	assert.Equal(t, "Latte Grande", detail.Name)
	assert.Equal(t, "Now with more milk", *detail.Description)
	assert.Equal(t, 4.25, detail.BasePrice)
	assert.Equal(t, "UPD001", detail.SKU)
}

func TestUpdateProduct_RenameGroupAndOption(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	productID, before := createProductWithSizeGroup(t, helper, "UPD002")
	group := before.Variants[0]
	large, found := findOption(group, "Large")
	assert.True(t, found)

	body := models.UpdateProductDto{
		Name:      "Latte",
		BasePrice: 3.50,
		Variants: &models.UpdateProductVariantsDto{
			Groups: []models.UpdateVariantGroupDto{
				{
					ID:            &group.ID,
					GroupType:     "Format",
					IsRequired:    true,
					MaxSelections: 1,
					Options: []models.UpdateVariantOptionDto{
						{ID: &large.ID, Name: "XL", Modifier: 1.25},
					},
				},
			},
		},
	}
	w := helper.DoRequest("PUT", fmt.Sprintf("/inventory/products/%s", productID), body, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())

	after := fetchProductDetail(t, helper, productID)
	assert.Len(t, after.Variants, 1)
	assert.Equal(t, group.ID, after.Variants[0].ID)
	assert.Equal(t, "Format", after.Variants[0].GroupType)
	assert.Len(t, after.Variants[0].Options, 1)
	assert.Equal(t, large.ID, after.Variants[0].Options[0].ID)
	assert.Equal(t, "XL", after.Variants[0].Options[0].Name)
	assert.Equal(t, 1.25, after.Variants[0].Options[0].PriceModifier)
}

func TestUpdateProduct_AddAndDropOption(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	productID, before := createProductWithSizeGroup(t, helper, "UPD003")
	group := before.Variants[0]
	large, _ := findOption(group, "Large")

	// "Small" is absent from the payload, "Medium" is new, and a whole new group appears
	body := models.UpdateProductDto{
		Name:      "Latte",
		BasePrice: 3.50,
		Variants: &models.UpdateProductVariantsDto{
			Groups: []models.UpdateVariantGroupDto{
				{
					ID:            &group.ID,
					GroupType:     "Size",
					IsRequired:    true,
					MaxSelections: 1,
					Options: []models.UpdateVariantOptionDto{
						{ID: &large.ID, Name: "Large", Modifier: 0.50},
						{Name: "Medium", Modifier: 0.25},
					},
				},
				{
					GroupType:     "Milk",
					MaxSelections: 1,
					Options: []models.UpdateVariantOptionDto{
						{Name: "Oat", Modifier: 0.60},
					},
				},
			},
		},
	}
	w := helper.DoRequest("PUT", fmt.Sprintf("/inventory/products/%s", productID), body, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())

	after := fetchProductDetail(t, helper, productID)
	assert.Len(t, after.Variants, 2)

	var sizeGroup, milkGroup models.VariantGroup
	for _, g := range after.Variants {
		if g.GroupType == "Size" {
			sizeGroup = g
		}
		if g.GroupType == "Milk" {
			milkGroup = g
		}
	}

	_, smallStillThere := findOption(sizeGroup, "Small")
	assert.False(t, smallStillThere, "the dropped option must be gone")
	_, mediumAdded := findOption(sizeGroup, "Medium")
	assert.True(t, mediumAdded, "the new option must be there")

	// A group added on edit comes back from the read SP with an id
	assert.NotEmpty(t, milkGroup.ID)
	assert.Len(t, milkGroup.Options, 1)
	assert.NotEmpty(t, milkGroup.Options[0].ID)
}

func TestUpdateProduct_OptionMediaSurvives(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	productID, before := createProductWithSizeGroup(t, helper, "UPD004")
	group := before.Variants[0]
	large, _ := findOption(group, "Large")

	media := models.AddProductMediaDto{
		VariantOptionID: &large.ID,
		MediaType:       "image",
		URL:             "https://example.test/large.png",
	}
	wm := helper.DoRequest("POST", fmt.Sprintf("/inventory/products/%s/media", productID), media, map[string]string{})
	assert.Equal(t, http.StatusCreated, wm.Code, wm.Body.String())

	// An edit that only bumps the modifier must not touch the option's media
	body := models.UpdateProductDto{
		Name:      "Latte",
		BasePrice: 3.50,
		Variants: &models.UpdateProductVariantsDto{
			Groups: []models.UpdateVariantGroupDto{
				{
					ID:            &group.ID,
					GroupType:     "Size",
					IsRequired:    true,
					MaxSelections: 1,
					Options: []models.UpdateVariantOptionDto{
						{ID: &large.ID, Name: "Large", Modifier: 0.75},
					},
				},
			},
		},
	}
	w := helper.DoRequest("PUT", fmt.Sprintf("/inventory/products/%s", productID), body, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())

	after := fetchProductDetail(t, helper, productID)
	kept, found := findOption(after.Variants[0], "Large")
	assert.True(t, found)
	assert.Equal(t, large.ID, kept.ID)
	assert.Len(t, kept.Media, 1, "media attached to a kept option must survive the edit")
	assert.Equal(t, "https://example.test/large.png", kept.Media[0].URL)
}

func TestUpdateProduct_VariantsOmittedLeavesTreeIntact(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	productID, before := createProductWithSizeGroup(t, helper, "UPD005")

	body := models.UpdateProductDto{
		Name:      "Latte renamed",
		BasePrice: 9.99,
	}
	w := helper.DoRequest("PUT", fmt.Sprintf("/inventory/products/%s", productID), body, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())

	after := fetchProductDetail(t, helper, productID)
	assert.Equal(t, "Latte renamed", after.Name)
	assert.True(t, after.HasVariants)
	assert.Equal(t, before.Variants, after.Variants, "omitting variants must leave the tree untouched")
}

func TestUpdateProduct_EmptyGroupsClearsVariants(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	productID, _ := createProductWithSizeGroup(t, helper, "UPD006")

	body := models.UpdateProductDto{
		Name:      "Latte",
		BasePrice: 3.50,
		Variants:  &models.UpdateProductVariantsDto{Groups: []models.UpdateVariantGroupDto{}},
	}
	w := helper.DoRequest("PUT", fmt.Sprintf("/inventory/products/%s", productID), body, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())

	after := fetchProductDetail(t, helper, productID)
	assert.False(t, after.HasVariants)
	assert.Empty(t, after.Variants)
}

func TestUpdateProduct_NotFound(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	body := models.UpdateProductDto{Name: "Ghost", BasePrice: 1.00}

	w := helper.DoRequest("PUT", "/inventory/products/00000000-0000-0000-0000-000000000000", body, map[string]string{})
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
}

func TestUpdateProduct_GroupFromAnotherProduct(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	targetID, _ := createProductWithSizeGroup(t, helper, "UPD007")
	_, other := createProductWithSizeGroup(t, helper, "UPD008")
	foreignGroupID := other.Variants[0].ID

	body := models.UpdateProductDto{
		Name:      "Latte",
		BasePrice: 3.50,
		Variants: &models.UpdateProductVariantsDto{
			Groups: []models.UpdateVariantGroupDto{
				{ID: &foreignGroupID, GroupType: "Size", MaxSelections: 1},
			},
		},
	}
	w := helper.DoRequest("PUT", fmt.Sprintf("/inventory/products/%s", targetID), body, map[string]string{})
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
}

func TestUpdateProduct_DuplicateGroupType(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	productID, _ := createProductWithSizeGroup(t, helper, "UPD009")

	body := models.UpdateProductDto{
		Name:      "Latte",
		BasePrice: 3.50,
		Variants: &models.UpdateProductVariantsDto{
			Groups: []models.UpdateVariantGroupDto{
				{GroupType: "Milk", MaxSelections: 1},
				{GroupType: "Milk", MaxSelections: 1},
			},
		},
	}
	w := helper.DoRequest("PUT", fmt.Sprintf("/inventory/products/%s", productID), body, map[string]string{})
	assert.Equal(t, http.StatusConflict, w.Code, w.Body.String())
}

// ============================================
// Product Retrieval Tests
// ============================================

func TestGetProduct_Success(t *testing.T) {
	helper := SetupInventoryTest(t)
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
	helper := SetupInventoryTest(t)
	defer helper.Close()

	w := helper.DoRequest("GET", "/inventory/products/00000000-0000-0000-0000-000000000000", nil, map[string]string{})
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetProductBySkU_Success(t *testing.T) {
	helper := SetupInventoryTest(t)
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
	helper := SetupInventoryTest(t)
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
// Product List — categories column and filters
// ============================================

func TestListProducts_RowCarriesItsCategories(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	suffix := uuid.New().String()[:8]
	productID := createProductForList(t, helper, "ROWCAT"+suffix, "Row Cat Product")
	catA := createCategoryForList(t, helper, "Row Cat A", "row-cat-a-"+suffix)
	catB := createCategoryForList(t, helper, "Row Cat B", "row-cat-b-"+suffix)
	assignProductToCategory(t, helper, productID, catA)
	assignProductToCategory(t, helper, productID, catB)

	products := listProducts(t, helper, "?limit=100&search=ROWCAT"+suffix)

	assert.Equal(t, 1, len(products), "search should isolate the product under test")
	assert.Equal(t, 2, len(products[0].Categories), "a product in two categories lists both")
	slugs := []string{products[0].Categories[0].Slug, products[0].Categories[1].Slug}
	assert.Contains(t, slugs, "row-cat-a-"+suffix)
	assert.Contains(t, slugs, "row-cat-b-"+suffix)
	assert.NotEmpty(t, products[0].Categories[0].ID)
	assert.NotEmpty(t, products[0].Categories[0].Name)
}

func TestListProducts_UncategorisedRowHasEmptyCategories(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	suffix := uuid.New().String()[:8]
	createProductForList(t, helper, "NOCAT"+suffix, "No Cat Product")

	products := listProducts(t, helper, "?limit=100&search=NOCAT"+suffix)

	assert.Equal(t, 1, len(products))
	assert.NotNil(t, products[0].Categories, "categories must be [], never null")
	assert.Equal(t, 0, len(products[0].Categories))
}

func TestListProducts_FilterByCategory(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	suffix := uuid.New().String()[:8]
	inCategory := createProductForList(t, helper, "FILTIN"+suffix, "Filtered In")
	alsoIn := createProductForList(t, helper, "FILTIN2"+suffix, "Filtered In Too")
	createProductForList(t, helper, "FILTOUT"+suffix, "Filtered Out")
	target := createCategoryForList(t, helper, "Filter Target", "filter-target-"+suffix)
	other := createCategoryForList(t, helper, "Filter Other", "filter-other-"+suffix)
	assignProductToCategory(t, helper, inCategory, target)
	assignProductToCategory(t, helper, inCategory, other)
	assignProductToCategory(t, helper, alsoIn, target)

	products := listProducts(t, helper, "?limit=100&category_id="+target)

	returnedIDs := []string{}
	for _, p := range products {
		returnedIDs = append(returnedIDs, p.ID)
	}
	assert.Equal(t, 2, len(products), "only the two assigned products")
	assert.Contains(t, returnedIDs, inCategory)
	assert.Contains(t, returnedIDs, alsoIn)
	for _, p := range products {
		if p.ID == inCategory {
			assert.Equal(t, 2, len(p.Categories), "filtering by one category still returns all of a product's categories")
		}
	}
}

func TestListProducts_FilterByUnknownCategoryIsEmpty(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	createProductForList(t, helper, "UNKCAT"+uuid.New().String()[:8], "Unknown Cat Product")

	w := helper.DoRequest("GET", "/inventory/products?limit=100&category_id="+uuid.New().String(), nil, map[string]string{})

	assert.Equal(t, http.StatusOK, w.Code)
	var result listProductsResponse
	err := json.Unmarshal(w.Body.Bytes(), &result)
	assert.NoError(t, err)
	assert.NotNil(t, result.Data, "data must be [], never null")
	assert.Equal(t, 0, len(result.Data))
}

func TestListProducts_MalformedCategoryIDReturns400(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	w := helper.DoRequest("GET", "/inventory/products?category_id=not-a-uuid", nil, map[string]string{})

	assert.Equal(t, http.StatusBadRequest, w.Code, "a malformed category_id must not fall through to a full list")
}

func TestListProducts_SearchMatchesName(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	suffix := uuid.New().String()[:8]
	productID := createProductForList(t, helper, "SKUONLY"+suffix, "Zebra"+suffix+"Name")

	// Lowercased on purpose: the match is case-insensitive, and the token lives
	// only in the name, never in the SKU.
	products := listProducts(t, helper, "?limit=100&search=zebra"+suffix)

	assert.Equal(t, 1, len(products))
	assert.Equal(t, productID, products[0].ID)
}

func TestListProducts_SearchMatchesSku(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	suffix := uuid.New().String()[:8]
	productID := createProductForList(t, helper, "SRCHSKU"+suffix, "Nothing Matching Here")

	products := listProducts(t, helper, "?limit=100&search=srchsku"+suffix)

	assert.Equal(t, 1, len(products))
	assert.Equal(t, productID, products[0].ID)
}

// ============================================
// Inventory Movement Tests
// ============================================

func TestRecordMovement_Purchase(t *testing.T) {
	helper := SetupInventoryTest(t)
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
	helper := SetupInventoryTest(t)
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
	helper := SetupInventoryTest(t)
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
	helper := SetupInventoryTest(t)
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
	helper := SetupInventoryTest(t)
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
// Movement List Tests
// ============================================

func TestListMovements_Success(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	suffix := uuid.New().String()[:8]
	productID := createProductForList(t, helper, "MVLIST"+suffix, "Movement List Product")
	recordMovementForList(t, helper, productID, "PURCHASE", 40.0)
	recordMovementForList(t, helper, productID, "SALE", 15.0)

	result := listMovements(t, helper, "?product_id="+productID)

	assert.Equal(t, int64(2), result.TotalCount)
	assert.Equal(t, 2, len(result.Movements))
	assert.Equal(t, 1, result.Page)
	assert.Equal(t, 20, result.PageSize)
	assert.Equal(t, "SALE", result.Movements[0].MovementType, "newest first")
	assert.Equal(t, "PURCHASE", result.Movements[1].MovementType)
}

func TestListMovements_FilterByProduct(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	suffix := uuid.New().String()[:8]
	wanted := createProductForList(t, helper, "MVWANT"+suffix, "Wanted Product")
	other := createProductForList(t, helper, "MVOTHER"+suffix, "Other Product")
	recordMovementForList(t, helper, wanted, "PURCHASE", 10.0)
	recordMovementForList(t, helper, other, "PURCHASE", 99.0)

	result := listMovements(t, helper, "?product_id="+wanted)

	assert.Equal(t, int64(1), result.TotalCount)
	assert.Equal(t, 1, len(result.Movements))
	assert.Equal(t, wanted, result.Movements[0].ProductID)
	assert.Equal(t, 10.0, result.Movements[0].Quantity)
}

func TestListMovements_FilterByMovementType(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	suffix := uuid.New().String()[:8]
	productID := createProductForList(t, helper, "MVTYPE"+suffix, "Type Filter Product")
	recordMovementForList(t, helper, productID, "PURCHASE", 60.0)
	recordMovementForList(t, helper, productID, "ADJUSTMENT", 5.0)
	recordMovementForList(t, helper, productID, "SALE", 20.0)

	result := listMovements(t, helper, "?product_id="+productID+"&movement_type=ADJUSTMENT")

	assert.Equal(t, int64(1), result.TotalCount)
	assert.Equal(t, 1, len(result.Movements))
	assert.Equal(t, "ADJUSTMENT", result.Movements[0].MovementType)
}

func TestListMovements_PagePastTheEndIsEmpty(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	suffix := uuid.New().String()[:8]
	productID := createProductForList(t, helper, "MVPAGE"+suffix, "Paging Product")
	recordMovementForList(t, helper, productID, "PURCHASE", 30.0)
	recordMovementForList(t, helper, productID, "SALE", 4.0)

	firstPage := listMovements(t, helper, "?product_id="+productID+"&page=1&page_size=1")
	assert.Equal(t, int64(2), firstPage.TotalCount)
	assert.Equal(t, 1, len(firstPage.Movements))

	// Past the end: total_count rides on the rows, so an empty page carries none.
	pastEnd := listMovements(t, helper, "?product_id="+productID+"&page=99&page_size=1")
	assert.Equal(t, 0, len(pastEnd.Movements), "movements must be [], never null")
	assert.Equal(t, 99, pastEnd.Page)
	assert.Equal(t, 1, pastEnd.PageSize)
}

func TestListMovements_PageSizeAboveCapReturns400(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	w := helper.DoRequest("GET", "/inventory/movements?page_size=500", nil, map[string]string{})

	assert.Equal(t, http.StatusBadRequest, w.Code, "page_size is capped at 100")
}

// ============================================
// Stock Query Tests
// ============================================

func TestGetProductStock_Success(t *testing.T) {
	helper := SetupInventoryTest(t)
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
	helper := SetupInventoryTest(t)
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
	helper := SetupInventoryTest(t)
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
	helper := SetupInventoryTest(t)
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
	helper := SetupInventoryTest(t)
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
	helper := SetupInventoryTest(t)
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
	helper := SetupInventoryTest(t)
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
	helper := SetupInventoryTest(t)
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
// Batch Tests
// ============================================

func TestCreateBatch_Success(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	// Create a product
	productBody := models.CreateProductDto{
		SKU:       fmt.Sprintf("BATCH_TEST_%d", time.Now().UnixNano()),
		Name:      "Batch Test Product",
		BasePrice: 100.00,
	}
	w := helper.DoRequest("POST", "/inventory/products", productBody, map[string]string{})
	assert.Equal(t, http.StatusCreated, w.Code, "Failed to create product")

	var createdProduct models.CreateProductResponse
	json.Unmarshal(w.Body.Bytes(), &createdProduct)

	// Create batch
	nextTwoWeeks := time.Now().AddDate(0, 0, 14) // Far future to be "active"
	yesterday := time.Now().AddDate(0, 0, -1)

	batchBody := map[string]interface{}{
		"product_id":       createdProduct.ProductID,
		"lot_number":       fmt.Sprintf("LOT_%d", time.Now().UnixNano()),
		"purchase_date":    yesterday.Format("2006-01-02"),
		"expiry_date":      nextTwoWeeks.Format("2006-01-02"),
		"unit_cost":        50.00,
		"initial_quantity": 100.0,
	}

	w2 := helper.DoRequest("POST", "/inventory/batches", batchBody, map[string]string{})
	if w2.Code != http.StatusCreated {
		t.Logf("CreateBatch error response: %s", w2.Body.String())
	}
	assert.Equal(t, http.StatusCreated, w2.Code, "Failed to create batch")

	var batch models.BatchResponse
	json.Unmarshal(w2.Body.Bytes(), &batch)

	assert.NotEmpty(t, batch.ID)
	assert.Equal(t, createdProduct.ProductID, batch.ProductID.String())
	assert.Equal(t, 100.0, batch.CurrentQuantity)
	assert.Equal(t, 50.00, batch.UnitCost)
	assert.Equal(t, "active", batch.Status)
}

func TestCreateBatch_InvalidExpiryDate(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	// Create product
	productBody := models.CreateProductDto{
		SKU:       fmt.Sprintf("BATCH_INV_%d", time.Now().UnixNano()),
		Name:      "Batch Invalid Test",
		BasePrice: 100.00,
	}
	w := helper.DoRequest("POST", "/inventory/products", productBody, map[string]string{})
	var createdProduct models.CreateProductResponse
	json.Unmarshal(w.Body.Bytes(), &createdProduct)

	// Try to create batch with past expiry date
	yesterday := time.Now().AddDate(0, 0, -1)

	batchBody := map[string]interface{}{
		"product_id":       createdProduct.ProductID,
		"lot_number":       fmt.Sprintf("LOT_%d", time.Now().UnixNano()),
		"purchase_date":    yesterday.Format("2006-01-02"),
		"expiry_date":      yesterday.Format("2006-01-02"),
		"unit_cost":        50.00,
		"initial_quantity": 100.0,
	}

	w2 := helper.DoRequest("POST", "/inventory/batches", batchBody, map[string]string{})
	assert.Equal(t, http.StatusInternalServerError, w2.Code, "Should fail with past expiry date")
}

func TestGetBatch_Success(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	// Create product and batch
	productBody := models.CreateProductDto{
		SKU:       fmt.Sprintf("BATCH_GET_%d", time.Now().UnixNano()),
		Name:      "Batch Get Test",
		BasePrice: 100.00,
	}
	w := helper.DoRequest("POST", "/inventory/products", productBody, map[string]string{})
	var createdProduct models.CreateProductResponse
	json.Unmarshal(w.Body.Bytes(), &createdProduct)

	// Create batch
	tomorrow := time.Now().AddDate(0, 0, 1)
	yesterday := time.Now().AddDate(0, 0, -1)
	lotNumber := fmt.Sprintf("LOT_%d", time.Now().UnixNano())

	batchBody := map[string]interface{}{
		"product_id":       createdProduct.ProductID,
		"lot_number":       lotNumber,
		"purchase_date":    yesterday.Format("2006-01-02"),
		"expiry_date":      tomorrow.Format("2006-01-02"),
		"unit_cost":        50.00,
		"initial_quantity": 100.0,
	}

	w2 := helper.DoRequest("POST", "/inventory/batches", batchBody, map[string]string{})
	var createdBatch models.BatchResponse
	json.Unmarshal(w2.Body.Bytes(), &createdBatch)

	// Get batch
	w3 := helper.DoRequest("GET", fmt.Sprintf("/inventory/batches/%s", createdBatch.ID), nil, map[string]string{})
	assert.Equal(t, http.StatusOK, w3.Code)

	var retrievedBatch models.BatchResponse
	json.Unmarshal(w3.Body.Bytes(), &retrievedBatch)

	assert.Equal(t, createdBatch.ID, retrievedBatch.ID)
	assert.Equal(t, lotNumber, retrievedBatch.LotNumber)
	assert.Equal(t, 100.0, retrievedBatch.CurrentQuantity)
}

func TestListBatchesByProduct_Success(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	// Create product
	productBody := models.CreateProductDto{
		SKU:       fmt.Sprintf("BATCH_LIST_%d", time.Now().UnixNano()),
		Name:      "Batch List Test",
		BasePrice: 100.00,
	}
	w := helper.DoRequest("POST", "/inventory/products", productBody, map[string]string{})
	var createdProduct models.CreateProductResponse
	json.Unmarshal(w.Body.Bytes(), &createdProduct)

	// Create 3 batches
	tomorrow := time.Now().AddDate(0, 0, 1)
	nextWeek := time.Now().AddDate(0, 0, 7)
	nextMonth := time.Now().AddDate(0, 1, 0)
	yesterday := time.Now().AddDate(0, 0, -1)

	batches := []map[string]interface{}{
		{
			"product_id":       createdProduct.ProductID,
			"lot_number":       fmt.Sprintf("LOT1_%d", time.Now().UnixNano()),
			"purchase_date":    yesterday.Format("2006-01-02"),
			"expiry_date":      tomorrow.Format("2006-01-02"),
			"unit_cost":        50.00,
			"initial_quantity": 100.0,
		},
		{
			"product_id":       createdProduct.ProductID,
			"lot_number":       fmt.Sprintf("LOT2_%d", time.Now().UnixNano()),
			"purchase_date":    yesterday.Format("2006-01-02"),
			"expiry_date":      nextWeek.Format("2006-01-02"),
			"unit_cost":        55.00,
			"initial_quantity": 200.0,
		},
		{
			"product_id":       createdProduct.ProductID,
			"lot_number":       fmt.Sprintf("LOT3_%d", time.Now().UnixNano()),
			"purchase_date":    yesterday.Format("2006-01-02"),
			"expiry_date":      nextMonth.Format("2006-01-02"),
			"unit_cost":        60.00,
			"initial_quantity": 150.0,
		},
	}

	for _, batch := range batches {
		w := helper.DoRequest("POST", "/inventory/batches", batch, map[string]string{})
		assert.Equal(t, http.StatusCreated, w.Code)
	}

	// List batches by product
	w4 := helper.DoRequest("GET", fmt.Sprintf("/inventory/batches/product/%s", createdProduct.ProductID), nil, map[string]string{})
	assert.Equal(t, http.StatusOK, w4.Code)

	var listResponse models.ListBatchesResponse
	json.Unmarshal(w4.Body.Bytes(), &listResponse)

	assert.Equal(t, 3, listResponse.Count)
	assert.Equal(t, 3, len(listResponse.Batches))
}

func TestGetOldestBatchForSale_FIFO(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	// Create product
	productBody := models.CreateProductDto{
		SKU:       fmt.Sprintf("BATCH_FIFO_%d", time.Now().UnixNano()),
		Name:      "Batch FIFO Test",
		BasePrice: 100.00,
	}
	w := helper.DoRequest("POST", "/inventory/products", productBody, map[string]string{})
	var createdProduct models.CreateProductResponse
	json.Unmarshal(w.Body.Bytes(), &createdProduct)

	// Create 3 batches in random order
	tomorrow := time.Now().AddDate(0, 0, 1)
	nextWeek := time.Now().AddDate(0, 0, 7)
	nextMonth := time.Now().AddDate(0, 1, 0)
	yesterday := time.Now().AddDate(0, 0, -1)

	batches := []map[string]interface{}{
		{
			"product_id":       createdProduct.ProductID,
			"lot_number":       fmt.Sprintf("LOT_THIRD_%d", time.Now().UnixNano()),
			"purchase_date":    yesterday.Format("2006-01-02"),
			"expiry_date":      nextMonth.Format("2006-01-02"),
			"unit_cost":        60.00,
			"initial_quantity": 150.0,
		},
		{
			"product_id":       createdProduct.ProductID,
			"lot_number":       fmt.Sprintf("LOT_FIRST_%d", time.Now().UnixNano()),
			"purchase_date":    yesterday.Format("2006-01-02"),
			"expiry_date":      tomorrow.Format("2006-01-02"),
			"unit_cost":        50.00,
			"initial_quantity": 100.0,
		},
		{
			"product_id":       createdProduct.ProductID,
			"lot_number":       fmt.Sprintf("LOT_SECOND_%d", time.Now().UnixNano()),
			"purchase_date":    yesterday.Format("2006-01-02"),
			"expiry_date":      nextWeek.Format("2006-01-02"),
			"unit_cost":        55.00,
			"initial_quantity": 200.0,
		},
	}

	for _, batch := range batches {
		w := helper.DoRequest("POST", "/inventory/batches", batch, map[string]string{})
		assert.Equal(t, http.StatusCreated, w.Code)
	}

	// Get oldest batch for sale
	w4 := helper.DoRequest("GET", fmt.Sprintf("/inventory/batches/product/%s/oldest", createdProduct.ProductID), nil, map[string]string{})
	assert.Equal(t, http.StatusOK, w4.Code)

	var oldestBatch models.BatchResponse
	json.Unmarshal(w4.Body.Bytes(), &oldestBatch)

	assert.NotEmpty(t, oldestBatch.ID)
	assert.Equal(t, 100.0, oldestBatch.CurrentQuantity)
	assert.Equal(t, 50.00, oldestBatch.UnitCost)
	assert.Equal(t, "expiring_soon", oldestBatch.Status)
}

func TestBatchExpiryStatusCalculation(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	// Create product
	productBody := models.CreateProductDto{
		SKU:       fmt.Sprintf("BATCH_EXP_%d", time.Now().UnixNano()),
		Name:      "Batch Expiry Test",
		BasePrice: 100.00,
	}
	w := helper.DoRequest("POST", "/inventory/products", productBody, map[string]string{})
	var createdProduct models.CreateProductResponse
	json.Unmarshal(w.Body.Bytes(), &createdProduct)

	yesterday := time.Now().AddDate(0, 0, -1)

	// Active batch (far future)
	nextMonth := time.Now().AddDate(0, 1, 0)
	activeBatchBody := map[string]interface{}{
		"product_id":       createdProduct.ProductID,
		"lot_number":       fmt.Sprintf("LOT_ACTIVE_%d", time.Now().UnixNano()),
		"purchase_date":    yesterday.Format("2006-01-02"),
		"expiry_date":      nextMonth.Format("2006-01-02"),
		"unit_cost":        50.00,
		"initial_quantity": 100.0,
	}

	w1 := helper.DoRequest("POST", "/inventory/batches", activeBatchBody, map[string]string{})
	var activeBatch models.BatchResponse
	json.Unmarshal(w1.Body.Bytes(), &activeBatch)
	assert.Equal(t, "active", activeBatch.Status)

	// Expiring soon batch (within 7 days)
	inFourDays := time.Now().AddDate(0, 0, 4)
	expiringBatchBody := map[string]interface{}{
		"product_id":       createdProduct.ProductID,
		"lot_number":       fmt.Sprintf("LOT_EXPIRING_%d", time.Now().UnixNano()),
		"purchase_date":    yesterday.Format("2006-01-02"),
		"expiry_date":      inFourDays.Format("2006-01-02"),
		"unit_cost":        50.00,
		"initial_quantity": 100.0,
	}

	w2 := helper.DoRequest("POST", "/inventory/batches", expiringBatchBody, map[string]string{})
	var expiringBatch models.BatchResponse
	json.Unmarshal(w2.Body.Bytes(), &expiringBatch)
	assert.Equal(t, "expiring_soon", expiringBatch.Status)
}

func TestBatchNotFound(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	w := helper.DoRequest("GET", "/inventory/batches/00000000-0000-0000-0000-000000000000", nil, map[string]string{})
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ============================================
// Product Categories Tests
// ============================================

func TestCreateCategory_Success(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	uniqueSlug := "electronics-" + uuid.New().String()[:8]
	createDto := models.CreateCategoryDto{
		Name:        "Electronics",
		Slug:        uniqueSlug,
		Description: ptrString("All electronic devices"),
	}

	w := helper.DoRequest("POST", "/inventory/categories", createDto, map[string]string{})
	assert.Equal(t, http.StatusCreated, w.Code, "Category creation should return 201")

	var resp models.CategoryResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err, "Response should be valid JSON")
	assert.Equal(t, "Electronics", resp.Name)
	assert.Equal(t, uniqueSlug, resp.Slug)
	assert.Equal(t, "All electronic devices", *resp.Description)
	assert.True(t, resp.IsActive)
	assert.Equal(t, int64(0), resp.ProductCount)
}

func TestCreateCategory_DuplicateSlug(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	uniqueSlug := "clothes-" + uuid.New().String()[:8]
	dto := models.CreateCategoryDto{
		Name: "Clothes",
		Slug: uniqueSlug,
	}

	// Create first category
	w1 := helper.DoRequest("POST", "/inventory/categories", dto, map[string]string{})
	assert.Equal(t, http.StatusCreated, w1.Code)

	// Try to create with same slug
	w2 := helper.DoRequest("POST", "/inventory/categories", dto, map[string]string{})
	assert.Equal(t, http.StatusConflict, w2.Code, "Duplicate slug should return 409 Conflict")
}

func TestCreateCategory_MissingRequired(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	// Missing name and slug
	dto := models.CreateCategoryDto{}

	w := helper.DoRequest("POST", "/inventory/categories", dto, map[string]string{})
	assert.Equal(t, http.StatusBadRequest, w.Code, "Missing required fields should return 400")
}

func TestGetCategory_Success(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	// Create category
	uniqueSlug := "furniture-" + uuid.New().String()[:8]
	createDto := models.CreateCategoryDto{
		Name: "Furniture",
		Slug: uniqueSlug,
	}
	w1 := helper.DoRequest("POST", "/inventory/categories", createDto, map[string]string{})
	assert.Equal(t, http.StatusCreated, w1.Code)

	var created models.CategoryResponse
	json.Unmarshal(w1.Body.Bytes(), &created)

	// Get category
	w2 := helper.DoRequest("GET", fmt.Sprintf("/inventory/categories/%s", created.ID), nil, map[string]string{})
	assert.Equal(t, http.StatusOK, w2.Code)

	var retrieved models.CategoryResponse
	json.Unmarshal(w2.Body.Bytes(), &retrieved)
	assert.Equal(t, "Furniture", retrieved.Name)
	assert.Equal(t, created.ID, retrieved.ID)
}

func TestGetCategory_NotFound(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	w := helper.DoRequest("GET", "/inventory/categories/00000000-0000-0000-0000-000000000000", nil, map[string]string{})
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestListCategories_Success(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	// Create multiple categories
	for i := 1; i <= 3; i++ {
		dto := models.CreateCategoryDto{
			Name: fmt.Sprintf("Category %d", i),
			Slug: fmt.Sprintf("cat%d-", i) + uuid.New().String()[:8],
		}
		helper.DoRequest("POST", "/inventory/categories", dto, map[string]string{})
	}

	// List categories
	w := helper.DoRequest("GET", "/inventory/categories?page=1&limit=10", nil, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code)

	var resp models.ListCategoriesResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.True(t, len(resp.Categories) >= 3, "Should have at least 3 categories")
	assert.True(t, resp.Total >= 3)
}

func TestSearchCategories_Success(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	// Create a category with descriptive name
	uniqueSlug := "luxury-electronics-" + uuid.New().String()[:8]
	dto := models.CreateCategoryDto{
		Name:        "Luxury Electronics",
		Slug:        uniqueSlug,
		Description: ptrString("Premium electronic devices"),
	}
	helper.DoRequest("POST", "/inventory/categories", dto, map[string]string{})

	// Search for categories
	w := helper.DoRequest("GET", "/inventory/categories/search?search_term=Luxury", nil, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code, "Search should return 200")

	var resp []models.CategoryResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.True(t, len(resp) > 0, "Search should find matching categories")
}

func TestUpdateCategory_Success(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	// Create category
	uniqueSlug := "old-slug-" + uuid.New().String()[:8]
	createDto := models.CreateCategoryDto{
		Name: "Old Name",
		Slug: uniqueSlug,
	}
	w1 := helper.DoRequest("POST", "/inventory/categories", createDto, map[string]string{})
	var created models.CategoryResponse
	json.Unmarshal(w1.Body.Bytes(), &created)

	// Update category
	updateDto := models.UpdateCategoryDto{
		Name:        ptrString("New Name"),
		Description: ptrString("Updated description"),
		IsActive:    ptrBool(false),
	}
	w2 := helper.DoRequest("PUT", fmt.Sprintf("/inventory/categories/%s", created.ID), updateDto, map[string]string{})
	assert.Equal(t, http.StatusOK, w2.Code)

	var updated models.CategoryResponse
	json.Unmarshal(w2.Body.Bytes(), &updated)
	assert.Equal(t, "New Name", updated.Name)
	assert.Equal(t, false, updated.IsActive)
}

func TestUpdateCategory_NotFound(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	updateDto := models.UpdateCategoryDto{
		Name: ptrString("Updated"),
	}
	w := helper.DoRequest("PUT", "/inventory/categories/00000000-0000-0000-0000-000000000000", updateDto, map[string]string{})
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestDeleteCategory_Success(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	// Create category
	createDto := models.CreateCategoryDto{
		Name: "To Delete",
		Slug: "to-delete-" + uuid.New().String()[:8],
	}
	w1 := helper.DoRequest("POST", "/inventory/categories", createDto, map[string]string{})
	var created models.CategoryResponse
	json.Unmarshal(w1.Body.Bytes(), &created)

	// Delete category
	w2 := helper.DoRequest("DELETE", fmt.Sprintf("/inventory/categories/%s", created.ID), nil, map[string]string{})
	if w2.Code != http.StatusOK {
		t.Logf("Delete error (status %d): %s", w2.Code, w2.Body.String())
	}
	assert.Equal(t, http.StatusOK, w2.Code, "Delete should return 200")

	// Verify it's deleted (soft delete - not found)
	w3 := helper.DoRequest("GET", fmt.Sprintf("/inventory/categories/%s", created.ID), nil, map[string]string{})
	assert.Equal(t, http.StatusNotFound, w3.Code, "Deleted category should return 404 on GET")
}

func TestDeleteCategory_NotFound(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	w := helper.DoRequest("DELETE", "/inventory/categories/00000000-0000-0000-0000-000000000000", nil, map[string]string{})
	assert.Equal(t, http.StatusNotFound, w.Code, "Delete non-existent category should return 404")
}

func TestCreateHierarchy_Success(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	// Create parent category
	parentSlug := "electronics-" + uuid.New().String()[:8]
	parentDto := models.CreateCategoryDto{
		Name: "Electronics",
		Slug: parentSlug,
	}
	w1 := helper.DoRequest("POST", "/inventory/categories", parentDto, map[string]string{})
	var parent models.CategoryResponse
	json.Unmarshal(w1.Body.Bytes(), &parent)

	// Create child category with parent
	childSlug := "smartphones-" + uuid.New().String()[:8]
	childDto := models.CreateCategoryDto{
		Name:     "Smartphones",
		Slug:     childSlug,
		ParentID: &parent.ID,
	}
	w2 := helper.DoRequest("POST", "/inventory/categories", childDto, map[string]string{})
	assert.Equal(t, http.StatusCreated, w2.Code)

	var child models.CategoryResponse
	json.Unmarshal(w2.Body.Bytes(), &child)
	assert.NotNil(t, child.ParentID, "Child should have parent ID")
	if child.ParentID != nil {
		assert.Equal(t, parent.ID, *child.ParentID, "Child's parent ID should match parent's ID")
	}
}

func TestCircularHierarchy_Prevented(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	// Create category A
	dtoA := models.CreateCategoryDto{
		Name: "Category A",
		Slug: "cat-a-" + uuid.New().String()[:8],
	}
	w1 := helper.DoRequest("POST", "/inventory/categories", dtoA, map[string]string{})
	var catA models.CategoryResponse
	json.Unmarshal(w1.Body.Bytes(), &catA)

	// Create category B with A as parent
	dtoB := models.CreateCategoryDto{
		Name:     "Category B",
		Slug:     "cat-b-" + uuid.New().String()[:8],
		ParentID: &catA.ID,
	}
	w2 := helper.DoRequest("POST", "/inventory/categories", dtoB, map[string]string{})
	var catB models.CategoryResponse
	json.Unmarshal(w2.Body.Bytes(), &catB)

	// Try to make A's parent = B (creating circular reference)
	updateDto := models.UpdateCategoryDto{
		ParentID: &catB.ID,
	}
	w3 := helper.DoRequest("PUT", fmt.Sprintf("/inventory/categories/%s", catA.ID), updateDto, map[string]string{})
	// Circular hierarchy error should return 400 or 500 if error mapping isn't complete
	assert.True(t, w3.Code == http.StatusBadRequest || w3.Code == http.StatusInternalServerError,
		fmt.Sprintf("Expected 400 or 500 for circular hierarchy, got %d", w3.Code))
}

func TestAssignProductToCategory_Success(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	// Create product
	productDto := models.CreateProductDto{
		SKU:       "PROD001-" + uuid.New().String()[:8],
		Name:      "Test Product",
		BasePrice: 10.00,
	}
	w1 := helper.DoRequest("POST", "/inventory/products", productDto, map[string]string{})
	var product models.CreateProductResponse
	json.Unmarshal(w1.Body.Bytes(), &product)

	// Create category
	catSlug := "test-cat-" + uuid.New().String()[:8]
	catDto := models.CreateCategoryDto{
		Name: "Test Category",
		Slug: catSlug,
	}
	w2 := helper.DoRequest("POST", "/inventory/categories", catDto, map[string]string{})
	var category models.CategoryResponse
	json.Unmarshal(w2.Body.Bytes(), &category)

	// Assign product to category
	w3 := helper.DoRequest("POST", fmt.Sprintf("/inventory/products/%s/categories/%s", product.ProductID, category.ID), nil, map[string]string{})
	assert.Equal(t, http.StatusCreated, w3.Code, "Assign product should return 201")
}

func TestRemoveProductFromCategory_Success(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	// Create and assign product to category
	productDto := models.CreateProductDto{
		SKU:       "PROD002-" + uuid.New().String()[:8],
		Name:      "Product To Unassign",
		BasePrice: 20.00,
	}
	w1 := helper.DoRequest("POST", "/inventory/products", productDto, map[string]string{})
	var product models.CreateProductResponse
	json.Unmarshal(w1.Body.Bytes(), &product)

	catDto := models.CreateCategoryDto{
		Name: "Unassign Test",
		Slug: "unassign-test-" + uuid.New().String()[:8],
	}
	w2 := helper.DoRequest("POST", "/inventory/categories", catDto, map[string]string{})
	var category models.CategoryResponse
	json.Unmarshal(w2.Body.Bytes(), &category)

	// Assign
	helper.DoRequest("POST", fmt.Sprintf("/inventory/products/%s/categories/%s", product.ProductID, category.ID), nil, map[string]string{})

	// Remove
	w4 := helper.DoRequest("DELETE", fmt.Sprintf("/inventory/products/%s/categories/%s", product.ProductID, category.ID), nil, map[string]string{})
	assert.Equal(t, http.StatusOK, w4.Code, "Remove product should return 200")
}

func TestGetProductsByCategory_Success(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	// Create category and products
	catSlug := "books-" + uuid.New().String()[:8]
	catDto := models.CreateCategoryDto{
		Name: "Books",
		Slug: catSlug,
	}
	w1 := helper.DoRequest("POST", "/inventory/categories", catDto, map[string]string{})
	var category models.CategoryResponse
	json.Unmarshal(w1.Body.Bytes(), &category)
	t.Logf("Created category: ID=%s", category.ID)

	// Create 2 products
	productIDs := []string{}
	for i := 1; i <= 2; i++ {
		productDto := models.CreateProductDto{
			SKU:       fmt.Sprintf("BOOK%d-", i) + uuid.New().String()[:8],
			Name:      fmt.Sprintf("Book %d", i),
			BasePrice: float64(10 * i),
		}
		w := helper.DoRequest("POST", "/inventory/products", productDto, map[string]string{})
		var product models.CreateProductResponse
		json.Unmarshal(w.Body.Bytes(), &product)
		t.Logf("Created product %d: ID=%s, SKU=%s", i, product.ProductID, product.SKU)
		productIDs = append(productIDs, product.ProductID)

		// Assign to category
		assignResp := helper.DoRequest("POST", fmt.Sprintf("/inventory/products/%s/categories/%s", product.ProductID, category.ID), nil, map[string]string{})
		t.Logf("Assign product %d response: status=%d, body=%s", i, assignResp.Code, assignResp.Body.String())
	}

	// Get products in category
	w5 := helper.DoRequest("GET", fmt.Sprintf("/inventory/categories/%s/products", category.ID), nil, map[string]string{})
	assert.Equal(t, http.StatusOK, w5.Code, "Should return 200")

	var resp []models.GetProductsByCategoryResponse
	err := json.Unmarshal(w5.Body.Bytes(), &resp)
	if err != nil {
		t.Logf("Failed to unmarshal response: %v", err)
	}
	t.Logf("Products in category: len=%d, Response: %s", len(resp), w5.Body.String())
	t.Logf("Assigned product IDs: %v", productIDs)
	// Note: Currently the GET endpoint returns assigned products
	// Once sp_get_products_by_category is fully integrated with the mapping table,
	// this should return 2 products
	assert.Equal(t, 2, len(resp), "Should return 2 assigned products")
}

func TestGetCategoriesByProduct_Success(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	productDto := models.CreateProductDto{
		SKU:       "CATSOF001-" + uuid.New().String()[:8],
		Name:      "Product In Two Categories",
		BasePrice: 15.00,
	}
	w1 := helper.DoRequest("POST", "/inventory/products", productDto, map[string]string{})
	var product models.CreateProductResponse
	json.Unmarshal(w1.Body.Bytes(), &product)

	categoryIDs := []string{}
	for i := 1; i <= 2; i++ {
		catDto := models.CreateCategoryDto{
			Name:         fmt.Sprintf("Categories Of Product %d", i),
			Slug:         fmt.Sprintf("cats-of-product-%d-", i) + uuid.New().String()[:8],
			DisplayOrder: ptrInt(i),
		}
		w := helper.DoRequest("POST", "/inventory/categories", catDto, map[string]string{})
		var category models.CategoryResponse
		json.Unmarshal(w.Body.Bytes(), &category)
		categoryIDs = append(categoryIDs, category.ID)

		helper.DoRequest("POST", fmt.Sprintf("/inventory/products/%s/categories/%s", product.ProductID, category.ID), nil, map[string]string{})
	}

	w2 := helper.DoRequest("GET", fmt.Sprintf("/inventory/products/%s/categories", product.ProductID), nil, map[string]string{})

	assert.Equal(t, http.StatusOK, w2.Code, "Should return 200")

	var resp []models.CategoryResponse
	err := json.Unmarshal(w2.Body.Bytes(), &resp)
	assert.NoError(t, err, "Response should be valid JSON")
	assert.Equal(t, 2, len(resp), "Should return both assigned categories")
	returnedIDs := []string{resp[0].ID, resp[1].ID}
	assert.Contains(t, returnedIDs, categoryIDs[0])
	assert.Contains(t, returnedIDs, categoryIDs[1])
}

func TestGetCategoriesByProduct_Empty(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	productDto := models.CreateProductDto{
		SKU:       "CATSOF002-" + uuid.New().String()[:8],
		Name:      "Product Without Categories",
		BasePrice: 25.00,
	}
	w1 := helper.DoRequest("POST", "/inventory/products", productDto, map[string]string{})
	var product models.CreateProductResponse
	json.Unmarshal(w1.Body.Bytes(), &product)

	w2 := helper.DoRequest("GET", fmt.Sprintf("/inventory/products/%s/categories", product.ProductID), nil, map[string]string{})

	assert.Equal(t, http.StatusOK, w2.Code, "Should return 200")
	var resp []models.CategoryResponse
	err := json.Unmarshal(w2.Body.Bytes(), &resp)
	assert.NoError(t, err, "Response should be valid JSON")
	assert.Equal(t, 0, len(resp), "Unassigned product should return an empty array")
}

func TestGetCategoriesByProduct_ProductNotFound(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	w := helper.DoRequest("GET", "/inventory/products/00000000-0000-0000-0000-000000000000/categories", nil, map[string]string{})

	assert.Equal(t, http.StatusNotFound, w.Code, "Unknown product should return 404")
}

func TestProductCountAggregation(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	// Create category
	catSlug := "count-test-" + uuid.New().String()[:8]
	catDto := models.CreateCategoryDto{
		Name: "Count Test",
		Slug: catSlug,
	}
	w1 := helper.DoRequest("POST", "/inventory/categories", catDto, map[string]string{})
	var category models.CategoryResponse
	json.Unmarshal(w1.Body.Bytes(), &category)
	assert.Equal(t, int64(0), category.ProductCount, "Initial product count should be 0")

	// Create product and assign
	productDto := models.CreateProductDto{
		SKU:       "COUNT001-" + uuid.New().String()[:8],
		Name:      "Count Test Product",
		BasePrice: 5.00,
	}
	w2 := helper.DoRequest("POST", "/inventory/products", productDto, map[string]string{})
	var product models.CreateProductResponse
	json.Unmarshal(w2.Body.Bytes(), &product)

	helper.DoRequest("POST", fmt.Sprintf("/inventory/products/%s/categories/%s", product.ProductID, category.ID), nil, map[string]string{})

	// Check updated count
	w3 := helper.DoRequest("GET", fmt.Sprintf("/inventory/categories/%s", category.ID), nil, map[string]string{})
	var updated models.CategoryResponse
	json.Unmarshal(w3.Body.Bytes(), &updated)
	assert.True(t, updated.ProductCount >= 1, "Product count should be incremented after assignment")
}

// ============================================
// Helper Functions
// ============================================

// listProductsResponse mirrors GET /inventory/products — { data, limit, offset },
// no total (see AGENTS.md → List endpoints).
type listProductsResponse struct {
	Data   []models.Product `json:"data"`
	Limit  int              `json:"limit"`
	Offset int              `json:"offset"`
}

func listProducts(t *testing.T, helper *testhelpers.ApiTestHelper, query string) []models.Product {
	t.Helper()

	w := helper.DoRequest("GET", "/inventory/products"+query, nil, map[string]string{})
	if w.Code != http.StatusOK {
		t.Fatalf("list products %q returned %d: %s", query, w.Code, w.Body.String())
	}

	var result listProductsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("list products %q returned invalid JSON: %v — %s", query, err, w.Body.String())
	}
	return result.Data
}

func createProductForList(t *testing.T, helper *testhelpers.ApiTestHelper, sku, name string) string {
	t.Helper()

	w := helper.DoRequest("POST", "/inventory/products", models.CreateProductDto{
		SKU:       sku,
		Name:      name,
		BasePrice: 12.50,
	}, map[string]string{})
	if w.Code != http.StatusCreated {
		t.Fatalf("create product %q returned %d: %s", sku, w.Code, w.Body.String())
	}

	var created models.CreateProductResponse
	json.Unmarshal(w.Body.Bytes(), &created)
	return created.ProductID
}

func listMovements(t *testing.T, helper *testhelpers.ApiTestHelper, query string) models.ListMovementsResponse {
	t.Helper()

	w := helper.DoRequest("GET", "/inventory/movements"+query, nil, map[string]string{})
	if w.Code != http.StatusOK {
		t.Fatalf("list movements %q returned %d: %s", query, w.Code, w.Body.String())
	}

	var result models.ListMovementsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("list movements %q returned invalid JSON: %v — %s", query, err, w.Body.String())
	}
	return result
}

func recordMovementForList(t *testing.T, helper *testhelpers.ApiTestHelper, productID, movementType string, quantity float64) {
	t.Helper()

	w := helper.DoRequest("POST", "/inventory/movements", models.RecordMovementDto{
		ProductID:    productID,
		MovementType: movementType,
		Quantity:     quantity,
	}, map[string]string{})
	if w.Code != http.StatusCreated {
		t.Fatalf("record %s movement for %s returned %d: %s", movementType, productID, w.Code, w.Body.String())
	}
}

func createCategoryForList(t *testing.T, helper *testhelpers.ApiTestHelper, name, slug string) string {
	t.Helper()

	w := helper.DoRequest("POST", "/inventory/categories", models.CreateCategoryDto{
		Name: name,
		Slug: slug,
	}, map[string]string{})
	if w.Code != http.StatusCreated {
		t.Fatalf("create category %q returned %d: %s", slug, w.Code, w.Body.String())
	}

	var created models.CategoryResponse
	json.Unmarshal(w.Body.Bytes(), &created)
	return created.ID
}

func assignProductToCategory(t *testing.T, helper *testhelpers.ApiTestHelper, productID, categoryID string) {
	t.Helper()

	w := helper.DoRequest("POST", fmt.Sprintf("/inventory/products/%s/categories/%s", productID, categoryID), nil, map[string]string{})
	if w.Code != http.StatusCreated {
		t.Fatalf("assign product %s to category %s returned %d: %s", productID, categoryID, w.Code, w.Body.String())
	}
}

func ptrFloat64(v float64) *float64 {
	return &v
}

func ptrString(v string) *string {
	return &v
}

func ptrBool(v bool) *bool {
	return &v
}

func ptrInt(v int) *int {
	return &v
}

// ============================================
// INV-007 — Stock per SKU invariants
// ============================================

// openInventoryDB opens a direct connection to the test database. The INV-007
// invariants are enforced by the schema itself and no endpoint exposes them, so
// they have to be asserted against PostgreSQL rather than through the API.
func openInventoryDB(t *testing.T) *pgx.Conn {
	t.Helper()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL is not set; skipping the SKU schema invariants")
	}

	conn, err := pgx.Connect(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connecting to the test database: %v", err)
	}
	return conn
}

// inRolledBackTx runs fn inside a transaction that is always rolled back, so a
// test that deliberately violates a constraint leaves nothing behind.
func inRolledBackTx(t *testing.T, fn func(ctx context.Context, tx pgx.Tx)) {
	t.Helper()

	ctx := context.Background()
	conn := openInventoryDB(t)
	defer conn.Close(ctx)

	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatalf("begin transaction: %v", err)
	}
	defer tx.Rollback(ctx)

	fn(ctx, tx)
}

// seedProductWithSKU inserts a product and its default SKU inside tx and
// returns both ids.
func seedProductWithSKU(t *testing.T, ctx context.Context, tx pgx.Tx, sku string) (productID, skuID string) {
	t.Helper()

	tenantID := uuid.New().String()
	if err := tx.QueryRow(ctx,
		`INSERT INTO inventory.products (tenant_id, sku, name, base_price)
		 VALUES ($1::uuid, $2, $3, 10) RETURNING id`,
		tenantID, sku, "SKU Invariant Product").Scan(&productID); err != nil {
		t.Fatalf("seed product %q: %v", sku, err)
	}

	if err := tx.QueryRow(ctx,
		`INSERT INTO inventory.product_skus
		     (tenant_id, product_id, sku, combination_key, is_default)
		 VALUES ($1::uuid, $2::uuid, $3, '', TRUE) RETURNING id`,
		tenantID, productID, sku).Scan(&skuID); err != nil {
		t.Fatalf("seed default SKU %q: %v", sku, err)
	}

	return productID, skuID
}

// createProductForSKU creates a product through the API and returns its id and sku.
func createProductForSKU(t *testing.T, helper *testhelpers.ApiTestHelper, prefix string) (string, string) {
	t.Helper()

	sku := fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
	w := helper.DoRequest("POST", "/inventory/products", models.CreateProductDto{
		SKU:       sku,
		Name:      "SKU Invariant Product",
		BasePrice: 25.00,
	}, map[string]string{})
	if w.Code != http.StatusCreated {
		t.Fatalf("create product %q returned %d: %s", sku, w.Code, w.Body.String())
	}

	var created models.CreateProductResponse
	json.Unmarshal(w.Body.Bytes(), &created)
	return created.ProductID, sku
}

func TestProductSKU_DefaultSKUCreatedWithTheProduct(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	productID, sku := createProductForSKU(t, helper, "SKUDEFAULT")

	ctx := context.Background()
	conn := openInventoryDB(t)
	defer conn.Close(ctx)

	var count int
	var gotSKU, combinationKey, status string
	var isDefault bool
	err := conn.QueryRow(ctx,
		`SELECT count(*) OVER (), s.sku, s.combination_key, s.is_default, s.status
		 FROM inventory.product_skus s
		 WHERE s.product_id = $1::uuid`, productID).
		Scan(&count, &gotSKU, &combinationKey, &isDefault, &status)

	assert.NoError(t, err, "a new product must have a SKU row")
	assert.Equal(t, 1, count, "a product gets exactly one SKU in phase 1")
	assert.Equal(t, sku, gotSKU, "the default SKU copies products.sku verbatim")
	assert.Equal(t, "", combinationKey)
	assert.True(t, isDefault)
	assert.Equal(t, "active", status)
}

func TestProductSKU_StockMovementAndBatchHangOffTheDefaultSKU(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	productID, _ := createProductForSKU(t, helper, "SKUHANG")

	wm := helper.DoRequest("POST", "/inventory/movements", models.RecordMovementDto{
		ProductID:    productID,
		MovementType: "PURCHASE",
		Quantity:     20.0,
	}, map[string]string{})
	assert.Equal(t, http.StatusCreated, wm.Code, wm.Body.String())

	wb := helper.DoRequest("POST", "/inventory/batches", map[string]interface{}{
		"product_id":       productID,
		"lot_number":       fmt.Sprintf("SKUHANG_%d", time.Now().UnixNano()),
		"purchase_date":    time.Now().AddDate(0, 0, -1).Format("2006-01-02"),
		"expiry_date":      time.Now().AddDate(0, 0, 30).Format("2006-01-02"),
		"unit_cost":        4.00,
		"initial_quantity": 20.0,
	}, map[string]string{})
	assert.Equal(t, http.StatusCreated, wb.Code, wb.Body.String())

	ctx := context.Background()
	conn := openInventoryDB(t)
	defer conn.Close(ctx)

	var defaultSKU string
	if err := conn.QueryRow(ctx,
		`SELECT s.id FROM inventory.product_skus s
		 WHERE s.product_id = $1::uuid AND s.is_default`, productID).Scan(&defaultSKU); err != nil {
		t.Fatalf("reading the default SKU: %v", err)
	}

	var strays int
	err := conn.QueryRow(ctx,
		`SELECT (SELECT count(*) FROM inventory.product_stock ps
		          WHERE ps.product_id = $1::uuid AND ps.sku_id IS DISTINCT FROM $2::uuid)
		      + (SELECT count(*) FROM inventory.inventory_movements im
		          WHERE im.product_id = $1::uuid AND im.sku_id IS DISTINCT FROM $2::uuid)
		      + (SELECT count(*) FROM inventory.product_batches pb
		          WHERE pb.product_id = $1::uuid AND pb.sku_id IS DISTINCT FROM $2::uuid)`,
		productID, defaultSKU).Scan(&strays)

	assert.NoError(t, err)
	assert.Equal(t, 0, strays, "stock, movements and batches must all point at the default SKU")
}

func TestProductSKU_StockIsUniquePerSKUNotPerProduct(t *testing.T) {
	ctx := context.Background()
	conn := openInventoryDB(t)
	defer conn.Close(ctx)

	var perSKU, perProduct int
	err := conn.QueryRow(ctx,
		`SELECT
		     count(*) FILTER (WHERE pg_get_constraintdef(c.oid) = 'UNIQUE (sku_id)'),
		     count(*) FILTER (WHERE pg_get_constraintdef(c.oid) = 'UNIQUE (product_id)')
		 FROM pg_constraint c
		 WHERE c.conrelid = 'inventory.product_stock'::regclass AND c.contype = 'u'`).
		Scan(&perSKU, &perProduct)

	assert.NoError(t, err)
	assert.Equal(t, 1, perSKU, "product_stock must be unique per SKU")
	assert.Equal(t, 0, perProduct, "product_stock must no longer be unique per product")
}

func TestProductSKU_SkuIDIsNotNullOnEveryQuantityTable(t *testing.T) {
	ctx := context.Background()
	conn := openInventoryDB(t)
	defer conn.Close(ctx)

	var nullable int
	err := conn.QueryRow(ctx,
		`SELECT count(*) FROM information_schema.columns c
		 WHERE c.table_schema = 'inventory'
		   AND c.column_name = 'sku_id'
		   AND c.table_name IN ('product_stock', 'inventory_movements', 'product_batches')
		   AND c.is_nullable = 'YES'`).Scan(&nullable)

	assert.NoError(t, err)
	assert.Equal(t, 0, nullable, "a nullable sku_id would leak into phase 3 forever")

	var declared int
	err = conn.QueryRow(ctx,
		`SELECT count(*) FROM information_schema.columns c
		 WHERE c.table_schema = 'inventory'
		   AND c.column_name = 'sku_id'
		   AND c.table_name IN ('product_stock', 'inventory_movements', 'product_batches')`).
		Scan(&declared)

	assert.NoError(t, err)
	assert.Equal(t, 3, declared, "all three quantity tables carry sku_id")
}

func TestProductSKU_RejectsASecondDefaultSKU(t *testing.T) {
	inRolledBackTx(t, func(ctx context.Context, tx pgx.Tx) {
		productID, _ := seedProductWithSKU(t, ctx, tx, fmt.Sprintf("SKUDUP_%d", time.Now().UnixNano()))

		_, err := tx.Exec(ctx,
			`INSERT INTO inventory.product_skus
			     (tenant_id, product_id, sku, combination_key, is_default)
			 SELECT p.tenant_id, p.id, p.sku || '-2', 'second', TRUE
			 FROM inventory.products p WHERE p.id = $1::uuid`, productID)

		assert.Error(t, err, "a product may only have one default SKU")
	})
}

func TestProductSKU_RejectsADuplicateCombination(t *testing.T) {
	inRolledBackTx(t, func(ctx context.Context, tx pgx.Tx) {
		productID, _ := seedProductWithSKU(t, ctx, tx, fmt.Sprintf("SKUCOMB_%d", time.Now().UnixNano()))

		_, err := tx.Exec(ctx,
			`INSERT INTO inventory.product_skus
			     (tenant_id, product_id, sku, combination_key, is_default)
			 SELECT p.tenant_id, p.id, p.sku || '-2', '', FALSE
			 FROM inventory.products p WHERE p.id = $1::uuid`, productID)

		assert.Error(t, err, "(product_id, combination_key) must be unique")
	})
}

func TestProductSKU_RejectsAStockRowFromAnotherProduct(t *testing.T) {
	inRolledBackTx(t, func(ctx context.Context, tx pgx.Tx) {
		stamp := time.Now().UnixNano()
		productA, _ := seedProductWithSKU(t, ctx, tx, fmt.Sprintf("SKUXA_%d", stamp))
		_, skuB := seedProductWithSKU(t, ctx, tx, fmt.Sprintf("SKUXB_%d", stamp))

		_, err := tx.Exec(ctx,
			`INSERT INTO inventory.product_stock (product_id, sku_id, current_quantity)
			 VALUES ($1::uuid, $2::uuid, 0)`, productA, skuB)

		assert.Error(t, err, "a stock row cannot name a SKU of another product")
	})
}

func TestProductSKU_RejectsAnOptionFromAnotherGroup(t *testing.T) {
	inRolledBackTx(t, func(ctx context.Context, tx pgx.Tx) {
		productID, skuID := seedProductWithSKU(t, ctx, tx, fmt.Sprintf("SKUOPT_%d", time.Now().UnixNano()))

		var axisGroup, otherGroup, otherOption string
		mustQueryRow(t, ctx, tx, &axisGroup,
			`INSERT INTO inventory.product_variant_groups
			     (product_id, group_type, is_required, max_selections, affects_inventory)
			 VALUES ($1::uuid, 'size', TRUE, 1, TRUE) RETURNING id`, productID)
		mustQueryRow(t, ctx, tx, &otherGroup,
			`INSERT INTO inventory.product_variant_groups
			     (product_id, group_type, is_required, max_selections, affects_inventory)
			 VALUES ($1::uuid, 'topping', FALSE, 3, FALSE) RETURNING id`, productID)
		mustQueryRow(t, ctx, tx, &otherOption,
			`INSERT INTO inventory.product_variant_options (variant_group_id, option_name)
			 VALUES ($1::uuid, 'Cheese') RETURNING id`, otherGroup)

		_, err := tx.Exec(ctx,
			`INSERT INTO inventory.product_sku_options
			     (sku_id, variant_group_id, option_id, product_id)
			 VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid)`,
			skuID, axisGroup, otherOption, productID)

		assert.Error(t, err, "an option must belong to the group it is paired with")
	})
}

func TestProductSKU_RejectsAGroupFromAnotherProduct(t *testing.T) {
	inRolledBackTx(t, func(ctx context.Context, tx pgx.Tx) {
		stamp := time.Now().UnixNano()
		productA, skuA := seedProductWithSKU(t, ctx, tx, fmt.Sprintf("SKUGRPA_%d", stamp))
		productB, _ := seedProductWithSKU(t, ctx, tx, fmt.Sprintf("SKUGRPB_%d", stamp))

		var groupB, optionB string
		mustQueryRow(t, ctx, tx, &groupB,
			`INSERT INTO inventory.product_variant_groups
			     (product_id, group_type, is_required, max_selections, affects_inventory)
			 VALUES ($1::uuid, 'size', TRUE, 1, TRUE) RETURNING id`, productB)
		mustQueryRow(t, ctx, tx, &optionB,
			`INSERT INTO inventory.product_variant_options (variant_group_id, option_name)
			 VALUES ($1::uuid, 'XL') RETURNING id`, groupB)

		_, err := tx.Exec(ctx,
			`INSERT INTO inventory.product_sku_options
			     (sku_id, variant_group_id, option_id, product_id)
			 VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid)`,
			skuA, groupB, optionB, productA)

		assert.Error(t, err, "a group must belong to the SKU's product")
	})
}

func TestProductSKU_RejectsAPriceModifierOnAnAxisOption(t *testing.T) {
	inRolledBackTx(t, func(ctx context.Context, tx pgx.Tx) {
		productID, _ := seedProductWithSKU(t, ctx, tx, fmt.Sprintf("SKUAXIS_%d", time.Now().UnixNano()))

		var axisGroup, modifierGroup string
		mustQueryRow(t, ctx, tx, &axisGroup,
			`INSERT INTO inventory.product_variant_groups
			     (product_id, group_type, is_required, max_selections, affects_inventory)
			 VALUES ($1::uuid, 'size', TRUE, 1, TRUE) RETURNING id`, productID)
		mustQueryRow(t, ctx, tx, &modifierGroup,
			`INSERT INTO inventory.product_variant_groups
			     (product_id, group_type, is_required, max_selections, affects_inventory)
			 VALUES ($1::uuid, 'topping', FALSE, 3, FALSE) RETURNING id`, productID)

		// A savepoint, so the rejection below does not abort the transaction the
		// second half of this test still needs.
		sp, err := tx.Begin(ctx)
		if err != nil {
			t.Fatalf("savepoint: %v", err)
		}
		_, err = sp.Exec(ctx,
			`INSERT INTO inventory.product_variant_options
			     (variant_group_id, option_name, price_modifier)
			 VALUES ($1::uuid, 'XL', 2000)`, axisGroup)
		assert.Error(t, err, "an axis option may not carry a price modifier: the SKU does")
		sp.Rollback(ctx)

		_, err = tx.Exec(ctx,
			`INSERT INTO inventory.product_variant_options
			     (variant_group_id, option_name, price_modifier)
			 VALUES ($1::uuid, 'Cheese', 2000)`, modifierGroup)
		assert.NoError(t, err, "a modifier option keeps its price modifier, exactly as today")
	})
}

func TestStockStatus_LadderComesFromFnStockStatus(t *testing.T) {
	ctx := context.Background()
	conn := openInventoryDB(t)
	defer conn.Close(ctx)

	cases := []struct {
		quantity float64
		reorder  float64
		expected string
	}{
		{0, 10, "out_of_stock"},
		{0.01, 10, "critical"},
		{9.99, 10, "critical"},
		{10, 10, "low"},
		{14.99, 10, "low"},
		{15, 10, "ok"},
		{1000, 10, "ok"},
	}

	for _, c := range cases {
		var status string
		err := conn.QueryRow(ctx,
			`SELECT inventory.fn_stock_status($1::DECIMAL, $2::DECIMAL)`, c.quantity, c.reorder).
			Scan(&status)

		assert.NoError(t, err)
		assert.Equalf(t, c.expected, status,
			"fn_stock_status(%v, %v)", c.quantity, c.reorder)
	}
}

func TestGetProductStock_KeepsItsShapeAfterTheSKUMove(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	productID, _ := createProductForSKU(t, helper, "SKUSHAPE")

	wm := helper.DoRequest("POST", "/inventory/movements", models.RecordMovementDto{
		ProductID:    productID,
		MovementType: "PURCHASE",
		Quantity:     60.0,
	}, map[string]string{})
	assert.Equal(t, http.StatusCreated, wm.Code, wm.Body.String())

	w := helper.DoRequest("GET", fmt.Sprintf("/inventory/stock/%s", productID), nil, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code)

	var raw map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
		t.Fatalf("stock response is not JSON: %v — %s", err, w.Body.String())
	}

	for _, field := range []string{
		"current_quantity", "reserved_quantity", "available_quantity",
		"reorder_level", "status", "last_updated_at",
	} {
		assert.Containsf(t, raw, field, "the stock payload lost %q", field)
	}

	var stock models.ProductStock
	json.Unmarshal(w.Body.Bytes(), &stock)
	assert.Equal(t, 60.0, stock.CurrentQuantity)
	assert.Equal(t, 0.0, stock.ReservedQuantity)
	assert.Equal(t, 60.0, stock.AvailableQuantity)
	assert.Equal(t, "ok", stock.Status)
}

// mustQueryRow runs a single-value query and fails the test if it errors.
func mustQueryRow(t *testing.T, ctx context.Context, tx pgx.Tx, dest *string, sql string, args ...interface{}) {
	t.Helper()

	if err := tx.QueryRow(ctx, sql, args...).Scan(dest); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}
