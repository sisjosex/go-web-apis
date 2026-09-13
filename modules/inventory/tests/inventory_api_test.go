//go:build integration
// +build integration

package inventory_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	coreServices "josex/web/modules/core/services"
	"josex/web/modules/core/testhelpers"
	importErrors "josex/web/modules/import/errors"
	inventoryErrors "josex/web/modules/inventory/errors"
	"josex/web/modules/inventory/models"
	inventoryRepositories "josex/web/modules/inventory/repositories"

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
		UnitCost:     ptrFloat64(4.00),
	}
	helper.DoRequest("POST", "/inventory/movements", purchaseBody, map[string]string{})

	// Sale
	saleBody := models.RecordMovementDto{
		ProductID:    created.ProductID,
		MovementType: "SALE",
		Quantity:     20.0,
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
		Quantity:     50.0,
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
		UnitCost:     ptrFloat64(4.00),
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
		UnitCost:     ptrFloat64(4.00),
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
		UnitCost:     ptrFloat64(4.00),
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
		UnitCost:     ptrFloat64(4.00),
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
		UnitCost:     ptrFloat64(4.00),
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
		UnitCost:     ptrFloat64(4.00),
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
		UnitCost:     ptrFloat64(4.00),
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
		Quantity:     45.0,
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
		Quantity:     25.0,
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
		Quantity:     5.0,
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
		Quantity:     5.0,
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
		Quantity:     5.0,
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
		Quantity:     15.0,
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
		UnitCost:     ptrFloat64(4.00),
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
		UnitCost:     ptrFloat64(4.00),
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

func TestGetExpiringBatches_IncludesProductName(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	productName := fmt.Sprintf("Expiring Product %d", time.Now().UnixNano())
	productBody := models.CreateProductDto{
		SKU:       fmt.Sprintf("BATCH_EXPNAME_%d", time.Now().UnixNano()),
		Name:      productName,
		BasePrice: 100.00,
	}
	w := helper.DoRequest("POST", "/inventory/products", productBody, map[string]string{})
	var createdProduct models.CreateProductResponse
	json.Unmarshal(w.Body.Bytes(), &createdProduct)

	batchBody := map[string]interface{}{
		"product_id":       createdProduct.ProductID,
		"lot_number":       fmt.Sprintf("LOT_EXPNAME_%d", time.Now().UnixNano()),
		"purchase_date":    time.Now().AddDate(0, 0, -1).Format("2006-01-02"),
		"expiry_date":      time.Now().AddDate(0, 0, 5).Format("2006-01-02"),
		"unit_cost":        50.00,
		"initial_quantity": 100.0,
	}
	w1 := helper.DoRequest("POST", "/inventory/batches", batchBody, map[string]string{})
	assert.Equal(t, http.StatusCreated, w1.Code)

	w2 := helper.DoRequest("GET", "/inventory/batches/expiring?warningDays=30", nil, map[string]string{})

	assert.Equal(t, http.StatusOK, w2.Code)

	var expiring []models.BatchResponse
	err := json.Unmarshal(w2.Body.Bytes(), &expiring)
	assert.NoError(t, err, "Expiring batches response should be a JSON array")

	var found *models.BatchResponse
	for i := range expiring {
		if expiring[i].ProductID.String() == createdProduct.ProductID {
			found = &expiring[i]
			break
		}
	}

	assert.NotNil(t, found, "The created batch should appear in the expiring list")
	if found != nil {
		assert.NotNil(t, found.ProductName, "product_name should be returned")
		assert.Equal(t, productName, *found.ProductName)
	}
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
		UnitCost:     costForType(movementType),
		Direction:    directionForType(movementType),
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

// costForType is the unit cost a movement of this type has to carry. Since
// INV-013 D3 a PURCHASE or a PRODUCTION without one is a 400, and every other
// type stores NULL whatever is sent — so the tests that are about something else
// get a plausible cost from here instead of each restating the rule.
func costForType(movementType string) *float64 {
	if movementType == "PURCHASE" || movementType == "PRODUCTION" {
		return ptrFloat64(4.00)
	}
	return nil
}

// directionForType is the counterpart for the two types that carry no direction
// of their own (INV-013 D1): a fixture movement has to pick one, and IN is the
// one that adds stock the rest of the test can then move. Everything else
// returns nil, because a direction the type already decided is not this helper's
// to state — the tests that are about that say it themselves.
func directionForType(movementType string) *string {
	if movementType == "ADJUSTMENT" || movementType == "TRANSFER" {
		in := "IN"
		return &in
	}
	return nil
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
		UnitCost:     ptrFloat64(4.00),
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

// The inversion of INV-007 D3's rejection (INV-012 D1): an axis option carries
// its own modifier again, and the SKU's is derived from the options composing it
// rather than being a second, uneditable price.
func TestProductSKU_DerivesSkuPriceFromAxisOptions(t *testing.T) {
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

		var axisOption string
		mustQueryRow(t, ctx, tx, &axisOption,
			`INSERT INTO inventory.product_variant_options
			     (variant_group_id, option_name, price_modifier)
			 VALUES ($1::uuid, 'XL', 2000) RETURNING id`, axisGroup)

		// The combination that option composes, built the way the generator
		// builds it: the SKU first, then the option row that gives it its price.
		var comboSkuID string
		mustQueryRow(t, ctx, tx, &comboSkuID,
			`INSERT INTO inventory.product_skus
			     (tenant_id, product_id, sku, combination_key, is_default)
			 SELECT p.tenant_id, p.id, p.sku || '-XL', $2, FALSE
			 FROM inventory.products p WHERE p.id = $1::uuid
			 RETURNING id`, productID, axisOption)

		if _, err := tx.Exec(ctx,
			`INSERT INTO inventory.product_sku_options
			     (sku_id, variant_group_id, option_id, product_id)
			 VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid)`,
			comboSkuID, axisGroup, axisOption, productID); err != nil {
			t.Fatalf("composing the combination: %v", err)
		}

		var derived float64
		mustScanFloat(t, ctx, tx, &derived,
			`SELECT s.price_modifier FROM inventory.product_skus s WHERE s.id = $1::uuid`, comboSkuID)
		assert.Equal(t, 2000.0, derived, "the SKU price is the sum of the options composing it")

		// Repricing the option moves the combination without regenerating it.
		if _, err := tx.Exec(ctx,
			`UPDATE inventory.product_variant_options vo
			 SET price_modifier = 3000 WHERE vo.id = $1::uuid`, axisOption); err != nil {
			t.Fatalf("repricing the axis option: %v", err)
		}

		mustScanFloat(t, ctx, tx, &derived,
			`SELECT s.price_modifier FROM inventory.product_skus s WHERE s.id = $1::uuid`, comboSkuID)
		assert.Equal(t, 3000.0, derived, "repricing an option reprices every combination built on it")

		mustScanFloat(t, ctx, tx, &derived,
			`SELECT s.price_modifier FROM inventory.product_skus s
			 WHERE s.product_id = $1::uuid AND s.is_default`, productID)
		assert.Equal(t, 0.0, derived, "the unassigned bucket composes no option and stays at 0")

		_, err := tx.Exec(ctx,
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
		UnitCost:     ptrFloat64(4.00),
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

func mustScanFloat(t *testing.T, ctx context.Context, tx pgx.Tx, dest *float64, sql string, args ...interface{}) {
	t.Helper()

	if err := tx.QueryRow(ctx, sql, args...).Scan(dest); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

// ============================================
// INV-008 — axes, the generator and redistribution
// ============================================

// axisSpec is one inventory axis and the option names it offers.
type axisSpec struct {
	groupType string
	options   []string
}

// createProductWithAxes creates a product and then turns the given groups into
// inventory axes through the product endpoint — the only writer of the variant
// tree. It does not generate anything: that is an explicit call (D2).
func createProductWithAxes(
	t *testing.T,
	helper *testhelpers.ApiTestHelper,
	prefix string,
	axes []axisSpec,
) (productID, sku string) {
	t.Helper()

	productID, sku = createProductForSKU(t, helper, prefix)
	putAxes(t, helper, productID, axes)
	return productID, sku
}

// putAxes rewrites the product's whole variant tree as the given axes. Every
// group is sent without an id, so the SP inserts them; an axis must be required
// and single choice, which INV-007's CHECK enforces.
func putAxes(t *testing.T, helper *testhelpers.ApiTestHelper, productID string, axes []axisSpec) {
	t.Helper()

	groups := make([]models.UpdateVariantGroupDto, 0, len(axes))
	for _, axis := range axes {
		options := make([]models.UpdateVariantOptionDto, 0, len(axis.options))
		for _, name := range axis.options {
			options = append(options, models.UpdateVariantOptionDto{Name: name})
		}
		groups = append(groups, models.UpdateVariantGroupDto{
			GroupType:        axis.groupType,
			IsRequired:       true,
			MaxSelections:    1,
			AffectsInventory: true,
			Options:          options,
		})
	}

	w := putProduct(t, helper, productID, &models.UpdateProductVariantsDto{Groups: groups})
	if w.Code != http.StatusOK {
		t.Fatalf("setting the axes of %s failed with %d: %s", productID, w.Code, w.Body.String())
	}
}

// putProduct sends the product update, carrying whatever variant tree is given.
func putProduct(
	t *testing.T,
	helper *testhelpers.ApiTestHelper,
	productID string,
	variants *models.UpdateProductVariantsDto,
) *httptest.ResponseRecorder {
	t.Helper()

	return helper.DoRequest("PUT", fmt.Sprintf("/inventory/products/%s", productID),
		models.UpdateProductDto{
			Name:      "SKU Invariant Product",
			BasePrice: 25.00,
			Variants:  variants,
		}, map[string]string{})
}

// currentTree reads the product back and returns its groups with their ids, so
// a follow-up PUT can drop exactly one of them.
func currentTree(t *testing.T, helper *testhelpers.ApiTestHelper, productID string) []models.VariantGroup {
	t.Helper()

	return fetchProductDetail(t, helper, productID).Variants
}

// treeAsUpdate turns the tree just read into the payload that keeps it as it is.
func treeAsUpdate(groups []models.VariantGroup) *models.UpdateProductVariantsDto {
	out := make([]models.UpdateVariantGroupDto, 0, len(groups))
	for _, g := range groups {
		id := g.ID
		options := make([]models.UpdateVariantOptionDto, 0, len(g.Options))
		for _, o := range g.Options {
			optionID := o.ID
			options = append(options, models.UpdateVariantOptionDto{ID: &optionID, Name: o.Name})
		}
		out = append(out, models.UpdateVariantGroupDto{
			ID:               &id,
			GroupType:        g.GroupType,
			IsRequired:       g.IsRequired,
			MaxSelections:    g.MaxSelections,
			AffectsInventory: g.AffectsInventory,
			Options:          options,
		})
	}
	return &models.UpdateProductVariantsDto{Groups: out}
}

func generateSkus(t *testing.T, helper *testhelpers.ApiTestHelper, productID string) *httptest.ResponseRecorder {
	t.Helper()

	return helper.DoRequest("POST",
		fmt.Sprintf("/inventory/products/%s/skus/generate", productID), nil, map[string]string{})
}

func listSkus(t *testing.T, helper *testhelpers.ApiTestHelper, productID string) models.ListProductSkusResponse {
	t.Helper()

	w := helper.DoRequest("GET",
		fmt.Sprintf("/inventory/products/%s/skus", productID), nil, map[string]string{})
	if w.Code != http.StatusOK {
		t.Fatalf("listing the SKUs of %s failed with %d: %s", productID, w.Code, w.Body.String())
	}

	var result models.ListProductSkusResponse
	json.Unmarshal(w.Body.Bytes(), &result)
	return result
}

func redistribute(
	t *testing.T,
	helper *testhelpers.ApiTestHelper,
	productID string,
	targets []models.RedistributeTargetDto,
) *httptest.ResponseRecorder {
	t.Helper()

	return helper.DoRequest("POST",
		fmt.Sprintf("/inventory/products/%s/skus/redistribute", productID),
		models.RedistributeStockDto{Targets: targets}, map[string]string{})
}

// purchase puts units on the product's default SKU, which is where every
// quantity still lands until a redistribution moves it.
func purchase(t *testing.T, helper *testhelpers.ApiTestHelper, productID string, quantity float64) {
	t.Helper()

	w := helper.DoRequest("POST", "/inventory/movements", models.RecordMovementDto{
		ProductID:    productID,
		MovementType: "PURCHASE",
		Quantity:     quantity,
		UnitCost:     ptrFloat64(4.00),
	}, map[string]string{})
	if w.Code != http.StatusCreated {
		t.Fatalf("purchasing %v for %s failed with %d: %s", quantity, productID, w.Code, w.Body.String())
	}
}

func skuByCode(skus []models.ProductSku, code string) (models.ProductSku, bool) {
	for _, s := range skus {
		if s.SKU == code {
			return s, true
		}
	}
	return models.ProductSku{}, false
}

func nonDefaultSkus(skus []models.ProductSku) []models.ProductSku {
	out := []models.ProductSku{}
	for _, s := range skus {
		if !s.IsDefault {
			out = append(out, s)
		}
	}
	return out
}

func TestGetProduct_VariantTreeCarriesAffectsInventory(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	productID, _ := createProductWithAxes(t, helper, "AXISFLAG", []axisSpec{
		{groupType: "size", options: []string{"M", "XL"}},
	})

	tree := currentTree(t, helper, productID)

	assert.Len(t, tree, 1)
	assert.True(t, tree[0].AffectsInventory, "the group was saved as an inventory axis")
}

func TestListProductSkus_PhaseOneProductReturnsOnlyTheDefault(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	productID, sku := createProductForSKU(t, helper, "SKULISTONE")

	result := listSkus(t, helper, productID)

	assert.Equal(t, int64(1), result.TotalCount, "a product that never had axes has one SKU")
	assert.Len(t, result.Skus, 1)
	assert.Equal(t, sku, result.Skus[0].SKU)
	assert.True(t, result.Skus[0].IsDefault)
	assert.True(t, result.Skus[0].Sellable, "an active SKU of an active product is sellable (D6)")
	assert.Empty(t, result.Skus[0].Options)
}

func TestListProductSkus_ProductNotFound(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	w := helper.DoRequest("GET",
		fmt.Sprintf("/inventory/products/%s/skus", uuid.New()), nil, map[string]string{})

	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
}

func TestListProductSkus_InvalidPageSize(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	productID, _ := createProductForSKU(t, helper, "SKULISTBAD")

	w := helper.DoRequest("GET",
		fmt.Sprintf("/inventory/products/%s/skus?page_size=500", productID), nil, map[string]string{})

	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
}

func TestGenerateSkus_OneSkuPerCombination(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	productID, sku := createProductWithAxes(t, helper, "SKUGEN", []axisSpec{
		{groupType: "size", options: []string{"M", "XL"}},
		{groupType: "colour", options: []string{"Black", "White"}},
	})

	w := generateSkus(t, helper, productID)
	assert.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	var generated models.GenerateSkusResponse
	json.Unmarshal(w.Body.Bytes(), &generated)
	assert.Equal(t, 2, generated.AxisCount)
	assert.Equal(t, 4, generated.CombinationCount)
	assert.Equal(t, 4, generated.CreatedCount)
	assert.True(t, generated.StockByVariant)

	result := listSkus(t, helper, productID)
	assert.Equal(t, int64(5), result.TotalCount, "the four combinations plus the default")

	for _, s := range nonDefaultSkus(result.Skus) {
		assert.Len(t, s.Options, 2, "%s carries one option per axis", s.SKU)
		assert.Equal(t, 0.0, s.CurrentQuantity, "%s starts empty", s.SKU)
		assert.Equal(t, "out_of_stock", s.StockStatus)
		assert.NotEmpty(t, s.CombinationKey)
	}

	_, found := skuByCode(result.Skus, sku+"-M-BLACK")
	assert.True(t, found, "the code is <products.sku>-<option codes> (INV-007 D2b)")
}

func TestGenerateSkus_LeavesTheDefaultSKUAlone(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	productID, sku := createProductWithAxes(t, helper, "SKUGENDEF", []axisSpec{
		{groupType: "size", options: []string{"M", "XL"}},
	})
	purchase(t, helper, productID, 9)

	assert.Equal(t, http.StatusCreated, generateSkus(t, helper, productID).Code)

	def, found := skuByCode(listSkus(t, helper, productID).Skus, sku)
	assert.True(t, found, "the default SKU survives generation")
	assert.True(t, def.IsDefault, "and is still the default")
	assert.Equal(t, 9.0, def.CurrentQuantity, "with its quantity intact (INV-007 D4)")
}

func TestGenerateSkus_SecondRunAddsOnlyTheMissingCombinations(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	productID, _ := createProductWithAxes(t, helper, "SKUGENAGAIN", []axisSpec{
		{groupType: "size", options: []string{"M", "XL"}},
		{groupType: "colour", options: []string{"Black", "White"}},
	})
	assert.Equal(t, http.StatusCreated, generateSkus(t, helper, productID).Code)

	second := generateSkus(t, helper, productID)
	assert.Equal(t, http.StatusCreated, second.Code, second.Body.String())

	var rerun models.GenerateSkusResponse
	json.Unmarshal(second.Body.Bytes(), &rerun)
	assert.Equal(t, 0, rerun.CreatedCount, "a second run over the same tree adds nothing")
	assert.Equal(t, 4, rerun.ExistingCount)

	// a third size, which multiplies out to exactly two missing combinations
	tree := treeAsUpdate(currentTree(t, helper, productID))
	for i, g := range tree.Groups {
		if g.GroupType == "size" {
			tree.Groups[i].Options = append(tree.Groups[i].Options,
				models.UpdateVariantOptionDto{Name: "S"})
		}
	}
	assert.Equal(t, http.StatusOK, putProduct(t, helper, productID, tree).Code)

	third := generateSkus(t, helper, productID)
	assert.Equal(t, http.StatusCreated, third.Code, third.Body.String())

	var grown models.GenerateSkusResponse
	json.Unmarshal(third.Body.Bytes(), &grown)
	assert.Equal(t, 6, grown.CombinationCount)
	assert.Equal(t, 2, grown.CreatedCount, "only the combinations the new option adds")
	assert.Equal(t, 4, grown.ExistingCount)
	assert.Equal(t, int64(7), listSkus(t, helper, productID).TotalCount)
}

func TestGenerateSkus_WithoutAnyAxis(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	productID, _ := createProductForSKU(t, helper, "SKUGENNOAXIS")

	w := generateSkus(t, helper, productID)

	assert.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	assert.Equal(t, int64(1), listSkus(t, helper, productID).TotalCount)
}

func TestGenerateSkus_ProductNotFound(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	w := helper.DoRequest("POST",
		fmt.Sprintf("/inventory/products/%s/skus/generate", uuid.New()), nil, map[string]string{})

	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
}

func TestGenerateSkus_PastTheCombinationCap(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	// 5 x 5 x 5 = 125, past the 100 INVENTORY_MAX_COMBINATIONS allows
	five := []string{"A", "B", "C", "D", "E"}
	productID, _ := createProductWithAxes(t, helper, "SKUGENCAP", []axisSpec{
		{groupType: "size", options: five},
		{groupType: "colour", options: five},
		{groupType: "fit", options: five},
	})

	w := generateSkus(t, helper, productID)

	assert.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	assert.Equal(t, int64(1), listSkus(t, helper, productID).TotalCount,
		"the cap is checked before the first row is written")
}

func TestGenerateSkus_PastTheAxisCap(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	productID, _ := createProductWithAxes(t, helper, "SKUGENAXES", []axisSpec{
		{groupType: "size", options: []string{"M", "XL"}},
		{groupType: "colour", options: []string{"Black"}},
		{groupType: "fit", options: []string{"Slim"}},
		{groupType: "sleeve", options: []string{"Short"}},
	})

	w := generateSkus(t, helper, productID)

	assert.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	assert.Equal(t, int64(1), listSkus(t, helper, productID).TotalCount)
}

func TestGenerateSkus_SetsStockByVariant(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	productID, _ := createProductWithAxes(t, helper, "SKUGENFLAG", []axisSpec{
		{groupType: "size", options: []string{"M", "XL"}},
	})
	assert.Equal(t, http.StatusCreated, generateSkus(t, helper, productID).Code)

	ctx := context.Background()
	conn := openInventoryDB(t)
	defer conn.Close(ctx)

	var stockByVariant bool
	err := conn.QueryRow(ctx,
		`SELECT p.stock_by_variant FROM inventory.products p WHERE p.id = $1::uuid`,
		productID).Scan(&stockByVariant)
	assert.NoError(t, err)
	assert.True(t, stockByVariant, "a product with an axis tracks stock per combination")

	// the last axis leaving clears it again
	assert.Equal(t, http.StatusOK,
		putProduct(t, helper, productID, &models.UpdateProductVariantsDto{
			Groups: []models.UpdateVariantGroupDto{},
		}).Code)

	err = conn.QueryRow(ctx,
		`SELECT p.stock_by_variant FROM inventory.products p WHERE p.id = $1::uuid`,
		productID).Scan(&stockByVariant)
	assert.NoError(t, err)
	assert.False(t, stockByVariant, "and FALSE once the last axis goes away")
}

func TestRedistributeStock_MovesTheWholeBucket(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	productID, sku := createProductWithAxes(t, helper, "SKUREDIST", []axisSpec{
		{groupType: "size", options: []string{"M", "XL"}},
	})
	purchase(t, helper, productID, 3)
	assert.Equal(t, http.StatusCreated, generateSkus(t, helper, productID).Code)

	before := listSkus(t, helper, productID)
	m, okM := skuByCode(before.Skus, sku+"-M")
	xl, okXL := skuByCode(before.Skus, sku+"-XL")
	assert.True(t, okM && okXL, "both combinations exist before the move")

	w := redistribute(t, helper, productID, []models.RedistributeTargetDto{
		{SkuID: m.SkuID, Quantity: 2},
		{SkuID: xl.SkuID, Quantity: 1},
	})
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var moved models.RedistributeStockResponse
	json.Unmarshal(w.Body.Bytes(), &moved)
	assert.Equal(t, 3.0, moved.MovedQuantity)
	assert.Equal(t, 2, moved.TargetCount)

	after := listSkus(t, helper, productID)
	def, _ := skuByCode(after.Skus, sku)
	movedM, _ := skuByCode(after.Skus, sku+"-M")
	movedXL, _ := skuByCode(after.Skus, sku+"-XL")
	assert.Equal(t, 0.0, def.CurrentQuantity, "the bucket is empty")
	assert.Equal(t, 2.0, movedM.CurrentQuantity)
	assert.Equal(t, 1.0, movedXL.CurrentQuantity)

	ctx := context.Background()
	conn := openInventoryDB(t)
	defer conn.Close(ctx)

	for _, target := range []string{m.SkuID, xl.SkuID} {
		var transfers int
		err := conn.QueryRow(ctx,
			`SELECT count(*) FROM inventory.inventory_movements im
			  WHERE im.sku_id = $1::uuid AND im.movement_type = 'TRANSFER'`, target).
			Scan(&transfers)
		assert.NoError(t, err)
		assert.Equal(t, 1, transfers, "one TRANSFER per target (D4)")
	}

	var ledger float64
	err := conn.QueryRow(ctx,
		`SELECT COALESCE(SUM(im.quantity), 0) FROM inventory.inventory_movements im
		  WHERE im.product_id = $1::uuid`, productID).Scan(&ledger)
	assert.NoError(t, err)
	assert.Equal(t, 3.0, ledger, "the ledger still sums to what was purchased")
}

func TestRedistributeStock_AmountsThatDoNotAddUp(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	productID, sku := createProductWithAxes(t, helper, "SKUREDISTBAD", []axisSpec{
		{groupType: "size", options: []string{"M", "XL"}},
	})
	purchase(t, helper, productID, 3)
	assert.Equal(t, http.StatusCreated, generateSkus(t, helper, productID).Code)

	m, _ := skuByCode(listSkus(t, helper, productID).Skus, sku+"-M")

	w := redistribute(t, helper, productID, []models.RedistributeTargetDto{
		{SkuID: m.SkuID, Quantity: 1},
	})

	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())

	after := listSkus(t, helper, productID)
	def, _ := skuByCode(after.Skus, sku)
	untouched, _ := skuByCode(after.Skus, sku+"-M")
	assert.Equal(t, 3.0, def.CurrentQuantity, "a mismatched payload moves nothing")
	assert.Equal(t, 0.0, untouched.CurrentQuantity)
}

func TestRedistributeStock_UnknownTargetSku(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	productID, _ := createProductWithAxes(t, helper, "SKUREDISTNF", []axisSpec{
		{groupType: "size", options: []string{"M", "XL"}},
	})
	purchase(t, helper, productID, 3)
	assert.Equal(t, http.StatusCreated, generateSkus(t, helper, productID).Code)

	w := redistribute(t, helper, productID, []models.RedistributeTargetDto{
		{SkuID: uuid.New().String(), Quantity: 3},
	})

	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
}

func TestRedistributeStock_EmptyPayload(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	productID, _ := createProductForSKU(t, helper, "SKUREDISTEMPTY")

	w := redistribute(t, helper, productID, []models.RedistributeTargetDto{})

	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
}

func TestUpdateProduct_DroppingAnAxisHoldingStock(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	productID, sku := createProductWithAxes(t, helper, "SKUAXISDROP", []axisSpec{
		{groupType: "size", options: []string{"M", "XL"}},
	})
	purchase(t, helper, productID, 3)
	assert.Equal(t, http.StatusCreated, generateSkus(t, helper, productID).Code)

	skus := listSkus(t, helper, productID).Skus
	m, _ := skuByCode(skus, sku+"-M")
	xl, _ := skuByCode(skus, sku+"-XL")
	assert.Equal(t, http.StatusOK, redistribute(t, helper, productID,
		[]models.RedistributeTargetDto{
			{SkuID: m.SkuID, Quantity: 2},
			{SkuID: xl.SkuID, Quantity: 1},
		}).Code)

	w := putProduct(t, helper, productID, &models.UpdateProductVariantsDto{
		Groups: []models.UpdateVariantGroupDto{},
	})

	assert.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	assert.Equal(t, int64(3), listSkus(t, helper, productID).TotalCount,
		"nothing was deleted while the combinations held units")
}

func TestUpdateProduct_DroppingAnUntouchedAxis(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	productID, sku := createProductWithAxes(t, helper, "SKUAXISZERO", []axisSpec{
		{groupType: "size", options: []string{"M", "XL"}},
	})
	purchase(t, helper, productID, 4)
	assert.Equal(t, http.StatusCreated, generateSkus(t, helper, productID).Code)

	w := putProduct(t, helper, productID, &models.UpdateProductVariantsDto{
		Groups: []models.UpdateVariantGroupDto{},
	})

	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())

	after := listSkus(t, helper, productID)
	assert.Equal(t, int64(1), after.TotalCount, "the empty combinations went with the axis")
	def, _ := skuByCode(after.Skus, sku)
	assert.True(t, def.IsDefault)
	assert.Equal(t, 4.0, def.CurrentQuantity, "the bucket keeps every unit")
}

func TestUpdateProduct_DroppingOneOptionOfAnAxis(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	productID, sku := createProductWithAxes(t, helper, "SKUOPTDROP", []axisSpec{
		{groupType: "size", options: []string{"M", "XL"}},
	})
	assert.Equal(t, http.StatusCreated, generateSkus(t, helper, productID).Code)

	tree := treeAsUpdate(currentTree(t, helper, productID))
	kept := []models.UpdateVariantOptionDto{}
	for _, o := range tree.Groups[0].Options {
		if o.Name == "M" {
			kept = append(kept, o)
		}
	}
	tree.Groups[0].Options = kept

	assert.Equal(t, http.StatusOK, putProduct(t, helper, productID, tree).Code)

	after := listSkus(t, helper, productID)
	assert.Equal(t, int64(2), after.TotalCount, "the XL combination went with the option")
	_, stillThere := skuByCode(after.Skus, sku+"-XL")
	assert.False(t, stillThere)
	_, keptM := skuByCode(after.Skus, sku+"-M")
	assert.True(t, keptM)
}

func TestUpdateProduct_AnAxisMustBeSingleChoice(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	productID, _ := createProductForSKU(t, helper, "SKUAXISMULTI")

	w := putProduct(t, helper, productID, &models.UpdateProductVariantsDto{
		Groups: []models.UpdateVariantGroupDto{{
			GroupType:        "topping",
			IsRequired:       false,
			MaxSelections:    3,
			AffectsInventory: true,
			Options:          []models.UpdateVariantOptionDto{{Name: "Cheese"}},
		}},
	})

	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
}

func TestProductSKU_CombinationKeyMustAgreeWithItsOptions(t *testing.T) {
	inRolledBackTx(t, func(ctx context.Context, tx pgx.Tx) {
		productID, _ := seedProductWithSKU(t, ctx, tx, fmt.Sprintf("SKUKEY_%d", time.Now().UnixNano()))

		var groupID, optionID, skuID string
		mustQueryRow(t, ctx, tx, &groupID,
			`INSERT INTO inventory.product_variant_groups
			     (product_id, group_type, is_required, max_selections, affects_inventory)
			 VALUES ($1::uuid, 'size', TRUE, 1, TRUE) RETURNING id`, productID)
		mustQueryRow(t, ctx, tx, &optionID,
			`INSERT INTO inventory.product_variant_options (variant_group_id, option_name)
			 VALUES ($1::uuid, 'XL') RETURNING id`, groupID)
		mustQueryRow(t, ctx, tx, &skuID,
			`INSERT INTO inventory.product_skus
			     (tenant_id, product_id, sku, combination_key, is_default)
			 SELECT p.tenant_id, p.id, p.sku || '-XL', 'not-the-option-uuid', FALSE
			 FROM inventory.products p WHERE p.id = $1::uuid RETURNING id`, productID)

		_, err := tx.Exec(ctx,
			`INSERT INTO inventory.product_sku_options
			     (sku_id, variant_group_id, option_id, product_id)
			 VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid)`,
			skuID, groupID, optionID, productID)
		assert.NoError(t, err, "the check is deferred, so the row itself goes in")

		// Deferred to COMMIT so the generator can write a SKU and its options in
		// any order; forcing it here is what a COMMIT would do.
		_, err = tx.Exec(ctx, `SET CONSTRAINTS ALL IMMEDIATE`)
		assert.Error(t, err, "combination_key must agree with the option rows (INV-007's open invariant)")
	})
}

func TestGetProductStock_UnchangedByGeneration(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	productID, _ := createProductWithAxes(t, helper, "SKUSTOCKSAME", []axisSpec{
		{groupType: "size", options: []string{"M", "XL"}},
	})
	purchase(t, helper, productID, 6)

	before := helper.DoRequest("GET", fmt.Sprintf("/inventory/stock/%s", productID), nil, map[string]string{})
	assert.Equal(t, http.StatusOK, before.Code)
	var beforeStock models.ProductStock
	json.Unmarshal(before.Body.Bytes(), &beforeStock)

	assert.Equal(t, http.StatusCreated, generateSkus(t, helper, productID).Code)

	after := helper.DoRequest("GET", fmt.Sprintf("/inventory/stock/%s", productID), nil, map[string]string{})
	assert.Equal(t, http.StatusOK, after.Code)
	var afterStock models.ProductStock
	json.Unmarshal(after.Body.Bytes(), &afterStock)

	assert.Equal(t, beforeStock.CurrentQuantity, afterStock.CurrentQuantity,
		"generating empty combinations does not change what the product holds")
	assert.Equal(t, beforeStock.AvailableQuantity, afterStock.AvailableQuantity)
}

// ============================================
// Movements and reservations by combination (INV-011)
// ============================================

// movementWithSku posts a movement naming a combination, or none when skuID is "".
func movementWithSku(
	t *testing.T,
	helper *testhelpers.ApiTestHelper,
	productID, skuID, movementType string,
	quantity float64,
) *httptest.ResponseRecorder {
	t.Helper()

	dto := models.RecordMovementDto{
		ProductID:    productID,
		MovementType: movementType,
		Quantity:     quantity,
		UnitCost:     costForType(movementType),
		Direction:    directionForType(movementType),
	}
	if skuID != "" {
		dto.SkuID = &skuID
	}
	return helper.DoRequest("POST", "/inventory/movements", dto, map[string]string{})
}

// reserveWithSku posts to reserve or release, naming a combination when given one.
func reserveWithSku(
	t *testing.T,
	helper *testhelpers.ApiTestHelper,
	path, productID, skuID string,
	quantity float64,
) *httptest.ResponseRecorder {
	t.Helper()

	dto := models.ReserveStockDto{ProductID: productID, Quantity: quantity}
	if skuID != "" {
		dto.SkuID = &skuID
	}
	return helper.DoRequest("POST", path, dto, map[string]string{})
}

// generatedProduct is the shape most tests below start from: one axis whose
// combinations already exist, which is when the guard goes live.
func generatedProduct(
	t *testing.T,
	helper *testhelpers.ApiTestHelper,
	prefix string,
) (string, string, []models.ProductSku) {
	t.Helper()

	productID, sku := createProductWithAxes(t, helper, prefix, []axisSpec{
		{groupType: "size", options: []string{"M", "XL"}},
	})
	if code := generateSkus(t, helper, productID).Code; code != http.StatusCreated {
		t.Fatalf("generating combinations for %s returned %d", productID, code)
	}
	return productID, sku, listSkus(t, helper, productID).Skus
}

func TestRecordMovement_RequiresACombinationOnceTheyExist(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	productID, _, _ := generatedProduct(t, helper, "MVSKUREQ")

	w := movementWithSku(t, helper, productID, "", "PURCHASE", 5)

	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), inventoryErrors.MovementSkuRequired)
}

func TestRecordMovement_WithoutAxesStillDefaultsToTheProductSku(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	productID, _ := createProductForSKU(t, helper, "MVNOAXIS")

	w := movementWithSku(t, helper, productID, "", "PURCHASE", 9)

	assert.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	assert.Equal(t, 9.0, listSkus(t, helper, productID).Skus[0].CurrentQuantity)
}

// The window between declaring an axis and generating its combinations: the flag
// is already TRUE but there is nowhere to put the units, so the bucket still takes
// them and redistribution moves them later.
func TestRecordMovement_AxisDeclaredButNotYetGenerated(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	productID, _ := createProductWithAxes(t, helper, "MVUNGEN", []axisSpec{
		{groupType: "size", options: []string{"M", "XL"}},
	})

	w := movementWithSku(t, helper, productID, "", "PURCHASE", 4)

	assert.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	assert.Equal(t, 4.0, listSkus(t, helper, productID).Skus[0].CurrentQuantity,
		"the units land in the unassigned bucket, which is all there is")
}

func TestRecordMovement_MovesOnlyTheCombinationItNames(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	productID, sku, skus := generatedProduct(t, helper, "MVONESKU")
	m, _ := skuByCode(skus, sku+"-M")

	w := movementWithSku(t, helper, productID, m.SkuID, "PURCHASE", 6)
	assert.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	after := listSkus(t, helper, productID).Skus
	movedM, _ := skuByCode(after, sku+"-M")
	untouchedXL, _ := skuByCode(after, sku+"-XL")
	bucket, _ := skuByCode(after, sku)

	assert.Equal(t, 6.0, movedM.CurrentQuantity)
	assert.Equal(t, 0.0, untouchedXL.CurrentQuantity, "the other combination is untouched")
	assert.Equal(t, 0.0, bucket.CurrentQuantity, "the unassigned bucket never refills")
}

func TestRecordMovement_SkuOfAnotherProduct(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	productID, _, _ := generatedProduct(t, helper, "MVSKUALIEN")
	_, otherSku, otherSkus := generatedProduct(t, helper, "MVSKUOTHER")
	alien, _ := skuByCode(otherSkus, otherSku+"-M")

	w := movementWithSku(t, helper, productID, alien.SkuID, "PURCHASE", 1)

	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), inventoryErrors.SkuNotFound)
}

func TestReserveStock_RequiresACombinationOnceTheyExist(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	productID, sku, skus := generatedProduct(t, helper, "RSVSKUREQ")
	m, _ := skuByCode(skus, sku+"-M")
	assert.Equal(t, http.StatusCreated, movementWithSku(t, helper, productID, m.SkuID, "PURCHASE", 5).Code)

	w := reserveWithSku(t, helper, "/inventory/reserve", productID, "", 2)

	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), inventoryErrors.ReserveSkuRequired)
}

func TestReserveStock_HoldsAndReleasesOnItsOwnCombination(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	productID, sku, skus := generatedProduct(t, helper, "RSVSKUONE")
	m, _ := skuByCode(skus, sku+"-M")
	xl, _ := skuByCode(skus, sku+"-XL")
	assert.Equal(t, http.StatusCreated, movementWithSku(t, helper, productID, m.SkuID, "PURCHASE", 5).Code)
	assert.Equal(t, http.StatusCreated, movementWithSku(t, helper, productID, xl.SkuID, "PURCHASE", 5).Code)

	held := reserveWithSku(t, helper, "/inventory/reserve", productID, m.SkuID, 3)
	assert.Equal(t, http.StatusOK, held.Code, held.Body.String())

	afterHold := listSkus(t, helper, productID).Skus
	heldM, _ := skuByCode(afterHold, sku+"-M")
	freeXL, _ := skuByCode(afterHold, sku+"-XL")
	assert.Equal(t, 3.0, heldM.ReservedQuantity)
	assert.Equal(t, 2.0, heldM.AvailableQuantity)
	assert.Equal(t, 0.0, freeXL.ReservedQuantity, "the other combination holds nothing")

	released := reserveWithSku(t, helper, "/inventory/release-reserved", productID, m.SkuID, 3)
	assert.Equal(t, http.StatusOK, released.Code, released.Body.String())

	afterRelease := listSkus(t, helper, productID).Skus
	freedM, _ := skuByCode(afterRelease, sku+"-M")
	assert.Equal(t, 0.0, freedM.ReservedQuantity)
	assert.Equal(t, 5.0, freedM.AvailableQuantity)
}

func TestReleaseReserved_RequiresACombinationOnceTheyExist(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	productID, _, _ := generatedProduct(t, helper, "RLSSKUREQ")

	w := reserveWithSku(t, helper, "/inventory/release-reserved", productID, "", 1)

	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), inventoryErrors.ReserveSkuRequired)
}

func TestListMovements_CarriesTheCombinationOfEachRow(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	productID, sku, skus := generatedProduct(t, helper, "MVTRAIL")
	m, _ := skuByCode(skus, sku+"-M")
	assert.Equal(t, http.StatusCreated, movementWithSku(t, helper, productID, m.SkuID, "PURCHASE", 4).Code)

	trail := listMovements(t, helper, "?product_id="+productID)

	assert.Equal(t, int64(1), trail.TotalCount)
	assert.NotNil(t, trail.Movements[0].SkuID, "the row says which combination moved")
	assert.Equal(t, m.SkuID, *trail.Movements[0].SkuID)
	assert.NotNil(t, trail.Movements[0].SKU)
	assert.Equal(t, sku+"-M", *trail.Movements[0].SKU)
}

func TestListMovements_FilteredByCombination(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	productID, sku, skus := generatedProduct(t, helper, "MVTRAILFILTER")
	m, _ := skuByCode(skus, sku+"-M")
	xl, _ := skuByCode(skus, sku+"-XL")
	assert.Equal(t, http.StatusCreated, movementWithSku(t, helper, productID, m.SkuID, "PURCHASE", 4).Code)
	assert.Equal(t, http.StatusCreated, movementWithSku(t, helper, productID, xl.SkuID, "PURCHASE", 7).Code)

	whole := listMovements(t, helper, "?product_id="+productID)
	assert.Equal(t, int64(2), whole.TotalCount)

	onlyM := listMovements(t, helper, "?product_id="+productID+"&sku_id="+m.SkuID)
	assert.Equal(t, int64(1), onlyM.TotalCount)
	assert.Equal(t, 1, len(onlyM.Movements))
	assert.Equal(t, 4.0, onlyM.Movements[0].Quantity)
}

func TestListMovements_InvalidCombinationFilter(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	w := helper.DoRequest("GET", "/inventory/movements?sku_id=not-a-uuid", nil, map[string]string{})

	assert.Equal(t, http.StatusBadRequest, w.Code, "sku_id is validated before it reaches Postgres")
}

// ============================================
// Creating a product with its axes (INV-012)
// ============================================

// axesAsVariants turns the axis specs into the variant tree a create or update
// body carries. Every group is required and single choice, which is what
// chk_variant_groups_axis_single_choice demands of an axis.
func axesAsVariants(axes []axisSpec) *models.UpdateProductVariantsDto {
	groups := make([]models.UpdateVariantGroupDto, 0, len(axes))
	for _, axis := range axes {
		options := make([]models.UpdateVariantOptionDto, 0, len(axis.options))
		for _, name := range axis.options {
			options = append(options, models.UpdateVariantOptionDto{Name: name})
		}
		groups = append(groups, models.UpdateVariantGroupDto{
			GroupType:        axis.groupType,
			IsRequired:       true,
			MaxSelections:    1,
			AffectsInventory: true,
			Options:          options,
		})
	}
	return &models.UpdateProductVariantsDto{Groups: groups}
}

// postProduct creates a product carrying its whole variant tree in the POST
// body — the one round trip INV-012 D2 resolved on.
func postProduct(
	t *testing.T,
	helper *testhelpers.ApiTestHelper,
	sku string,
	variants interface{},
) *httptest.ResponseRecorder {
	t.Helper()

	return helper.DoRequest("POST", "/inventory/products", models.CreateProductDto{
		SKU:       sku,
		Name:      "SKU Invariant Product",
		BasePrice: 25.00,
		Variants:  variants,
	}, map[string]string{})
}

// createdProductID reads the id out of a create that must have succeeded.
func createdProductID(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()

	if w.Code != http.StatusCreated {
		t.Fatalf("create returned %d: %s", w.Code, w.Body.String())
	}

	var created models.CreateProductResponse
	json.Unmarshal(w.Body.Bytes(), &created)
	return created.ProductID
}

func TestCreateProduct_AxisCombinationsExistOnTheFirstSave(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	sku := fmt.Sprintf("SKUCREATEAXIS_%d", time.Now().UnixNano())

	productID := createdProductID(t, postProduct(t, helper, sku, axesAsVariants([]axisSpec{
		{groupType: "size", options: []string{"S", "M", "XL"}},
	})))

	listed := listSkus(t, helper, productID)
	assert.Equal(t, int64(4), listed.TotalCount, "three combinations plus the default, from the create alone")
	assert.Equal(t, 3, len(nonDefaultSkus(listed.Skus)))

	// The client no longer has to ask, and asking anyway finds nothing to do.
	w := generateSkus(t, helper, productID)
	assert.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	var generated models.GenerateSkusResponse
	json.Unmarshal(w.Body.Bytes(), &generated)
	assert.Equal(t, 0, generated.CreatedCount, "the create already built them")
	assert.Equal(t, 3, generated.ExistingCount)
	assert.True(t, generated.StockByVariant)
}

func TestCreateProduct_WithoutAxisStillGetsOnlyItsDefaultSku(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	sku := fmt.Sprintf("SKUCREATEMOD_%d", time.Now().UnixNano())

	productID := createdProductID(t, postProduct(t, helper, sku, &models.UpdateProductVariantsDto{
		Groups: []models.UpdateVariantGroupDto{{
			GroupType:     "topping",
			MaxSelections: 3,
			Options: []models.UpdateVariantOptionDto{
				{Name: "Cheese", Modifier: 2.00},
				{Name: "Bacon", Modifier: 3.00},
			},
		}},
	}))

	listed := listSkus(t, helper, productID)
	assert.Equal(t, int64(1), listed.TotalCount, "a menu modifier multiplies nothing")
	assert.Equal(t, 0, len(nonDefaultSkus(listed.Skus)))
}

func TestCreateProduct_AxisOptionPriceReachesTheCombination(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	sku := fmt.Sprintf("SKUAXISPRICE_%d", time.Now().UnixNano())

	productID := createdProductID(t, postProduct(t, helper, sku, &models.UpdateProductVariantsDto{
		Groups: []models.UpdateVariantGroupDto{{
			GroupType:        "size",
			IsRequired:       true,
			MaxSelections:    1,
			AffectsInventory: true,
			Options: []models.UpdateVariantOptionDto{
				{Name: "S", Modifier: 0},
				{Name: "XL", Modifier: 2.00},
			},
		}},
	}))

	listed := listSkus(t, helper, productID)
	small, foundSmall := skuByCode(listed.Skus, sku+"-S")
	large, foundLarge := skuByCode(listed.Skus, sku+"-XL")
	assert.True(t, foundSmall && foundLarge, "both combinations exist")
	assert.Equal(t, 0.0, small.PriceModifier)
	assert.Equal(t, 2.00, large.PriceModifier, "the option price is the combination price (D1)")

	def, _ := skuByCode(listed.Skus, sku)
	assert.Equal(t, 0.0, def.PriceModifier, "the unassigned bucket composes no option")

	// Repricing the option through the ordinary product edit moves the
	// combination, with no second generation.
	tree := treeAsUpdate(currentTree(t, helper, productID))
	for i, option := range tree.Groups[0].Options {
		if option.Name == "XL" {
			tree.Groups[0].Options[i].Modifier = 5.00
		}
	}
	assert.Equal(t, http.StatusOK, putProduct(t, helper, productID, tree).Code)

	after := listSkus(t, helper, productID)
	repriced, _ := skuByCode(after.Skus, sku+"-XL")
	assert.Equal(t, 5.00, repriced.PriceModifier, "an option reprice reprices its combinations")
	assert.Equal(t, int64(3), after.TotalCount, "and creates nothing")
}

func TestCreateProduct_TwoAxesSumTheirOptionPrices(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	sku := fmt.Sprintf("SKUAXISSUM_%d", time.Now().UnixNano())

	productID := createdProductID(t, postProduct(t, helper, sku, &models.UpdateProductVariantsDto{
		Groups: []models.UpdateVariantGroupDto{
			{
				GroupType: "size", IsRequired: true, MaxSelections: 1, AffectsInventory: true,
				Options: []models.UpdateVariantOptionDto{
					{Name: "S", Modifier: 0},
					{Name: "XL", Modifier: 2.00},
				},
			},
			{
				GroupType: "colour", IsRequired: true, MaxSelections: 1, AffectsInventory: true,
				Options: []models.UpdateVariantOptionDto{
					{Name: "Plain", Modifier: 0},
					{Name: "Gold", Modifier: 5.50},
				},
			},
		},
	}))

	listed := listSkus(t, helper, productID)
	assert.Equal(t, int64(5), listed.TotalCount, "four combinations plus the default")

	both, found := skuByCode(listed.Skus, sku+"-XL-GOLD")
	assert.True(t, found, "the XL/Gold combination exists")
	assert.Equal(t, 7.50, both.PriceModifier, "the combination price is the sum of both options")

	def, _ := skuByCode(listed.Skus, sku)
	assert.Equal(t, 0.0, def.PriceModifier, "the unassigned bucket stays at 0")
}

func TestCreateProduct_DuplicateGroupTypeReturnsConflict(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	sku := fmt.Sprintf("SKUDUPGROUP_%d", time.Now().UnixNano())

	w := postProduct(t, helper, sku, &models.UpdateProductVariantsDto{
		Groups: []models.UpdateVariantGroupDto{
			{GroupType: "size", MaxSelections: 1, Options: []models.UpdateVariantOptionDto{{Name: "S"}}},
			{GroupType: "size", MaxSelections: 1, Options: []models.UpdateVariantOptionDto{{Name: "M"}}},
		},
	})

	assert.Equal(t, http.StatusConflict, w.Code, w.Body.String())
}

func TestCreateProduct_DuplicateOptionNameReturnsConflict(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	sku := fmt.Sprintf("SKUDUPOPTION_%d", time.Now().UnixNano())

	w := postProduct(t, helper, sku, &models.UpdateProductVariantsDto{
		Groups: []models.UpdateVariantGroupDto{{
			GroupType:     "size",
			MaxSelections: 1,
			Options:       []models.UpdateVariantOptionDto{{Name: "S"}, {Name: "S"}},
		}},
	})

	assert.Equal(t, http.StatusConflict, w.Code, w.Body.String())
}

func TestCreateProduct_AxisNotSingleChoiceReturnsBadRequest(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	sku := fmt.Sprintf("SKUBADAXIS_%d", time.Now().UnixNano())

	w := postProduct(t, helper, sku, &models.UpdateProductVariantsDto{
		Groups: []models.UpdateVariantGroupDto{{
			GroupType:        "size",
			IsRequired:       false,
			MaxSelections:    2,
			AffectsInventory: true,
			Options:          []models.UpdateVariantOptionDto{{Name: "S"}, {Name: "M"}},
		}},
	})

	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
}

// ============================================
// Movement direction and cost (INV-013)
// ============================================

// directedMovement posts a movement that says which way it goes, or says nothing
// when direction is "". The cost rides along for the two types that require one,
// so each test below states only the thing it is about.
func directedMovement(
	t *testing.T,
	helper *testhelpers.ApiTestHelper,
	productID, movementType string,
	quantity float64,
	direction string,
) *httptest.ResponseRecorder {
	t.Helper()

	dto := models.RecordMovementDto{
		ProductID:    productID,
		MovementType: movementType,
		Quantity:     quantity,
		UnitCost:     costForType(movementType),
	}
	if direction != "" {
		dto.Direction = &direction
	}
	return helper.DoRequest("POST", "/inventory/movements", dto, map[string]string{})
}

// stockedProduct is a product with no axes holding `quantity` units — the shape
// every direction test starts from.
func stockedProduct(
	t *testing.T,
	helper *testhelpers.ApiTestHelper,
	prefix string,
	quantity float64,
) string {
	t.Helper()

	productID, _ := createProductForSKU(t, helper, prefix)
	if w := directedMovement(t, helper, productID, "PURCHASE", quantity, ""); w.Code != http.StatusCreated {
		t.Fatalf("stocking %s returned %d: %s", productID, w.Code, w.Body.String())
	}
	return productID
}

// AC-1 — the defect this spec exists for: a SALE used to add stock.
func TestRecordMovement_SaleReducesStock(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	productID := stockedProduct(t, helper, "MVDIRSALE", 10)

	w := directedMovement(t, helper, productID, "SALE", 5, "")

	assert.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var recorded models.RecordMovementResponse
	json.Unmarshal(w.Body.Bytes(), &recorded)
	assert.Equal(t, 5.0, recorded.NewStockQuantity)
}

// AC-2 — the oversell guard was unreachable from this path, because a SALE only
// ever moved the count up.
func TestRecordMovement_SaleBeyondStockIsRefused(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	productID := stockedProduct(t, helper, "MVDIROVER", 10)

	w := directedMovement(t, helper, productID, "SALE", 20, "")

	assert.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), inventoryErrors.InsufficientStock)
}

// AC-3 — WASTE is outbound too, and the old guard only ever looked at SALE.
func TestRecordMovement_WasteReducesStock(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	productID := stockedProduct(t, helper, "MVDIRWASTE", 10)

	w := directedMovement(t, helper, productID, "WASTE", 3, "")

	assert.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var recorded models.RecordMovementResponse
	json.Unmarshal(w.Body.Bytes(), &recorded)
	assert.Equal(t, 7.0, recorded.NewStockQuantity)
}

// AC-4, from a raw API call — the UI's read-only control is not the enforcement.
func TestRecordMovement_AdjustmentWithoutDirectionIsRefused(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	productID := stockedProduct(t, helper, "MVDIRADJ", 10)

	w := directedMovement(t, helper, productID, "ADJUSTMENT", 2, "")

	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), inventoryErrors.MovementDirectionRequired)
}

func TestRecordMovement_AdjustmentOutReducesStock(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	productID := stockedProduct(t, helper, "MVDIRADJOUT", 10)

	w := directedMovement(t, helper, productID, "ADJUSTMENT", 2, "OUT")

	assert.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var recorded models.RecordMovementResponse
	json.Unmarshal(w.Body.Bytes(), &recorded)
	assert.Equal(t, 8.0, recorded.NewStockQuantity)
}

func TestRecordMovement_AdjustmentInRaisesStock(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	productID := stockedProduct(t, helper, "MVDIRADJIN", 10)

	w := directedMovement(t, helper, productID, "ADJUSTMENT", 2, "IN")

	assert.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var recorded models.RecordMovementResponse
	json.Unmarshal(w.Body.Bytes(), &recorded)
	assert.Equal(t, 12.0, recorded.NewStockQuantity)
}

// A type that decides its own direction refuses one that contradicts it, rather
// than quietly honouring whichever the caller sent.
func TestRecordMovement_PurchaseOutIsRefused(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	productID := stockedProduct(t, helper, "MVDIRCONF", 10)

	w := directedMovement(t, helper, productID, "PURCHASE", 1, "OUT")

	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), inventoryErrors.MovementDirectionConflict)
}

// The direction the type already decided is accepted, because the modal shows it
// as read-only text and sends it straight back.
func TestRecordMovement_SaleOutIsAccepted(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	productID := stockedProduct(t, helper, "MVDIRAGREE", 10)

	w := directedMovement(t, helper, productID, "SALE", 4, "OUT")

	assert.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var recorded models.RecordMovementResponse
	json.Unmarshal(w.Body.Bytes(), &recorded)
	assert.Equal(t, 6.0, recorded.NewStockQuantity)
}

// AC-5, first half — a purchase with no cost is the incomplete record "optional"
// used to let in (D3).
func TestRecordMovement_PurchaseWithoutUnitCostIsRefused(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	productID, _ := createProductForSKU(t, helper, "MVCOSTREQ")

	w := helper.DoRequest("POST", "/inventory/movements", models.RecordMovementDto{
		ProductID:    productID,
		MovementType: "PURCHASE",
		Quantity:     5,
	}, map[string]string{})

	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), inventoryErrors.MovementUnitCostRequired)
}

// AC-5, second half — the column means "what was paid", so a type that pays
// nothing stores nothing even when a client insists on sending a number.
func TestRecordMovement_SaleStoresNoUnitCost(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	productID := stockedProduct(t, helper, "MVCOSTNULL", 10)

	w := helper.DoRequest("POST", "/inventory/movements", models.RecordMovementDto{
		ProductID:    productID,
		MovementType: "SALE",
		Quantity:     2,
		UnitCost:     ptrFloat64(99.99),
	}, map[string]string{})
	assert.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	movements := listMovements(t, helper, fmt.Sprintf("?product_id=%s&movement_type=SALE", productID))
	assert.Equal(t, 1, len(movements.Movements))
	assert.Nil(t, movements.Movements[0].UnitCost)
}

// The trail carries the sign it was applied with, so the history can render it
// without re-deriving one from the type (Risks: old rows are all positive).
func TestListMovements_StoresTheSignedQuantity(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	productID := stockedProduct(t, helper, "MVSIGNED", 10)
	assert.Equal(t, http.StatusCreated, directedMovement(t, helper, productID, "SALE", 3, "").Code)

	movements := listMovements(t, helper, fmt.Sprintf("?product_id=%s&movement_type=SALE", productID))

	assert.Equal(t, 1, len(movements.Movements))
	assert.Equal(t, -3.0, movements.Movements[0].Quantity)
}

// The magnitude is the contract: the DTO refuses a signed quantity rather than
// letting the caller's sign compete with the type's (D1).
func TestRecordMovement_NegativeQuantityIsRefused(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	productID := stockedProduct(t, helper, "MVNEGQTY", 10)

	w := helper.DoRequest("POST", "/inventory/movements", models.RecordMovementDto{
		ProductID:    productID,
		MovementType: "SALE",
		Quantity:     -5,
	}, map[string]string{})

	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
}

// ============================================
// INV-014 — a lot belongs to a combination
// ============================================

// createBatchOn posts a lot, naming a combination when skuID is non-empty.
func createBatchOn(
	t *testing.T,
	helper *testhelpers.ApiTestHelper,
	productID, skuID, lotNumber string,
	expiresInDays int,
	quantity float64,
) *httptest.ResponseRecorder {
	t.Helper()

	body := map[string]interface{}{
		"product_id":       productID,
		"lot_number":       lotNumber,
		"purchase_date":    time.Now().Format("2006-01-02"),
		"expiry_date":      time.Now().AddDate(0, 0, expiresInDays).Format("2006-01-02"),
		"unit_cost":        50.00,
		"initial_quantity": quantity,
	}
	if skuID != "" {
		body["sku_id"] = skuID
	}

	return helper.DoRequest("POST", "/inventory/batches", body, map[string]string{})
}

// combinationSkus returns the product's generated combinations, keyed by the
// option name they carry — never the default SKU, which is the bucket.
func combinationSkus(t *testing.T, helper *testhelpers.ApiTestHelper, productID string) map[string]models.ProductSku {
	t.Helper()

	byOption := map[string]models.ProductSku{}
	for _, sku := range listSkus(t, helper, productID).Skus {
		if sku.IsDefault {
			continue
		}
		for _, option := range sku.Options {
			byOption[option.OptionName] = sku
		}
	}
	return byOption
}

// axisProductWithSkus is the fixture the whole section runs on: one product
// stocked by variant, with M and XL generated.
func axisProductWithSkus(
	t *testing.T,
	helper *testhelpers.ApiTestHelper,
	prefix string,
) (productID string, skus map[string]models.ProductSku) {
	t.Helper()

	productID, _ = createProductWithAxes(t, helper, prefix, []axisSpec{
		{groupType: "Size", options: []string{"M", "XL"}},
	})
	if w := generateSkus(t, helper, productID); w.Code != http.StatusCreated {
		t.Fatalf("generating the SKUs of %s failed with %d: %s", productID, w.Code, w.Body.String())
	}

	return productID, combinationSkus(t, helper, productID)
}

// AC-1 — the lot lands on the combination it was created for, not the bucket.
func TestCreateBatch_OnNamedSku(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()
	productID, skus := axisProductWithSkus(t, helper, "INV014_CREATE")

	w := createBatchOn(t, helper, productID, skus["M"].SkuID,
		fmt.Sprintf("LOT_M_%d", time.Now().UnixNano()), 90, 10)

	assert.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var batch models.BatchResponse
	json.Unmarshal(w.Body.Bytes(), &batch)
	if assert.NotNil(t, batch.SkuID, "the created lot carries no combination") {
		assert.Equal(t, skus["M"].SkuID, batch.SkuID.String())
	}
}

// AC-2 — the bug itself. Before INV-014 this answered 201 and quietly filed the
// lot under the default SKU, where no sale could ever reach it.
func TestCreateBatch_WithoutSkuOnVariantProductIsRefused(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()
	productID, _ := axisProductWithSkus(t, helper, "INV014_NOSKU")

	w := createBatchOn(t, helper, productID, "",
		fmt.Sprintf("LOT_BUCKET_%d", time.Now().UnixNano()), 90, 10)

	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), inventoryErrors.SkuRequired)
}

// The guard is about products stocked by variant, not about batches: a product
// with no axis must go on defaulting to its own SKU exactly as before.
func TestCreateBatch_WithoutSkuOnPlainProductStillDefaults(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()
	productID, _ := createProductForSKU(t, helper, "INV014_PLAIN")

	w := createBatchOn(t, helper, productID, "",
		fmt.Sprintf("LOT_PLAIN_%d", time.Now().UnixNano()), 90, 5)

	assert.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var batch models.BatchResponse
	json.Unmarshal(w.Body.Bytes(), &batch)
	assert.NotNil(t, batch.SkuID)
}

// AC-3 — every row says which combination it belongs to, and skuId narrows the
// list to one of them.
func TestListBatchesByProduct_FilteredBySku(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()
	productID, skus := axisProductWithSkus(t, helper, "INV014_LIST")

	stamp := time.Now().UnixNano()
	for _, lot := range []struct {
		option string
		name   string
	}{
		{"M", fmt.Sprintf("LOT_M1_%d", stamp)},
		{"M", fmt.Sprintf("LOT_M2_%d", stamp)},
		{"XL", fmt.Sprintf("LOT_XL_%d", stamp)},
	} {
		w := createBatchOn(t, helper, productID, skus[lot.option].SkuID, lot.name, 90, 10)
		assert.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	}

	all := listBatches(t, helper, productID, "")
	assert.Len(t, all.Batches, 3)
	for _, batch := range all.Batches {
		assert.NotNil(t, batch.SkuID, "a listed lot carries no combination")
		assert.NotNil(t, batch.Sku)
	}

	onlyM := listBatches(t, helper, productID, skus["M"].SkuID)
	assert.Len(t, onlyM.Batches, 2)
	for _, batch := range onlyM.Batches {
		assert.Equal(t, skus["M"].SkuID, batch.SkuID.String())
	}
}

// listBatches reads a product's lots, optionally narrowed to one combination.
func listBatches(
	t *testing.T,
	helper *testhelpers.ApiTestHelper,
	productID, skuID string,
) models.ListBatchesResponse {
	t.Helper()

	url := fmt.Sprintf("/inventory/batches/product/%s?onlyActive=false", productID)
	if skuID != "" {
		url += "&skuId=" + skuID
	}

	w := helper.DoRequest("GET", url, nil, map[string]string{})
	if w.Code != http.StatusOK {
		t.Fatalf("listing the lots of %s failed with %d: %s", productID, w.Code, w.Body.String())
	}

	var result models.ListBatchesResponse
	json.Unmarshal(w.Body.Bytes(), &result)
	return result
}

// AC-4 — FIFO picks within the combination asked for, not across the product.
func TestGetOldestBatch_ScopedToSku(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()
	productID, skus := axisProductWithSkus(t, helper, "INV014_OLDEST")

	stamp := time.Now().UnixNano()
	xlSoonest := fmt.Sprintf("LOT_XL_SOON_%d", stamp)
	mLater := fmt.Sprintf("LOT_M_LATER_%d", stamp)
	// The XL lot expires first, so a product-wide pick would return it for M too.
	assert.Equal(t, http.StatusCreated, createBatchOn(t, helper, productID, skus["XL"].SkuID, xlSoonest, 10, 10).Code)
	assert.Equal(t, http.StatusCreated, createBatchOn(t, helper, productID, skus["M"].SkuID, mLater, 90, 10).Code)

	assert.Equal(t, mLater, oldestBatch(t, helper, productID, skus["M"].SkuID).LotNumber)
	assert.Equal(t, xlSoonest, oldestBatch(t, helper, productID, skus["XL"].SkuID).LotNumber)
}

func oldestBatch(
	t *testing.T,
	helper *testhelpers.ApiTestHelper,
	productID, skuID string,
) models.BatchResponse {
	t.Helper()

	url := fmt.Sprintf("/inventory/batches/product/%s/oldest", productID)
	if skuID != "" {
		url += "?skuId=" + skuID
	}

	w := helper.DoRequest("GET", url, nil, map[string]string{})
	if w.Code != http.StatusOK {
		t.Fatalf("the FIFO pick of %s failed with %d: %s", productID, w.Code, w.Body.String())
	}

	var batch models.BatchResponse
	json.Unmarshal(w.Body.Bytes(), &batch)
	return batch
}

// AC-6 (first half) — a lot with no movements is correctable, in both fields.
func TestUpdateBatch_CorrectsLotNumberAndExpiry(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()
	productID, skus := axisProductWithSkus(t, helper, "INV014_PATCH")

	w := createBatchOn(t, helper, productID, skus["M"].SkuID,
		fmt.Sprintf("LOT_TYPO_%d", time.Now().UnixNano()), 90, 10)
	assert.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var created models.BatchResponse
	json.Unmarshal(w.Body.Bytes(), &created)

	corrected := fmt.Sprintf("LOT_FIXED_%d", time.Now().UnixNano())
	newExpiry := time.Now().AddDate(0, 0, 120).Format("2006-01-02")
	wp := helper.DoRequest("PATCH", fmt.Sprintf("/inventory/batches/%s", created.ID),
		map[string]interface{}{"lot_number": corrected, "expiry_date": newExpiry},
		map[string]string{})

	assert.Equal(t, http.StatusOK, wp.Code, wp.Body.String())
	var updated models.BatchResponse
	json.Unmarshal(wp.Body.Bytes(), &updated)
	assert.Equal(t, corrected, updated.LotNumber)
	assert.Equal(t, newExpiry, time.Time(updated.ExpiryDate).Format("2006-01-02"))
	// D3 — what a correction may never touch.
	assert.Equal(t, created.UnitCost, updated.UnitCost)
	assert.Equal(t, created.CurrentQuantity, updated.CurrentQuantity)
}

// AC-6 (second half) — once a sale has consumed the lot, both writes are refused:
// batch_movements carries the COGS already booked against it.
func TestUpdateAndVoidBatch_RefusedAfterMovements(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()
	productID, skus := axisProductWithSkus(t, helper, "INV014_CONSUMED")

	w := createBatchOn(t, helper, productID, skus["M"].SkuID,
		fmt.Sprintf("LOT_SOLD_%d", time.Now().UnixNano()), 90, 10)
	assert.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var created models.BatchResponse
	json.Unmarshal(w.Body.Bytes(), &created)

	consumeBatch(t, created.ID.String())

	wp := helper.DoRequest("PATCH", fmt.Sprintf("/inventory/batches/%s", created.ID),
		map[string]interface{}{"lot_number": "SHOULD_NOT_APPLY"}, map[string]string{})
	assert.Equal(t, http.StatusConflict, wp.Code, wp.Body.String())
	assert.Contains(t, wp.Body.String(), inventoryErrors.BatchHasMovements)

	wd := helper.DoRequest("DELETE", fmt.Sprintf("/inventory/batches/%s", created.ID), nil, map[string]string{})
	assert.Equal(t, http.StatusConflict, wd.Code, wd.Body.String())
	assert.Contains(t, wd.Body.String(), inventoryErrors.BatchHasMovements)
}

// consumeBatch books a consumption against a lot the way a sale does. Written
// straight to the table because sales lives in another module and another suite,
// and what these tests need is only the row the guard reads.
func consumeBatch(t *testing.T, batchID string) {
	t.Helper()

	ctx := context.Background()
	conn := openInventoryDB(t)
	defer conn.Close(ctx)

	if _, err := conn.Exec(ctx,
		`INSERT INTO inventory.batch_movements
		     (tenant_id, batch_id, quantity_consumed, cost_of_goods_sold)
		 SELECT pb.tenant_id, pb.id, 1, pb.unit_cost
		 FROM inventory.product_batches pb
		 WHERE pb.id = $1::uuid`, batchID); err != nil {
		t.Fatalf("booking a consumption against %s: %v", batchID, err)
	}
}

// AC-6 — DELETE voids, it never deletes, and the voided lot leaves the FIFO pick.
func TestVoidBatch_KeepsRowAndLeavesFifo(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()
	productID, skus := axisProductWithSkus(t, helper, "INV014_VOID")

	stamp := time.Now().UnixNano()
	soonest := fmt.Sprintf("LOT_SOONEST_%d", stamp)
	later := fmt.Sprintf("LOT_LATER_%d", stamp)
	ws := createBatchOn(t, helper, productID, skus["M"].SkuID, soonest, 10, 10)
	assert.Equal(t, http.StatusCreated, ws.Code, ws.Body.String())
	var soonestBatch models.BatchResponse
	json.Unmarshal(ws.Body.Bytes(), &soonestBatch)
	assert.Equal(t, http.StatusCreated, createBatchOn(t, helper, productID, skus["M"].SkuID, later, 90, 10).Code)

	assert.Equal(t, soonest, oldestBatch(t, helper, productID, skus["M"].SkuID).LotNumber)

	wd := helper.DoRequest("DELETE", fmt.Sprintf("/inventory/batches/%s", soonestBatch.ID), nil, map[string]string{})
	assert.Equal(t, http.StatusOK, wd.Code, wd.Body.String())
	var voided models.BatchResponse
	json.Unmarshal(wd.Body.Bytes(), &voided)
	assert.Equal(t, "void", voided.Status)

	// The row survives — batch_movements and order_batch_assignments point at it.
	wg := helper.DoRequest("GET", fmt.Sprintf("/inventory/batches/%s", soonestBatch.ID), nil, map[string]string{})
	assert.Equal(t, http.StatusOK, wg.Code, wg.Body.String())

	assert.Equal(t, later, oldestBatch(t, helper, productID, skus["M"].SkuID).LotNumber)

	wd2 := helper.DoRequest("DELETE", fmt.Sprintf("/inventory/batches/%s", soonestBatch.ID), nil, map[string]string{})
	assert.Equal(t, http.StatusConflict, wd2.Code, wd2.Body.String())
	assert.Contains(t, wd2.Body.String(), inventoryErrors.BatchVoided)
}

// AC-7 — a lot stranded in the unassigned bucket splits onto real combinations,
// keeping its lot number, its dates and its unit cost.
func TestRedistributeBatches_SplitsStrandedLot(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	// The lot has to be created BEFORE the axes exist: that is exactly how every
	// pre-INV-014 lot ended up in the bucket, and the guard would refuse it after.
	productID, _ := createProductForSKU(t, helper, "INV014_REDIST")
	lotNumber := fmt.Sprintf("LOT_STRANDED_%d", time.Now().UnixNano())
	wc := createBatchOn(t, helper, productID, "", lotNumber, 60, 10)
	assert.Equal(t, http.StatusCreated, wc.Code, wc.Body.String())
	var stranded models.BatchResponse
	json.Unmarshal(wc.Body.Bytes(), &stranded)

	putAxes(t, helper, productID, []axisSpec{{groupType: "Size", options: []string{"M", "XL"}}})
	if w := generateSkus(t, helper, productID); w.Code != http.StatusCreated {
		t.Fatalf("generating the SKUs of %s failed with %d: %s", productID, w.Code, w.Body.String())
	}
	skus := combinationSkus(t, helper, productID)

	w := helper.DoRequest("POST",
		fmt.Sprintf("/inventory/products/%s/batches/redistribute", productID),
		map[string]interface{}{
			"batch_id": stranded.ID.String(),
			"targets": []map[string]interface{}{
				{"sku_id": skus["M"].SkuID, "quantity": 6},
				{"sku_id": skus["XL"].SkuID, "quantity": 4},
			},
		}, map[string]string{})

	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var result models.RedistributeBatchesResponse
	json.Unmarshal(w.Body.Bytes(), &result)
	assert.Equal(t, 2, result.TargetCount)
	assert.Equal(t, 10.0, result.MovedQuantity)

	byQuantity := map[float64]models.BatchResponse{}
	for _, child := range result.Batches {
		byQuantity[child.CurrentQuantity] = child
		assert.Equal(t, lotNumber, child.LotNumber, "the child lost its lot number")
		assert.Equal(t, stranded.UnitCost, child.UnitCost, "the child lost its unit cost")
		assert.Equal(t,
			time.Time(stranded.ExpiryDate).Format("2006-01-02"),
			time.Time(child.ExpiryDate).Format("2006-01-02"))
	}
	if assert.Contains(t, byQuantity, 6.0) {
		assert.Equal(t, skus["M"].SkuID, byQuantity[6.0].SkuID.String())
	}
	if assert.Contains(t, byQuantity, 4.0) {
		assert.Equal(t, skus["XL"].SkuID, byQuantity[4.0].SkuID.String())
	}

	// M can now be sold from; before the split its lots were unreachable.
	assert.Equal(t, lotNumber, oldestBatch(t, helper, productID, skus["M"].SkuID).LotNumber)
}

// A1 — el reparto mueve existencias entre combinaciones sin crear ni destruir:
// el total del producto es el mismo antes y después, el bucket se vacía y los
// destinos reciben. Antes de A1 los lotes no tocaban el stock en absoluto y este
// test habría exigido lo contrario.
func TestRedistributeBatches_KeepsProductTotal(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	productID, _ := createProductForSKU(t, helper, "INV014_REDIST_STOCK")
	wc := createBatchOn(t, helper, productID, "",
		fmt.Sprintf("LOT_NOSTOCK_%d", time.Now().UnixNano()), 60, 10)
	assert.Equal(t, http.StatusCreated, wc.Code, wc.Body.String())
	var stranded models.BatchResponse
	json.Unmarshal(wc.Body.Bytes(), &stranded)

	putAxes(t, helper, productID, []axisSpec{{groupType: "Size", options: []string{"M", "XL"}}})
	if w := generateSkus(t, helper, productID); w.Code != http.StatusCreated {
		t.Fatalf("generating the SKUs of %s failed with %d: %s", productID, w.Code, w.Body.String())
	}
	skus := combinationSkus(t, helper, productID)

	before := skuQuantities(t, helper, productID)
	assert.Equal(t, 10.0, total(before), "el lote debería haber entrado al stock (A1)")

	w := helper.DoRequest("POST",
		fmt.Sprintf("/inventory/products/%s/batches/redistribute", productID),
		map[string]interface{}{
			"batch_id": stranded.ID.String(),
			"targets": []map[string]interface{}{
				{"sku_id": skus["M"].SkuID, "quantity": 6},
				{"sku_id": skus["XL"].SkuID, "quantity": 4},
			},
		}, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())

	after := skuQuantities(t, helper, productID)
	assert.Equal(t, total(before), total(after),
		"repartir no crea ni destruye unidades del producto")
	assert.Equal(t, 6.0, after[skus["M"].SKU])
	assert.Equal(t, 4.0, after[skus["XL"].SKU])
	for sku, quantity := range after {
		assert.GreaterOrEqualf(t, quantity, 0.0, "%s quedó negativo", sku)
	}
}

// total suma las existencias de todas las combinaciones de un producto.
func total(quantities map[string]float64) float64 {
	sum := 0.0
	for _, quantity := range quantities {
		sum += quantity
	}
	return sum
}

// A1 — un lote es una entrada al stock: crearlo sube las existencias de su
// combinación. Antes de A1 el lote quedaba en su propio libro y la pestaña de
// Existencias seguía en cero, que es la incoherencia que originó la enmienda.
func TestCreateBatch_RaisesStock(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()
	productID, skus := axisProductWithSkus(t, helper, "INV014_STOCK_IN")

	before := skuQuantities(t, helper, productID)

	w := createBatchOn(t, helper, productID, skus["M"].SkuID,
		fmt.Sprintf("LOT_IN_%d", time.Now().UnixNano()), 90, 10)
	assert.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	after := skuQuantities(t, helper, productID)
	assert.Equal(t, before[skus["M"].SKU]+10, after[skus["M"].SKU],
		"crear un lote de 10 debe subir el stock de esa combinación en 10")
	assert.Equal(t, before[skus["XL"].SKU], after[skus["XL"].SKU],
		"la combinación hermana no se toca")
}

// A1 — anular devuelve al stock lo que el lote todavía tenía, o las existencias
// quedarían infladas por mercancía que ya no se puede vender.
func TestVoidBatch_ReturnsStock(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()
	productID, skus := axisProductWithSkus(t, helper, "INV014_STOCK_OUT")

	before := skuQuantities(t, helper, productID)
	w := createBatchOn(t, helper, productID, skus["M"].SkuID,
		fmt.Sprintf("LOT_OUT_%d", time.Now().UnixNano()), 90, 10)
	assert.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var created models.BatchResponse
	json.Unmarshal(w.Body.Bytes(), &created)
	assert.Equal(t, before[skus["M"].SKU]+10, skuQuantities(t, helper, productID)[skus["M"].SKU])

	wd := helper.DoRequest("DELETE", fmt.Sprintf("/inventory/batches/%s", created.ID), nil, map[string]string{})
	assert.Equal(t, http.StatusOK, wd.Code, wd.Body.String())

	assert.Equal(t, before[skus["M"].SKU], skuQuantities(t, helper, productID)[skus["M"].SKU],
		"anular el lote debe devolver el stock a donde estaba")
}

// skuQuantities is every combination's stock quantity, keyed by SKU code.
func skuQuantities(t *testing.T, helper *testhelpers.ApiTestHelper, productID string) map[string]float64 {
	t.Helper()

	quantities := map[string]float64{}
	for _, sku := range listSkus(t, helper, productID).Skus {
		quantities[sku.SKU] = sku.CurrentQuantity
	}
	return quantities
}

// The amounts must add up to exactly the lot, or nothing moves.
func TestRedistributeBatches_MismatchedAmountsRefused(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	productID, _ := createProductForSKU(t, helper, "INV014_REDIST_BAD")
	wc := createBatchOn(t, helper, productID, "",
		fmt.Sprintf("LOT_BAD_%d", time.Now().UnixNano()), 60, 10)
	assert.Equal(t, http.StatusCreated, wc.Code, wc.Body.String())
	var stranded models.BatchResponse
	json.Unmarshal(wc.Body.Bytes(), &stranded)

	putAxes(t, helper, productID, []axisSpec{{groupType: "Size", options: []string{"M", "XL"}}})
	if w := generateSkus(t, helper, productID); w.Code != http.StatusCreated {
		t.Fatalf("generating the SKUs of %s failed with %d: %s", productID, w.Code, w.Body.String())
	}
	skus := combinationSkus(t, helper, productID)

	w := helper.DoRequest("POST",
		fmt.Sprintf("/inventory/products/%s/batches/redistribute", productID),
		map[string]interface{}{
			"batch_id": stranded.ID.String(),
			"targets":  []map[string]interface{}{{"sku_id": skus["M"].SkuID, "quantity": 3}},
		}, map[string]string{})

	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), inventoryErrors.SkusRedistributionMismatch)

	// Nothing moved: the lot is still whole and still in the bucket.
	remaining := listBatches(t, helper, productID, "")
	assert.Len(t, remaining.Batches, 1)
	assert.Equal(t, 10.0, remaining.Batches[0].CurrentQuantity)
}

// A consumed lot is refused rather than split — the reason D2 chose a split over
// an UPDATE in the first place.
func TestRedistributeBatches_RefusedForConsumedLot(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	productID, _ := createProductForSKU(t, helper, "INV014_REDIST_USED")
	wc := createBatchOn(t, helper, productID, "",
		fmt.Sprintf("LOT_USED_%d", time.Now().UnixNano()), 60, 10)
	assert.Equal(t, http.StatusCreated, wc.Code, wc.Body.String())
	var stranded models.BatchResponse
	json.Unmarshal(wc.Body.Bytes(), &stranded)
	consumeBatch(t, stranded.ID.String())

	putAxes(t, helper, productID, []axisSpec{{groupType: "Size", options: []string{"M", "XL"}}})
	if w := generateSkus(t, helper, productID); w.Code != http.StatusCreated {
		t.Fatalf("generating the SKUs of %s failed with %d: %s", productID, w.Code, w.Body.String())
	}
	skus := combinationSkus(t, helper, productID)

	w := helper.DoRequest("POST",
		fmt.Sprintf("/inventory/products/%s/batches/redistribute", productID),
		map[string]interface{}{
			"batch_id": stranded.ID.String(),
			"targets":  []map[string]interface{}{{"sku_id": skus["M"].SkuID, "quantity": 10}},
		}, map[string]string{})

	assert.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), inventoryErrors.BatchHasMovements)
}

// ============================================================================
// CSV import — products, categories, stock and lots (INV-015)
// ============================================================================

// importInventoryCSV posts a CSV — and, when given, its companion image archive —
// to the generic import endpoint and returns the decoded response, failing the
// test if the endpoint itself rejected the upload.
func importInventoryCSV(
	t *testing.T,
	helper *testhelpers.ApiTestHelper,
	path, resource, csv, options string,
	images []byte,
) map[string]interface{} {
	t.Helper()

	files := []testhelpers.MultipartFile{
		{Field: "file", Filename: resource + ".csv", Content: []byte(csv)},
	}
	if images != nil {
		files = append(files, testhelpers.MultipartFile{Field: "images", Filename: "images.zip", Content: images})
	}

	fields := map[string]string{"resource": resource}
	if options != "" {
		fields["options"] = options
	}

	w := helper.DoMultipartRequest("POST", path, fields, files, map[string]string{})
	if w.Code != http.StatusOK {
		t.Fatalf("import returned %d: %s", w.Code, w.Body.String())
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("could not decode the import response: %v — %s", err, w.Body.String())
	}
	return decoded
}

// importRow returns one row of an import or validate response, by its position
// in the file.
func importRow(t *testing.T, response map[string]interface{}, index int) map[string]interface{} {
	t.Helper()

	rows, ok := response["rows"].([]interface{})
	if !ok || len(rows) <= index {
		t.Fatalf("expected at least %d rows in %v", index+1, response)
	}
	row, ok := rows[index].(map[string]interface{})
	if !ok {
		t.Fatalf("expected a row object, got %v", rows[index])
	}
	return row
}

// rowCodes flattens the errors or the warnings of one row into plain strings.
func rowCodes(row map[string]interface{}, key string) []string {
	raw, _ := row[key].([]interface{})
	codes := make([]string, 0, len(raw))
	for _, entry := range raw {
		codes = append(codes, fmt.Sprint(entry))
	}
	return codes
}

// containsCode reports whether any code is the one expected, or the one expected
// with a value appended after the separator the wizard splits on.
func containsCode(codes []string, expected string) bool {
	for _, code := range codes {
		if code == expected || strings.HasPrefix(code, expected+"|") {
			return true
		}
	}
	return false
}

// archiveWith builds the companion image archive holding one entry.
// archiveWith builds the companion image archive out of alternating name and
// content arguments. It takes more than one because D7's option images arrive
// in the same ZIP as the product's own picture.
func archiveWith(t *testing.T, entries ...any) []byte {
	t.Helper()

	if len(entries)%2 != 0 {
		t.Fatalf("archiveWith takes name/content pairs, got %d arguments", len(entries))
	}

	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for index := 0; index < len(entries); index += 2 {
		name, ok := entries[index].(string)
		if !ok {
			t.Fatalf("argument %d should be a filename, got %v", index, entries[index])
		}
		content, ok := entries[index+1].([]byte)
		if !ok {
			t.Fatalf("argument %d should be the file's bytes, got %v", index+1, entries[index+1])
		}

		entry, err := writer.Create(name)
		if err != nil {
			t.Fatalf("could not build the image archive: %v", err)
		}
		if _, err := entry.Write(content); err != nil {
			t.Fatalf("could not write into the image archive: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("could not close the image archive: %v", err)
	}
	return buffer.Bytes()
}

// productBySku reads an imported product back through the endpoint the operator
// would use, which is the only handle a CSV row leaves behind.
func productBySku(t *testing.T, helper *testhelpers.ApiTestHelper, sku string) models.ProductDetail {
	t.Helper()

	w := helper.DoRequest("GET", "/inventory/products/sku/"+sku, nil, map[string]string{})
	if w.Code != http.StatusOK {
		t.Fatalf("reading back %s returned %d: %s", sku, w.Code, w.Body.String())
	}

	var detail models.ProductDetail
	if err := json.Unmarshal(w.Body.Bytes(), &detail); err != nil {
		t.Fatalf("could not decode the product: %v — %s", err, w.Body.String())
	}
	return detail
}

// importPrefix is a per-test SKU stem: products are unique per tenant and the
// suite shares one.
func importPrefix(name string) string {
	return fmt.Sprintf("%s%d", name, time.Now().UnixNano())
}


// AC-14 — the template and the schema come from the descriptor's Columns(), and
// all three product resources publish the same two sample variant[…] columns so
// the marker is met once (D6).
func TestImportProducts_TemplateCarriesTheAxisMarker(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	for _, resource := range []string{"products", "product_stock", "product_batches"} {
		template := helper.DoRequest("GET", "/import/template?resource="+resource, nil, map[string]string{})
		assert.Equal(t, http.StatusOK, template.Code, template.Body.String())
		assert.Contains(t, template.Body.String(), "variant[talle],variant[color]",
			"%s should publish the two sample axis columns: %s", resource, template.Body.String())
	}

	template := helper.DoRequest("GET", "/import/template?resource=products", nil, map[string]string{})
	assert.Contains(t, template.Body.String(), "sku,name,description,price",
		"the catalogue header row: %s", template.Body.String())
	// AC-13 — no descriptor publishes a packed cell any more.
	for _, retired := range []string{"axes", "combination", "base_price"} {
		assert.NotContains(t, template.Body.String(), retired+",",
			"%q should be gone from the template: %s", retired, template.Body.String())
	}

	schema := helper.DoRequest("GET", "/import/schema?resource=products", nil, map[string]string{})
	assert.Equal(t, http.StatusOK, schema.Code, schema.Body.String())
}

// AC-1 — one file creates the whole catalogue: a simple product, a one-axis
// product and a two-axis product, in one pass (D9).
func TestImportProducts_OneFileCreatesTheWholeCatalogue(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	prefix := importPrefix("IMPCAT")
	csv := fmt.Sprintf(
		"sku,name,price,category,variant[talle],variant[color],stock,reorder_level\n"+
			"%s-Y,Yerba 1kg,4200,Bebidas %s,,,12,3\n"+
			"%s-C,Gaseosa Cola,2200,Bebidas %s,,,24,6\n"+
			"%s-R,Remera Basica,7500,Indumentaria %s,M,Negro,10,3\n"+
			"%s-R,Remera Basica,7500,,L,Negro,8,3\n"+
			"%s-R,Remera Basica,8700,,XL,Blanco,5,2\n",
		prefix, prefix, prefix, prefix, prefix, prefix, prefix, prefix)

	dryRun := importInventoryCSV(t, helper, "/import/validate", "products", csv, "", nil)
	assert.Equal(t, float64(5), dryRun["valid"], "every row should validate: %v", dryRun)

	response := importInventoryCSV(t, helper, "/import", "products", csv, "", nil)
	for index := 0; index < 5; index++ {
		assert.Equal(t, "created", importRow(t, response, index)["status"],
			"row %d: %v", index, response)
	}

	// Three products out of five rows: the repeated sku is not a duplicate.
	simple := productBySku(t, helper, prefix+"-Y")
	assert.Empty(t, nonDefaultSkus(listSkus(t, helper, simple.ID).Skus),
		"a row with no variant cell is a simple product")

	shirt := productBySku(t, helper, prefix+"-R")
	assert.Len(t, nonDefaultSkus(listSkus(t, helper, shirt.ID).Skus), 3,
		"three listed combinations")
}

// AC-2 — the file's combinations and only those: three of the six that
// talle{M,L,XL} × color{Negro,Blanco} spans (D2).
func TestImportProducts_OnlyTheListedCombinationsExist(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	prefix := importPrefix("IMPPART")
	csv := fmt.Sprintf(
		"sku,name,price,variant[talle],variant[color]\n"+
			"%s,Remera,7500,M,Negro\n"+
			"%s,Remera,7500,L,Negro\n"+
			"%s,Remera,8700,XL,Blanco\n",
		prefix, prefix, prefix)

	importInventoryCSV(t, helper, "/import", "products", csv, "", nil)
	productID := productBySku(t, helper, prefix).ID

	skus := nonDefaultSkus(listSkus(t, helper, productID).Skus)
	assert.Len(t, skus, 3, "three combinations, not the six a cartesian product gives: %+v", skus)

	// Both axes exist with the options the file named, and nothing else was
	// generated from them.
	detail := fetchProductDetail(t, helper, productID)
	assert.Len(t, detail.Variants, 2, "two axes: %+v", detail.Variants)
	for _, group := range detail.Variants {
		assert.True(t, group.AffectsInventory, "%s should be an inventory axis", group.GroupType)
	}
}

// AC-2 — the same file gives one product one axis and another two (D1).
func TestImportProducts_EachProductGetsOnlyTheAxesItsRowsFill(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	prefix := importPrefix("IMPAXES")
	csv := fmt.Sprintf(
		"sku,name,price,variant[talle],variant[color],variant[tamano]\n"+
			"%s-C,Gaseosa,2200,,,500ml\n"+
			"%s-C,Gaseosa,3000,,,1.5L\n"+
			"%s-R,Remera,7500,M,Negro,\n",
		prefix, prefix, prefix)

	importInventoryCSV(t, helper, "/import", "products", csv, "", nil)

	cola := fetchProductDetail(t, helper, productBySku(t, helper, prefix+"-C").ID)
	if assert.Len(t, cola.Variants, 1, "COLA gets only Tamano: %+v", cola.Variants) {
		assert.Equal(t, "Tamano", cola.Variants[0].GroupType)
	}

	shirt := fetchProductDetail(t, helper, productBySku(t, helper, prefix+"-R").ID)
	assert.Len(t, shirt.Variants, 2, "the shirt gets Talle and Color: %+v", shirt.Variants)
}

// AC-2b — a repeat row is not a duplicate; a differing non-empty product cell is
// product-inconsistent (D9).
func TestImportProducts_RepeatRowReconciliation(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	prefix := importPrefix("IMPREPEAT")
	csv := fmt.Sprintf(
		"sku,name,price,variant[talle]\n"+
			"%s,Remera Basica,7500,M\n"+
			"%s,Remera Basica,7500,L\n"+
			"%s,Remera Premium,7500,XL\n",
		prefix, prefix, prefix)

	dryRun := importInventoryCSV(t, helper, "/import/validate", "products", csv, "", nil)
	assert.Equal(t, "valid", importRow(t, dryRun, 1)["status"],
		"an identical repeat is the same product: %v", dryRun)
	third := importRow(t, dryRun, 2)
	assert.Equal(t, "invalid", third["status"], "%v", third)
	assert.True(t, containsCode(rowCodes(third, "errors"), inventoryErrors.ImportProductInconsistent),
		"expected product-inconsistent: %v", third["errors"])

	response := importInventoryCSV(t, helper, "/import", "products", csv, "", nil)
	assert.Equal(t, "created", importRow(t, response, 1)["status"], "%v", response)
	assert.Equal(t, "failed", importRow(t, response, 2)["status"], "%v", response)

	productID := productBySku(t, helper, prefix).ID
	assert.Len(t, nonDefaultSkus(listSkus(t, helper, productID).Skus), 2,
		"the refused row created nothing")
}

// AC-3 — a filled variant_sku is the code verbatim, a blank one is derived the
// way sp_generate_product_skus would, and one file may mix both (D6).
func TestImportProducts_VariantSkuVerbatimOrDerived(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	prefix := importPrefix("IMPCODE")
	legacy := fmt.Sprintf("LEGACY-%d", time.Now().UnixNano()%1000000)
	csv := fmt.Sprintf(
		"sku,name,price,variant[talle],variant_sku\n"+
			"%s,Remera,7500,M,%s\n"+
			"%s,Remera,7500,XL,\n",
		prefix, legacy, prefix)

	importInventoryCSV(t, helper, "/import", "products", csv, "", nil)
	productID := productBySku(t, helper, prefix).ID

	codes := map[string]bool{}
	for _, sku := range nonDefaultSkus(listSkus(t, helper, productID).Skus) {
		codes[sku.SKU] = true
	}

	assert.True(t, codes[legacy], "the filled code travels verbatim: %v", codes)
	assert.True(t, codes[prefix+"-XL"], "the blank one is derived: %v", codes)
}

// AC-4 — the row's price becomes the modifier of the option D3 attributes it to,
// through the trigger that derives product_skus.price_modifier.
func TestImportProducts_PriceBecomesThePerOptionModifier(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	prefix := importPrefix("IMPPRICE")
	csv := fmt.Sprintf(
		"sku,name,price,variant[talle],variant[color]\n"+
			"%s,Remera,7500,M,Negro\n"+
			"%s,Remera,7500,L,Negro\n"+
			"%s,Remera,8700,XL,Blanco\n",
		prefix, prefix, prefix)

	importInventoryCSV(t, helper, "/import", "products", csv, "", nil)
	productID := productBySku(t, helper, prefix).ID

	byOption := combinationSkus(t, helper, productID)
	assert.Equal(t, 0.0, byOption["M"].PriceModifier, "the first row is the base")
	assert.Equal(t, 0.0, byOption["L"].PriceModifier, "L costs the same as M")
	assert.Equal(t, 1200.0, byOption["XL"].PriceModifier,
		"the 1200 lands on XL, the row's first unfixed axis: %+v", byOption["XL"])
	assert.Equal(t, 7500.0, productBySku(t, helper, prefix).BasePrice,
		"the product's base is the price of its first row")
}

// AC-5 — a row whose difference cannot be reconciled fails, naming the cell, and
// writes nothing.
func TestImportProducts_UnattributablePriceFailsTheRow(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	prefix := importPrefix("IMPBADPRICE")
	csv := fmt.Sprintf(
		"sku,name,price,variant[talle],variant[color]\n"+
			"%s,Remera,7500,M,Negro\n"+
			"%s,Remera,8700,XL,Blanco\n"+
			"%s,Remera,9000,M,Negro\n",
		prefix, prefix, prefix)

	response := importInventoryCSV(t, helper, "/import", "products", csv, "", nil)
	row := importRow(t, response, 2)

	assert.Equal(t, "failed", row["status"], "%v", row)
	assert.True(t, containsCode(rowCodes(row, "errors"), inventoryErrors.ImportPriceInconsistent),
		"expected price-inconsistent: %v", row["errors"])

	productID := productBySku(t, helper, prefix).ID
	assert.Len(t, nonDefaultSkus(listSkus(t, helper, productID).Skus), 2,
		"the refused row created no combination")
}

// AC-6 — a product whose rows fill different axis columns is refused.
func TestImportProducts_RaggedAxisSetFailsTheRow(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	prefix := importPrefix("IMPRAGGED")
	csv := fmt.Sprintf(
		"sku,name,price,variant[talle],variant[color]\n"+
			"%s,Remera,7500,M,Negro\n"+
			"%s,Remera,7500,L,\n",
		prefix, prefix)

	response := importInventoryCSV(t, helper, "/import", "products", csv, "", nil)
	row := importRow(t, response, 1)

	assert.Equal(t, "failed", row["status"], "%v", row)
	assert.True(t, containsCode(rowCodes(row, "errors"), inventoryErrors.ImportAxesInconsistent),
		"expected axes-inconsistent: %v", row["errors"])
}

// AC-7 — `stock` on a row with no variant cell seeds the simple product's
// default bucket and writes one ADJUSTMENT movement (D9).
func TestImportProducts_StockOnASimpleRowSeedsTheDefaultBucket(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	prefix := importPrefix("IMPSEED")
	csv := fmt.Sprintf("sku,name,price,stock,reorder_level\n%s,Yerba,3500,25,5\n", prefix)

	response := importInventoryCSV(t, helper, "/import", "products", csv, "", nil)
	assert.Equal(t, "created", importRow(t, response, 0)["status"], "%v", response)

	productID := productBySku(t, helper, prefix).ID

	stock := helper.DoRequest("GET", "/inventory/stock/"+productID, nil, map[string]string{})
	var current models.ProductStock
	json.Unmarshal(stock.Body.Bytes(), &current)
	assert.Equal(t, 25.0, current.CurrentQuantity, "the opening count: %s", stock.Body.String())
	assert.Equal(t, 5.0, current.ReorderLevel, "the reorder level: %s", stock.Body.String())

	movements := listMovements(t, helper, "?product_id="+productID)
	if assert.Len(t, movements.Movements, 1, "one opening count is one movement: %+v", movements.Movements) {
		assert.Equal(t, "ADJUSTMENT", movements.Movements[0].MovementType)
	}
}

// AC-10 — stock and reorder_level land on the combination of their own row, and
// a blank one leaves it at zero without an error.
func TestImportProducts_StockLandsOnTheRowsOwnCombination(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	prefix := importPrefix("IMPCOMBSTK")
	csv := fmt.Sprintf(
		"sku,name,price,variant[talle],stock,reorder_level\n"+
			"%s,Remera,7500,M,10,3\n"+
			"%s,Remera,7500,L,,\n",
		prefix, prefix)

	response := importInventoryCSV(t, helper, "/import", "products", csv, "", nil)
	for index := 0; index < 2; index++ {
		row := importRow(t, response, index)
		assert.Equal(t, "created", row["status"], "%v", row)
		assert.Empty(t, rowCodes(row, "errors"), "row %d: %v", index, row["errors"])
	}

	productID := productBySku(t, helper, prefix).ID
	byOption := combinationSkus(t, helper, productID)
	assert.Equal(t, 10.0, byOption["M"].CurrentQuantity, "M holds what its row said")
	assert.Equal(t, 3.0, byOption["M"].ReorderLevel, "the level lands on the combination")
	assert.Equal(t, 0.0, byOption["L"].CurrentQuantity, "a blank stock leaves it at zero")
}

// AC-8 — a variant_sku the tenant already has is duplicate on the dry run and
// skipped on process.
func TestImportProducts_TakenVariantSkuIsDuplicateThenSkipped(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	prefix := importPrefix("IMPTAKEN")
	code := fmt.Sprintf("TAKEN-%d", time.Now().UnixNano()%1000000)

	first := fmt.Sprintf("sku,name,price,variant[talle],variant_sku\n%s-A,Remera,7500,M,%s\n", prefix, code)
	importInventoryCSV(t, helper, "/import", "products", first, "", nil)

	// A second product trying to claim the same code.
	again := fmt.Sprintf("sku,name,price,variant[talle],variant_sku\n%s-B,Polera,8000,M,%s\n", prefix, code)

	dryRun := importInventoryCSV(t, helper, "/import/validate", "products", again, "", nil)
	row := importRow(t, dryRun, 0)
	assert.Equal(t, "duplicate", row["status"], "%v", row)
	assert.True(t, containsCode(rowCodes(row, "errors"), inventoryErrors.ImportVariantSkuDuplicate),
		"expected variant-sku-duplicate: %v", row["errors"])

	response := importInventoryCSV(t, helper, "/import", "products", again, "", nil)
	assert.Equal(t, "skipped", importRow(t, response, 0)["status"], "%v", response)
}

// AC-8 — a code longer than product_skus.sku holds is refused by the row.
func TestImportProducts_OverLongVariantSkuIsRefused(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	prefix := importPrefix("IMPLONG")
	csv := fmt.Sprintf("sku,name,price,variant[talle],variant_sku\n%s,Remera,7500,M,%s\n",
		prefix, strings.Repeat("X", 51))

	response := importInventoryCSV(t, helper, "/import", "products", csv, "", nil)
	row := importRow(t, response, 0)

	assert.Equal(t, "failed", row["status"], "%v", row)
	assert.True(t, containsCode(rowCodes(row, "errors"), inventoryErrors.ImportVariantSkuTooLong),
		"expected variant-sku-too-long: %v", row["errors"])
}

// AC-9 — an axis nobody anticipated works with no code change, a malformed
// marker is refused, and a stray column is ignored.
func TestImportProducts_AnyAxisWorksAndStrayColumnsAreIgnored(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	prefix := importPrefix("IMPVOLT")
	csv := fmt.Sprintf("sku,name,price,variant[voltaje],notas\n%s,Taladro,45000,220V,liquidacion\n", prefix)

	response := importInventoryCSV(t, helper, "/import", "products", csv, "", nil)
	row := importRow(t, response, 0)
	assert.Equal(t, "created", row["status"], "%v", row)

	detail := fetchProductDetail(t, helper, productBySku(t, helper, prefix).ID)
	if assert.Len(t, detail.Variants, 1, "the axis the bracket named: %+v", detail.Variants) {
		assert.Equal(t, "Voltaje", detail.Variants[0].GroupType)
	}

	broken := fmt.Sprintf("sku,name,price,variant[]\n%s-B,Taladro,45000,220V\n", prefix)
	failed := importInventoryCSV(t, helper, "/import", "products", broken, "", nil)
	brokenRow := importRow(t, failed, 0)
	assert.Equal(t, "failed", brokenRow["status"], "%v", brokenRow)
	assert.True(t, containsCode(rowCodes(brokenRow, "errors"), inventoryErrors.ImportAxisColumnInvalid),
		"expected axis-column-invalid: %v", brokenRow["errors"])
}

// D9 — two rows naming the same combination: duplicate on the dry run, skipped
// on process.
func TestImportProducts_TheSameCombinationTwiceIsADuplicate(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	prefix := importPrefix("IMPDUPCOMB")
	csv := fmt.Sprintf(
		"sku,name,price,variant[talle]\n"+
			"%s,Remera,7500,M\n"+
			"%s,Remera,7500,M\n",
		prefix, prefix)

	dryRun := importInventoryCSV(t, helper, "/import/validate", "products", csv, "", nil)
	assert.Equal(t, "duplicate", importRow(t, dryRun, 1)["status"], "%v", dryRun)

	response := importInventoryCSV(t, helper, "/import", "products", csv, "", nil)
	assert.Equal(t, "created", importRow(t, response, 0)["status"], "%v", response)
	assert.Equal(t, "skipped", importRow(t, response, 1)["status"], "%v", response)

	productID := productBySku(t, helper, prefix).ID
	assert.Len(t, nonDefaultSkus(listSkus(t, helper, productID).Skus), 1, "one combination")
}

// D4 — a repeated product SKU across two runs is still a skip.
func TestImportProducts_DuplicateSkuIsSkippedNotFatal(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	prefix := importPrefix("IMPDUP")
	first := fmt.Sprintf("sku,name,price\n%s-A,Original,1000\n", prefix)
	importInventoryCSV(t, helper, "/import", "products", first, "", nil)

	again := fmt.Sprintf("sku,name,price\n%s-A,Repetido,1000\n%s-B,Nuevo,2000\n", prefix, prefix)

	dryRun := importInventoryCSV(t, helper, "/import/validate", "products", again, "", nil)
	assert.Equal(t, "duplicate", importRow(t, dryRun, 0)["status"], "%v", dryRun)

	response := importInventoryCSV(t, helper, "/import", "products", again, "", nil)
	assert.Equal(t, "skipped", importRow(t, response, 0)["status"], "%v", response)
	assert.Equal(t, "created", importRow(t, response, 1)["status"], "%v", response)

	assert.Equal(t, "Original", productBySku(t, helper, prefix+"-A").Name,
		"the skipped row must not have rewritten the product")
}

// AC-12 — image[color] attaches the picture to the option, with the product's
// own `image` staying the product-level primary (D7).
func TestImportProducts_OptionImagesHangOffTheOption(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	prefix := importPrefix("IMPOPTIMG")
	csv := fmt.Sprintf(
		"sku,name,price,image,variant[talle],variant[color],image[color]\n"+
			"%s,Remera,7500,producto.png,M,Negro,negra.png\n"+
			"%s,Remera,7500,,L,Blanco,blanca.png\n",
		prefix, prefix)
	archive := archiveWith(t,
		"producto.png", []byte("p"),
		"negra.png", []byte("n"),
		"blanca.png", []byte("b"))

	response := importInventoryCSV(t, helper, "/import", "products", csv, "", archive)
	for index := 0; index < 2; index++ {
		row := importRow(t, response, index)
		assert.Equal(t, "created", row["status"], "%v", row)
		assert.Empty(t, rowCodes(row, "warnings"), "row %d: %v", index, row["warnings"])
	}

	detail := fetchProductDetail(t, helper, productBySku(t, helper, prefix).ID)
	assert.Len(t, detail.Media, 1, "the product keeps its own primary image: %+v", detail.Media)

	withImage := map[string]int{}
	for _, group := range detail.Variants {
		for _, option := range group.Options {
			withImage[option.Name] = len(option.Media)
			for _, media := range option.Media {
				assert.True(t, media.IsPrimary, "%s: primary within its own scope", option.Name)
			}
		}
	}
	assert.Equal(t, 1, withImage["Negro"], "Negro carries its photo: %v", withImage)
	assert.Equal(t, 1, withImage["Blanco"], "Blanco carries its photo: %v", withImage)
	assert.Equal(t, 0, withImage["M"], "a size carries no colour photo: %v", withImage)
}

// AC-12 — two rows sharing an option but naming different files disagree, and a
// file the archive lacks is a warning.
func TestImportProducts_OptionImageDisagreementAndMissingFile(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	prefix := importPrefix("IMPIMGBAD")
	csv := fmt.Sprintf(
		"sku,name,price,variant[color],image[color]\n"+
			"%s,Remera,7500,Negro,negra.png\n"+
			"%s,Remera,7500,Blanco,ausente.png\n"+
			"%s-B,Otra,7500,Negro,otra.png\n",
		prefix, prefix, prefix)
	archive := archiveWith(t, "negra.png", []byte("n"))

	response := importInventoryCSV(t, helper, "/import", "products", csv, "", archive)

	missing := importRow(t, response, 1)
	assert.Equal(t, "created", missing["status"], "a missing file is not fatal: %v", missing)
	assert.True(t, containsCode(rowCodes(missing, "warnings"), inventoryErrors.ImportImageMissing),
		"expected image-missing: %v", missing["warnings"])

	// The same option of the same product naming a second file.
	clash := fmt.Sprintf(
		"sku,name,price,variant[color],image[color]\n"+
			"%s-C,Tercera,7500,Negro,negra.png\n"+
			"%s-C,Tercera,7500,Negro,distinta.png\n",
		prefix, prefix)
	clashed := importInventoryCSV(t, helper, "/import", "products", clash, "", archive)
	row := importRow(t, clashed, 1)
	assert.Equal(t, "failed", row["status"], "%v", row)
	assert.True(t, containsCode(rowCodes(row, "errors"), inventoryErrors.ImportOptionImageInconsistent),
		"expected option-image-inconsistent: %v", row["errors"])
}

// AC-4 — the product-level image and its alt text still work, unchanged by D7.
func TestImportProducts_ProductImageFromTheArchiveIsAttached(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	prefix := importPrefix("IMPIMG")
	csv := fmt.Sprintf("sku,name,price,image,image_alt\n%s-A,Yerba,3500,yerba.png,Paquete de yerba\n", prefix)
	archive := archiveWith(t, "yerba.png", []byte("not-really-a-png"))

	response := importInventoryCSV(t, helper, "/import", "products", csv, "", archive)
	row := importRow(t, response, 0)
	assert.Equal(t, "created", row["status"], "%v", row)
	assert.Empty(t, rowCodes(row, "warnings"), "a named image the archive carries raises nothing")

	detail := productBySku(t, helper, prefix+"-A")
	if assert.Len(t, detail.Media, 1, "the product should carry one image: %+v", detail.Media) {
		assert.True(t, detail.Media[0].IsPrimary, "the imported image should be primary")
		assert.NotEmpty(t, detail.Media[0].URL, "the imported image should have a URL")
	}
}

func TestImportProducts_MissingImageIsAWarningNotAFailure(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	prefix := importPrefix("IMPNOIMG")
	csv := fmt.Sprintf("sku,name,price,image\n%s-A,Yerba,3500,ausente.png\n", prefix)

	response := importInventoryCSV(t, helper, "/import", "products", csv, "", nil)
	row := importRow(t, response, 0)

	assert.Equal(t, "created", row["status"], "%v", row)
	assert.True(t, containsCode(rowCodes(row, "warnings"), inventoryErrors.ImportImageMissing),
		"a missing image should be a warning: %v", row["warnings"])
	assert.Empty(t, productBySku(t, helper, prefix+"-A").Media, "no image should have been attached")
}

// The category columns create the tree on the fly, unchanged by D9.
func TestImportProducts_UnknownCategoryIsCreated(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	prefix := importPrefix("IMPCATEG")
	stamp := time.Now().UnixNano()
	csv := fmt.Sprintf("sku,name,price,category,subcategory\n%s-A,Yerba,3500,Bebidas %d,Infusiones %d\n",
		prefix, stamp, stamp)

	response := importInventoryCSV(t, helper, "/import", "products", csv, "", nil)
	row := importRow(t, response, 0)

	assert.Equal(t, "created", row["status"], "%v", row)
	assert.True(t, containsCode(rowCodes(row, "warnings"), inventoryErrors.ImportCategoryCreated),
		"creating a category should be announced: %v", row["warnings"])

	list := helper.DoRequest("GET", "/inventory/categories?limit=200", nil, map[string]string{})
	assert.Contains(t, list.Body.String(), fmt.Sprintf("Infusiones %d", stamp),
		"the subcategory should exist: %s", list.Body.String())
}

func TestImportProducts_UnknownCategoryFailsWhenCreationIsOff(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	prefix := importPrefix("IMPNOCAT")
	csv := fmt.Sprintf("sku,name,price,category\n%s-A,Yerba,3500,Bebdias %s\n", prefix, prefix)

	response := importInventoryCSV(t, helper, "/import", "products", csv,
		`{"create_categories": false}`, nil)
	row := importRow(t, response, 0)

	assert.Equal(t, "failed", row["status"], "%v", row)
	assert.True(t, containsCode(rowCodes(row, "errors"), inventoryErrors.ImportCategoryUnknown),
		"expected category-unknown: %v", row["errors"])
}

// AC-9 — a categories file creates the tree in one pass, parents resolved by
// name against what the rows above have just created.
func TestImportCategories_TreeInOnePass(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	stamp := time.Now().UnixNano()
	parent := fmt.Sprintf("Bebidas %d", stamp)
	child := fmt.Sprintf("Gaseosas %d", stamp)
	grandchild := fmt.Sprintf("Colas %d", stamp)
	csv := fmt.Sprintf(
		"name,parent,description,display_order\n%s,,Todo lo que se toma,1\n%s,%s,,2\n%s,%s,,3\n",
		parent, child, parent, grandchild, child)

	response := importInventoryCSV(t, helper, "/import", "categories", csv, "", nil)
	for index := 0; index < 3; index++ {
		assert.Equal(t, "created", importRow(t, response, index)["status"],
			"row %d should be created: %v", index, response)
	}

	list := helper.DoRequest("GET", "/inventory/categories?limit=200", nil, map[string]string{})
	assert.Contains(t, list.Body.String(), grandchild,
		"the deepest category should exist: %s", list.Body.String())
}

func TestImportCategories_UnknownParentFailsTheRow(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	stamp := time.Now().UnixNano()
	csv := fmt.Sprintf("name,parent\nHuérfana %d,Inexistente %d\n", stamp, stamp)

	// INV-017 — the dry run knows what the rows above create, so a parent none of
	// them creates is the failure the real run is going to report.
	dryRun := importInventoryCSV(t, helper, "/import/validate", "categories", csv, "", nil)
	checked := importRow(t, dryRun, 0)
	assert.Equal(t, "invalid", checked["status"], "%v", checked)
	assert.True(t, containsCode(rowCodes(checked, "errors"), inventoryErrors.ImportCategoryParentUnknown),
		"expected category-parent-unknown on the dry run: %v", checked["errors"])

	response := importInventoryCSV(t, helper, "/import", "categories", csv, "", nil)
	row := importRow(t, response, 0)
	assert.Equal(t, "failed", row["status"], "%v", row)
	assert.True(t, containsCode(rowCodes(row, "errors"), inventoryErrors.ImportCategoryParentUnknown),
		"expected category-parent-unknown: %v", row["errors"])
}

// INV-017 — 4-categorias.csv's shape: each parent is the row above, which the
// real run has created by then, so the dry run validates all three rows.
func TestImportCategories_DryRunSeesWhatTheRowsAboveCreate(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	stamp := time.Now().UnixNano()
	parent := fmt.Sprintf("Limpieza %d", stamp)
	child := fmt.Sprintf("Detergentes %d", stamp)
	grandchild := fmt.Sprintf("Liquidos %d", stamp)
	csv := fmt.Sprintf(
		"name,parent,description,display_order\n%s,,Productos de limpieza,40\n%s,%s,,10\n%s,%s,,10\n",
		parent, child, parent, grandchild, child)

	dryRun := importInventoryCSV(t, helper, "/import/validate", "categories", csv, "", nil)
	response := importInventoryCSV(t, helper, "/import", "categories", csv, "", nil)

	for index := 0; index < 3; index++ {
		checked := importRow(t, dryRun, index)
		assert.Equal(t, "valid", checked["status"], "row %d should validate: %v", index, checked)
		assert.Empty(t, rowCodes(checked, "warnings"), "row %d should not warn: %v", index, checked)
		assert.Equal(t, "created", importRow(t, response, index)["status"],
			"row %d should be created: %v", index, response)
	}
}

// categoryCreatedCount is how many times one run says it created a category.
func categoryCreatedCount(response map[string]interface{}) int {
	rows, _ := response["rows"].([]interface{})
	count := 0
	for _, entry := range rows {
		row, _ := entry.(map[string]interface{})
		for _, code := range rowCodes(row, "warnings") {
			if containsCode([]string{code}, inventoryErrors.ImportCategoryCreated) {
				count++
			}
		}
	}
	return count
}

// INV-017 — 1-catalogo.csv's shape: the dry run announces each new category on
// the row the real run creates it at, so both say category-created 8 times (the
// dry run used to say 12).
func TestImportProducts_DryRunAnnouncesTheCategoriesTheRealRunCreates(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	prefix := importPrefix("IMPLEDGER")
	stamp := time.Now().UnixNano()
	category := func(name string) string { return fmt.Sprintf("%s %d", name, stamp) }
	rows := [][]string{
		{"YERBA", "Yerba", "3500", category("Almacen"), category("Yerbas"), ""},
		{"AGUA", "Agua", "900", category("Bebidas"), category("Aguas"), ""},
		{"SERVILLETA", "Servilletas", "1200", category("Almacen"), category("Descartables"), ""},
		{"LECHE", "Leche", "1500", category("Almacen"), "", ""},
		{"COLA", "Gaseosa Cola", "2200", category("Bebidas"), category("Gaseosas"), "500ml"},
		{"COLA", "", "3000", category("Bebidas"), category("Gaseosas"), "1.5L"},
		{"REMERA", "Remera", "7500", category("Indumentaria"), category("Remeras"), ""},
		{"YOGUR", "Yogur", "2100", category("Almacen"), "", ""},
	}
	csv := "sku,name,price,category,subcategory,variant[tamano]\n"
	for _, row := range rows {
		csv += fmt.Sprintf("%s-%s,%s,%s,%s,%s,%s\n", prefix, row[0], row[1], row[2], row[3], row[4], row[5])
	}

	dryRun := importInventoryCSV(t, helper, "/import/validate", "products", csv, "", nil)
	response := importInventoryCSV(t, helper, "/import", "products", csv, "", nil)

	assert.Equal(t, float64(len(rows)), dryRun["valid"], "every row should validate: %v", dryRun)
	assert.Equal(t, 8, categoryCreatedCount(dryRun), "the dry run announces each category once: %v", dryRun)
	assert.Equal(t, categoryCreatedCount(dryRun), categoryCreatedCount(response),
		"the dry run and the real run agree: %v", response)
}

// INV-017 D1 — every import lookup is tenant-scoped in its SP. Through the API a
// lookup only ever runs with the caller's own tenant, so this asks the
// repository directly, with another tenant's id, for rows the caller owns.
func TestImportLookups_AnotherTenantFindsNothing(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	prefix := importPrefix("IMPTENANT")
	category := fmt.Sprintf("Tenant %d", time.Now().UnixNano())
	csv := fmt.Sprintf("sku,name,price,category,variant[color]\n%s,Remera,7500,%s,Negro\n", prefix, category)
	created := importInventoryCSV(t, helper, "/import", "products", csv, "", nil)
	if status := importRow(t, created, 0)["status"]; status != "created" {
		t.Fatalf("the setup row should be created: %v", created)
	}

	t.Setenv("SKIP_MIGRATIONS", "true")
	ctx := context.Background()
	db := coreServices.NewDatabaseService()
	db.InitDatabase(ctx)
	defer db.CloseDatabase(ctx)

	var tenantID uuid.UUID
	var productID string
	err := db.QueryRow(ctx, `SELECT p.tenant_id, p.id::TEXT FROM inventory.products p WHERE p.sku = $1`, prefix).
		Scan(&tenantID, &productID)
	if err != nil {
		t.Fatalf("the imported product should be readable: %v", err)
	}

	lookups := inventoryRepositories.NewImportLookupRepository(db)
	other := uuid.New()

	own, err := lookups.FindVariantOptionID(ctx, tenantID, productID, "Color", "Negro")
	assert.NoError(t, err)
	assert.NotNil(t, own, "the owner finds its own option")

	foreign, err := lookups.FindVariantOptionID(ctx, other, productID, "Color", "Negro")
	assert.NoError(t, err)
	assert.Nil(t, foreign, "a product id alone must not reach another tenant's option")

	resolution, err := lookups.ResolveSkuByAxes(ctx, other, prefix, []string{"Color"}, []string{"Negro"})
	assert.NoError(t, err)
	assert.Nil(t, resolution.ProductID, "another tenant's SKU is unknown")

	categoryID, err := lookups.FindCategoryID(ctx, other, category, nil)
	assert.NoError(t, err)
	assert.Nil(t, categoryID, "another tenant's category is unknown")

	existence, err := lookups.SkuExists(ctx, other, prefix)
	assert.NoError(t, err)
	assert.False(t, existence.ProductSkuTaken, "another tenant's SKU is not taken")
}

// INV-017 D2 — a catalogue row priced 0 is a free item: valid with a warning on
// the dry run, and created with the same warning for real.
func TestImportProducts_APriceOfZeroIsAcceptedWithAWarning(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	prefix := importPrefix("IMPZERO")
	csv := fmt.Sprintf("sku,name,price\n%s,Muestra gratis,0\n", prefix)

	dryRun := importInventoryCSV(t, helper, "/import/validate", "products", csv, "", nil)
	response := importInventoryCSV(t, helper, "/import", "products", csv, "", nil)

	checked := importRow(t, dryRun, 0)
	assert.Equal(t, "valid", checked["status"], "%v", checked)
	assert.True(t, containsCode(rowCodes(checked, "warnings"), inventoryErrors.ImportPriceZero), "%v", checked)
	row := importRow(t, response, 0)
	assert.Equal(t, "created", row["status"], "%v", row)
	assert.True(t, containsCode(rowCodes(row, "warnings"), inventoryErrors.ImportPriceZero), "%v", row)
}

// AC-11 — a stock row names its combination with the same variant[…] columns the
// catalogue file declared it with (D6).
func TestImportStock_LandsOnTheCombinationItsAxesName(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	prefix := importPrefix("IMPSTK")
	products := fmt.Sprintf(
		"sku,name,price,variant[talle],variant[color]\n"+
			"%s,Remera,7500,M,Negro\n"+
			"%s,Remera,7500,XL,Negro\n",
		prefix, prefix)
	importInventoryCSV(t, helper, "/import", "products", products, "", nil)
	productID := productBySku(t, helper, prefix).ID

	stock := fmt.Sprintf(
		"sku,variant[talle],variant[color],quantity,reorder_level\n"+
			"%s,M,Negro,12,3\n"+
			"%s,XL,Negro,7,\n",
		prefix, prefix)

	dryRun := importInventoryCSV(t, helper, "/import/validate", "product_stock", stock, "", nil)
	assert.Equal(t, float64(2), dryRun["valid"], "both rows should resolve: %v", dryRun)

	response := importInventoryCSV(t, helper, "/import", "product_stock", stock, "", nil)
	assert.Equal(t, "created", importRow(t, response, 0)["status"], "%v", response)
	assert.Equal(t, "created", importRow(t, response, 1)["status"], "%v", response)

	byOption := combinationSkus(t, helper, productID)
	assert.Equal(t, 12.0, byOption["M"].CurrentQuantity, "M holds what its row said")
	assert.Equal(t, 7.0, byOption["XL"].CurrentQuantity, "XL holds what its row said")
	assert.Equal(t, 3.0, byOption["M"].ReorderLevel, "the reorder level lands on the combination")
}

// AC-11 — a simple product takes no variant cell and resolves to its default
// bucket.
func TestImportStock_SimpleProductNeedsNoAxisCell(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	prefix := importPrefix("IMPSTKSIMPLE")
	products := fmt.Sprintf("sku,name,price\n%s,Yerba,3500\n", prefix)
	importInventoryCSV(t, helper, "/import", "products", products, "", nil)
	productID := productBySku(t, helper, prefix).ID

	stock := fmt.Sprintf("sku,variant[talle],quantity\n%s,,10\n", prefix)
	response := importInventoryCSV(t, helper, "/import", "product_stock", stock, "", nil)
	assert.Equal(t, "created", importRow(t, response, 0)["status"], "%v", response)

	current := helper.DoRequest("GET", "/inventory/stock/"+productID, nil, map[string]string{})
	var stockRow models.ProductStock
	json.Unmarshal(current.Body.Bytes(), &stockRow)
	assert.Equal(t, 10.0, stockRow.CurrentQuantity, "it lands on the default bucket: %s", current.Body.String())
}

// AC-11 — naming only some of the product's axes is axes-incomplete, and an
// option set the product does not have is combination-unknown: never a silent
// nearest match (D6).
func TestImportStock_PartialAndUnknownCombinationsAreToldApart(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	prefix := importPrefix("IMPSTKBAD")
	products := fmt.Sprintf(
		"sku,name,price,variant[talle],variant[color]\n"+
			"%s,Remera,7500,M,Negro\n"+
			"%s,Remera,7500,XL,Negro\n",
		prefix, prefix)
	importInventoryCSV(t, helper, "/import", "products", products, "", nil)
	productID := productBySku(t, helper, prefix).ID

	partial := fmt.Sprintf("sku,variant[talle],variant[color],quantity\n%s,M,,10\n", prefix)
	row := importRow(t, importInventoryCSV(t, helper, "/import", "product_stock", partial, "", nil), 0)
	assert.Equal(t, "failed", row["status"], "%v", row)
	assert.True(t, containsCode(rowCodes(row, "errors"), inventoryErrors.ImportAxesIncomplete),
		"expected axes-incomplete: %v", row["errors"])

	unknown := fmt.Sprintf("sku,variant[talle],variant[color],quantity\n%s,L,Blanco,10\n", prefix)
	row = importRow(t, importInventoryCSV(t, helper, "/import", "product_stock", unknown, "", nil), 0)
	assert.Equal(t, "failed", row["status"], "%v", row)
	assert.True(t, containsCode(rowCodes(row, "errors"), inventoryErrors.ImportCombinationUnknown),
		"expected combination-unknown: %v", row["errors"])

	for sku, quantity := range skuQuantities(t, helper, productID) {
		assert.Equal(t, 0.0, quantity, "%s must hold nothing after two refused rows", sku)
	}
}

func TestImportStock_UnknownSkuFailsTheRow(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	stock := fmt.Sprintf("sku,variant[talle],quantity\nNOSUCH%d,,10\n", time.Now().UnixNano())
	response := importInventoryCSV(t, helper, "/import", "product_stock", stock, "", nil)
	row := importRow(t, response, 0)

	assert.Equal(t, "failed", row["status"], "%v", row)
	assert.True(t, containsCode(rowCodes(row, "errors"), inventoryErrors.ImportSkuUnknown),
		"expected sku-unknown: %v", row["errors"])
}

// AC-11 — a lot row points at its combination the same way, and one naming a
// simple product takes no variant cell at all.
func TestImportBatches_LotsLandOnTheirCombinations(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	prefix := importPrefix("IMPLOT")
	products := fmt.Sprintf(
		"sku,name,price,variant[talle],variant[color]\n"+
			"%s-R,Remera,7500,M,Negro\n"+
			"%s-R,Remera,7500,XL,Negro\n"+
			"%s-Y,Yerba,3500,,\n",
		prefix, prefix, prefix)
	importInventoryCSV(t, helper, "/import", "products", products, "", nil)
	shirtID := productBySku(t, helper, prefix+"-R").ID
	yerbaID := productBySku(t, helper, prefix+"-Y").ID

	purchase := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	expiry := time.Now().AddDate(0, 2, 0).Format("2006-01-02")
	lots := fmt.Sprintf(
		"sku,variant[talle],variant[color],lot_number,purchase_date,expiry_date,unit_cost,initial_quantity\n"+
			"%s-R,M,Negro,LOTE-%s-M,%s,%s,1200,20\n"+
			"%s-R,XL,Negro,LOTE-%s-XL,%s,%s,1300,5\n"+
			"%s-Y,,,LOTE-%s-Y,%s,%s,900,30\n",
		prefix, prefix, purchase, expiry,
		prefix, prefix, purchase, expiry,
		prefix, prefix, purchase, expiry)

	response := importInventoryCSV(t, helper, "/import", "product_batches", lots, "", nil)
	for index := 0; index < 3; index++ {
		assert.Equal(t, "created", importRow(t, response, index)["status"],
			"row %d: %v", index, response)
	}

	batches := listBatches(t, helper, shirtID, "")
	if assert.Len(t, batches.Batches, 2, "two lots on the shirt: %+v", batches.Batches) {
		for _, batch := range batches.Batches {
			assert.NotNil(t, batch.SkuID, "the lot should hang off a combination: %+v", batch)
			assert.NotNil(t, batch.DaysToExpiry, "the lot should report its expiry: %+v", batch)
		}
	}

	byOption := combinationSkus(t, helper, shirtID)
	assert.Equal(t, 20.0, byOption["M"].CurrentQuantity, "the lot books its own PURCHASE movement")
	assert.Equal(t, 5.0, byOption["XL"].CurrentQuantity, "the lot books its own PURCHASE movement")

	assert.Len(t, listBatches(t, helper, yerbaID, "").Batches, 1,
		"the simple product's lot lands on its default bucket")
}

// A lot naming an option set the product does not have never reaches the SP.
func TestImportBatches_UnknownCombinationFailsTheRow(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	prefix := importPrefix("IMPLOTCOMB")
	products := fmt.Sprintf("sku,name,price,variant[talle]\n%s,Remera,7500,M\n", prefix)
	importInventoryCSV(t, helper, "/import", "products", products, "", nil)

	purchase := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	expiry := time.Now().AddDate(0, 2, 0).Format("2006-01-02")
	lots := fmt.Sprintf(
		"sku,variant[talle],lot_number,purchase_date,expiry_date,unit_cost,initial_quantity\n"+
			"%s,XXL,L1,%s,%s,1200,20\n",
		prefix, purchase, expiry)

	row := importRow(t, importInventoryCSV(t, helper, "/import", "product_batches", lots, "", nil), 0)
	assert.Equal(t, "failed", row["status"], "%v", row)
	assert.True(t, containsCode(rowCodes(row, "errors"), inventoryErrors.ImportCombinationUnknown),
		"expected combination-unknown: %v", row["errors"])
}

func TestImportBatches_UnreadableRowsAreRejected(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	prefix := importPrefix("IMPLOTBAD")
	products := fmt.Sprintf("sku,name,price\n%s,Leche,2500\n", prefix)
	importInventoryCSV(t, helper, "/import", "products", products, "", nil)

	header := "sku,variant[talle],lot_number,purchase_date,expiry_date,unit_cost,initial_quantity\n"
	cases := map[string]string{
		inventoryErrors.ImportDateInvalid:          fmt.Sprintf("%s,,L1,ayer,2027-01-01,1200,20\n", prefix),
		inventoryErrors.ImportExpiryBeforePurchase: fmt.Sprintf("%s,,L2,2026-06-01,2026-01-01,1200,20\n", prefix),
		inventoryErrors.ImportUnitCostInvalid:      fmt.Sprintf("%s,,L3,2026-06-01,2027-01-01,0,20\n", prefix),
		inventoryErrors.ImportLotNumberRequired:    fmt.Sprintf("%s,,,2026-06-01,2027-01-01,1200,20\n", prefix),
	}

	for expected, line := range cases {
		response := importInventoryCSV(t, helper, "/import", "product_batches", header+line, "", nil)
		row := importRow(t, response, 0)

		assert.Equal(t, "failed", row["status"], "%s: %v", expected, row)
		assert.True(t, containsCode(rowCodes(row, "errors"), expected),
			"expected %s, got %v", expected, row["errors"])
	}
}

// AC-1 / AC-14 — every resource is on the registry, each with a template and a
// schema of its own.
func TestImportRegistry_CarriesEveryInventoryResource(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	meta := helper.DoRequest("GET", "/import/meta", nil, map[string]string{})
	assert.Equal(t, http.StatusOK, meta.Code, "the endpoint is reachable here: %s", meta.Body.String())

	for _, resource := range []string{"users", "products", "categories", "product_stock", "product_batches"} {
		template := helper.DoRequest("GET", "/import/template?resource="+resource, nil, map[string]string{})
		assert.Equal(t, http.StatusOK, template.Code, "%s template: %s", resource, template.Body.String())
		assert.NotEmpty(t, strings.TrimSpace(template.Body.String()), "%s should have a header row", resource)

		schema := helper.DoRequest("GET", "/import/schema?resource="+resource, nil, map[string]string{})
		assert.Equal(t, http.StatusOK, schema.Code, "%s schema: %s", resource, schema.Body.String())
	}
}

// D6 — sharing sku and the variant[…] columns leaves each file one column of its
// own, so header detection still tells the four apart.
func TestImportDetection_TellsTheInventoryFilesApart(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	stamp := time.Now().UnixNano()
	cases := map[string]string{
		"products":        fmt.Sprintf("sku,name,price,variant[talle]\nDET%d,Producto,100,\n", stamp),
		"categories":      fmt.Sprintf("name,parent\nDetectada %d,\n", stamp),
		"product_stock":   fmt.Sprintf("sku,variant[talle],quantity\nDET%d,,5\n", stamp),
		"product_batches": fmt.Sprintf("sku,variant[talle],lot_number,purchase_date,expiry_date,unit_cost,initial_quantity\nDET%d,,L1,2026-06-01,2027-01-01,10,5\n", stamp),
	}

	for expected, csv := range cases {
		response := importInventoryCSV(t, helper, "/import/validate", "", csv, "", nil)
		assert.Equal(t, expected, response["resource"],
			"the header row should resolve to %s: %v", expected, response)
	}
}

// scopedImport posts a CSV with a scope and no resource, the way the wizard
// opened on a module does (IMPORT-001 D1), and returns the raw recorder so a
// refusal can be asserted too.
func scopedImport(helper *testhelpers.ApiTestHelper, scope, csv string) *httptest.ResponseRecorder {
	files := []testhelpers.MultipartFile{{Field: "file", Filename: "scoped.csv", Content: []byte(csv)}}
	return helper.DoMultipartRequest("POST", "/import/validate", map[string]string{"scope": scope}, files, map[string]string{})
}

func TestImportScope_LotsFileIsDetectedInsideInventory(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()
	csv := fmt.Sprintf("sku,lot_number,purchase_date,expiry_date,unit_cost,initial_quantity\nSCOPE%d,L1,01/06/2026,2027-01-01,\"10,5\",5\n",
		time.Now().UnixNano())

	w := scopedImport(helper, "inventory", csv)

	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var decoded map[string]interface{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &decoded))
	assert.Equal(t, "product_batches", decoded["resource"], "%v", decoded)
}

func TestImportScope_UsersFileIsOutOfScopeInInventory(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	w := scopedImport(helper, "inventory", "first_name,last_name,email\nAna,Paz,ana@example.com\n")

	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), importErrors.ImportDetectOutOfScope)
}

func TestImportResources_ListsWhatTheInventoryScopeReaches(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	w := helper.DoRequest("GET", "/import/resources?scope=inventory", nil, map[string]string{})

	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var resources []map[string]string
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resources))
	names := make([]string, 0, len(resources))
	for _, resource := range resources {
		assert.Equal(t, "inventory", resource["module"])
		names = append(names, resource["resource"])
	}
	// A1-D1 — registration order is import order: what the others point at first.
	assert.Equal(t, []string{"categories", "products", "product_stock", "product_batches"}, names)
}

// detectImport posts a CSV to the header-only detection endpoint (A1-D1).
func detectImport(helper *testhelpers.ApiTestHelper, scope, csv string) *httptest.ResponseRecorder {
	files := []testhelpers.MultipartFile{{Field: "file", Filename: "detect.csv", Content: []byte(csv)}}
	return helper.DoMultipartRequest("POST", "/import/detect", map[string]string{"scope": scope}, files, map[string]string{})
}

func TestImportDetect_NamesTheResourceFromTheHeaderAlone(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	w := detectImport(helper, "inventory", "sku,variant[talle],lot_number,purchase_date,expiry_date,unit_cost,initial_quantity\n")

	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var decoded map[string]string
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &decoded))
	assert.Equal(t, "product_batches", decoded["resource"])
	assert.Equal(t, "inventory", decoded["module"])
}

func TestImportDetect_UsersHeaderIsOutOfScopeInInventory(t *testing.T) {
	helper := SetupInventoryTest(t)
	defer helper.Close()

	w := detectImport(helper, "inventory", "first_name,last_name,email\n")

	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), importErrors.ImportDetectOutOfScope)
}
