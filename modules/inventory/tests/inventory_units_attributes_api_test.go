//go:build integration
// +build integration

package inventory_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	coreTestHelpers "josex/web/modules/core/testhelpers"
	"josex/web/modules/inventory/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// INV-024: attributes describe products across the catalogue, categories bring them, the list filters and searches by them.

func stampOf() string { return fmt.Sprintf("%d", time.Now().UnixNano()) }

func newAttribute(t *testing.T, helper *coreTestHelpers.ApiTestHelper, name, kind string, options ...string) models.Attribute {
	t.Helper()
	rows := []map[string]interface{}{}
	for _, option := range options {
		rows = append(rows, map[string]interface{}{"value": option})
	}
	w := helper.DoRequest("POST", "/inventory/attributes", map[string]interface{}{"name": name, "type": kind, "options": rows}, nil)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var attribute models.Attribute
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &attribute))
	return attribute
}

func newAttributedProduct(t *testing.T, helper *coreTestHelpers.ApiTestHelper, name string, attributes map[string]interface{}) string {
	t.Helper()
	w := helper.DoRequest("POST", "/inventory/products", map[string]interface{}{
		"sku": "ATTR_" + stampOf(), "name": name, "base_price": 10, "attributes": attributes,
	}, nil)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var created struct {
		ProductID string `json:"product_id"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	return created.ProductID
}

func newCategoryUnder(t *testing.T, helper *coreTestHelpers.ApiTestHelper, name string, parent *string) string {
	t.Helper()
	body := map[string]interface{}{"name": name, "slug": strings.ToLower(name) + "-" + stampOf()}
	if parent != nil {
		body["parent_id"] = *parent
	}
	w := helper.DoRequest("POST", "/inventory/categories", body, nil)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var created struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	return created.ID
}

func listNames(t *testing.T, helper *coreTestHelpers.ApiTestHelper, query string) []string {
	t.Helper()
	w := helper.DoRequest("GET", "/inventory/products?page_size=100&"+query, nil, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var list models.ListProductsResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &list))
	names := []string{}
	for _, row := range list.Products {
		names = append(names, row.Name)
	}
	return names
}

func TestAttributes_ValuesAreCheckedByType(t *testing.T) {
	helper := SetupInventoryTest(t)
	stamp := stampOf()
	brand := newAttribute(t, helper, "Marca "+stamp, "list", "Truper", "Stanley")
	weight := newAttribute(t, helper, "Peso "+stamp, "number")
	note := newAttribute(t, helper, "Nota "+stamp, "text")

	id := newAttributedProduct(t, helper, "Martillo "+stamp, map[string]interface{}{brand.Code: "truper", weight.Code: 2, note.Code: "Mango de fibra"})
	detail := productDetail(t, helper, id)
	assert.Equal(t, []string{"Truper"}, detail.Attributes[brand.Code], "stored in the option's own spelling")
	assert.Equal(t, []string{"2"}, detail.Attributes[weight.Code])

	cases := map[string]map[string]interface{}{
		"not an option": {brand.Code: "Bosch"},
		"not a number":  {weight.Code: "pesado"},
		"unknown code":  {"no-existe-" + stamp: "x"},
	}
	for name, attributes := range cases {
		w := helper.DoRequest("POST", "/inventory/products", map[string]interface{}{
			"sku": "BAD_" + stampOf(), "name": name, "base_price": 10, "attributes": attributes,
		}, nil)
		assert.Equal(t, http.StatusUnprocessableEntity, w.Code, "%s: %s", name, w.Body.String())
		assert.Contains(t, w.Body.String(), "inventory.attribute.invalid", name)
	}
	w := helper.DoRequest("POST", "/inventory/products", map[string]interface{}{
		"sku": "BAD_" + stampOf(), "name": "x", "base_price": 10, "attributes": map[string]interface{}{brand.Code: "Bosch"},
	}, nil)
	assert.Contains(t, w.Body.String(), brand.Code, "the refusal names the attribute")

	w = helper.DoRequest("DELETE", "/inventory/attributes/"+brand.ID, nil, nil)
	assert.Equal(t, http.StatusConflict, w.Code, "in use: %s", w.Body.String())
	assert.Contains(t, w.Body.String(), "inventory.attribute.in-use")
	w = helper.DoRequest("PUT", "/inventory/attributes/"+brand.ID, map[string]interface{}{"name": brand.Name, "type": "list", "options": []map[string]interface{}{{"value": "Stanley"}}}, nil)
	assert.Equal(t, http.StatusConflict, w.Code, "a product holds Truper: %s", w.Body.String())
}

func TestAttributes_CategoryBringsAndInherits(t *testing.T) {
	helper := SetupInventoryTest(t)
	stamp := stampOf()
	size := newAttribute(t, helper, "Talla "+stamp, "list", "S", "M")
	clothes := newCategoryUnder(t, helper, "Ropa", nil)
	shirts := newCategoryUnder(t, helper, "Poleras", &clothes)
	shoes := newCategoryUnder(t, helper, "Calzado", nil)

	w := helper.DoRequest("PUT", "/inventory/categories/"+clothes+"/attributes", map[string]interface{}{"attribute_ids": []string{size.ID}}, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	w = helper.DoRequest("GET", "/inventory/categories/"+shirts+"/attributes", nil, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var set models.CategoryAttributesResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &set))
	assert.Empty(t, set.Own)
	require.Len(t, set.Inherited, 1)
	assert.Equal(t, size.ID, set.Inherited[0].ID)
	assert.Len(t, set.Inherited[0].Options, 2)

	// Moved under Calzado, Poleras stops inheriting from Ropa.
	w = helper.DoRequest("PATCH", "/inventory/categories/order", map[string]interface{}{"items": []map[string]interface{}{
		{"id": shirts, "parent_id": shoes, "display_order": 3},
	}}, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	w = helper.DoRequest("GET", "/inventory/categories/"+shirts+"/attributes", nil, nil)
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &set))
	assert.Empty(t, set.Inherited)
	w = helper.DoRequest("GET", "/inventory/categories/"+shirts, nil, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), shoes, "the new parent persists")

	// A cycle is refused and undoes the whole statement.
	w = helper.DoRequest("PATCH", "/inventory/categories/order", map[string]interface{}{"items": []map[string]interface{}{
		{"id": shoes, "parent_id": shirts, "display_order": 0},
	}}, nil)
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "category.circular-hierarchy")

	// Another business reads nothing of it.
	stranger := ownerOfAnotherTenant(t)
	w = stranger.DoRequest("GET", "/inventory/categories/"+clothes+"/attributes", nil, nil)
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	w = stranger.DoRequest("PATCH", "/inventory/categories/order", map[string]interface{}{"items": []map[string]interface{}{{"id": clothes, "display_order": 1}}}, nil)
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	w = stranger.DoRequest("PUT", "/inventory/attributes/"+size.ID, map[string]interface{}{"name": "x", "type": "list"}, nil)
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
}

func TestAttributes_AxisBindsToTheSharedAttribute(t *testing.T) {
	helper := SetupInventoryTest(t)
	stamp := stampOf()
	body := func(name string) map[string]interface{} {
		return map[string]interface{}{
			"sku": "AXIS_" + stampOf(), "name": "Polera " + stamp, "base_price": 10,
			"variants": map[string]interface{}{"groups": []map[string]interface{}{{
				"group_type": name, "is_required": true, "max_selections": 1, "affects_inventory": true,
				"options": []map[string]interface{}{{"name": "M"}, {"name": "L"}},
			}}},
		}
	}
	w := helper.DoRequest("POST", "/inventory/products", body("Tálla"+stamp), nil)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	w = helper.DoRequest("POST", "/inventory/products", body("TALLA"+stamp), nil)
	require.Equal(t, http.StatusCreated, w.Code, "the same attribute, without case or accents: %s", w.Body.String())

	w = helper.DoRequest("GET", "/inventory/attributes", nil, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var list models.ListAttributesResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &list))
	var found []models.Attribute
	for _, row := range list.Attributes {
		if strings.Contains(row.Code, stamp) {
			found = append(found, row)
		}
	}
	require.Len(t, found, 1, "one attribute for both spellings")
	assert.EqualValues(t, 2, found[0].ProductCount)
	assert.Len(t, found[0].Options, 2)

	// Filtering reads the chips of an axis.
	names := listNames(t, helper, "attr["+found[0].Code+"]=m")
	assert.Len(t, names, 2)

	// With an id, the axis must use that attribute's options.
	bound := body("Otro" + stamp)
	groups := bound["variants"].(map[string]interface{})["groups"].([]map[string]interface{})
	groups[0]["attribute_id"] = found[0].ID
	groups[0]["options"] = []map[string]interface{}{{"name": "XXL"}}
	w = helper.DoRequest("POST", "/inventory/products", bound, nil)
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code, w.Body.String())
}

func TestProducts_FilterSearchAndFacets(t *testing.T) {
	helper := SetupInventoryTest(t)
	stamp := stampOf()
	brand := newAttribute(t, helper, "Fabricante "+stamp, "list", "Truper", "Stanley")
	newAttributedProduct(t, helper, "Martillo de uña "+stamp, map[string]interface{}{brand.Code: "Truper"})
	newAttributedProduct(t, helper, "Martillo bola "+stamp, map[string]interface{}{brand.Code: "Stanley"})
	newAttributedProduct(t, helper, "Serrucho "+stamp, map[string]interface{}{brand.Code: "Truper"})
	newAttributedProduct(t, helper, "Tubo PVC 1/2 "+stamp, nil)

	key := "attr[" + brand.Code + "]="
	assert.ElementsMatch(t, []string{"Martillo de uña " + stamp}, listNames(t, helper, key+"truper&search=martillo"))
	assert.Len(t, listNames(t, helper, key+"truper,stanley&search=martillo"), 2, "several values of one attribute")
	assert.ElementsMatch(t, []string{"Tubo PVC 1/2 " + stamp}, listNames(t, helper, "search=pvc+tubo+1/2+"+stamp), "words in any order")
	assert.ElementsMatch(t, []string{"Martillo de uña " + stamp}, listNames(t, helper, "search=MARTILLO+una+"+stamp), "without case or accents")
	assert.Contains(t, listNames(t, helper, "search=tuvo+pvc+1/2+"+stamp), "Tubo PVC 1/2 "+stamp, "by likeness when nothing matches")
	assert.Contains(t, listNames(t, helper, "search=truper+serrucho+"+stamp), "Serrucho "+stamp, "attribute values are searched")

	w := helper.DoRequest("GET", "/inventory/products/facets?search="+stamp, nil, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var facets models.ProductFacetsResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &facets))
	var counts = map[string]int64{}
	for _, facet := range facets.Facets {
		if facet.Code == brand.Code {
			for _, option := range facet.Options {
				counts[option.Value] = option.Count
			}
		}
	}
	assert.Equal(t, map[string]int64{"Truper": 2, "Stanley": 1}, counts)

	// A barcode saved on a presentation is searched too (a trigger keeps it, whichever SP writes).
	product := newAttributedProduct(t, helper, "Con codigo "+stamp, nil)
	detail := productDetail(t, helper, product)
	code := "SCAN" + stamp
	w = helper.DoRequest("PUT", "/inventory/products/"+product+"/units", map[string]interface{}{"units": []map[string]interface{}{
		{"id": detail.Units[0].ID, "unit_id": detail.Units[0].UnitID, "factor": 1, "is_base": true,
			"codes": []map[string]interface{}{{"sku_id": defaultSku(t, helper, product), "code": code}}},
	}}, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, []string{"Con codigo " + stamp}, listNames(t, helper, "search="+code))
}

func TestCategories_AppearanceReadsBack(t *testing.T) {
	helper := SetupInventoryTest(t)
	id := newCategoryUnder(t, helper, "Bebidas", nil)
	w := helper.DoRequest("PUT", "/inventory/categories/"+id, map[string]interface{}{"icon": "coffee", "color": "#123456", "pos_tab": false}, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var category struct {
		Icon   *string `json:"icon"`
		Color  *string `json:"color"`
		PosTab bool    `json:"pos_tab"`
	}
	w = helper.DoRequest("GET", "/inventory/categories/"+id, nil, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &category))
	assert.Equal(t, "coffee", *category.Icon)
	assert.Equal(t, "#123456", *category.Color)
	assert.False(t, category.PosTab)
}

func TestAttributes_ChangeAndDeleteOneNoProductUses(t *testing.T) {
	helper := SetupInventoryTest(t)
	stamp := stampOf()
	size := newAttribute(t, helper, "Largo "+stamp, "list", "Corto")
	options := []map[string]interface{}{{"id": size.Options[0].ID, "value": "Corto"}, {"value": "Largo"}}
	w := helper.DoRequest("PUT", "/inventory/attributes/"+size.ID, map[string]interface{}{"name": "Medida " + stamp, "type": "list", "options": options}, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var updated models.Attribute
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &updated))
	assert.Len(t, updated.Options, 2)
	assert.Contains(t, updated.Code, "medida")

	w = helper.DoRequest("DELETE", "/inventory/attributes/"+size.ID, nil, nil)
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
}
