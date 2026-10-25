//go:build integration
// +build integration

package tenancy_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"josex/web/modules/core/testhelpers"

	"github.com/stretchr/testify/assert"
)

// TENANCY-006: business types as a registry matrix, picked in one request.

func putVertical(helper *testhelpers.ApiTestHelper, slug string, body map[string]interface{}) (int, map[string]interface{}) {
	w := helper.DoRequest("PUT", "/tenants/"+slug+"/vertical", body, map[string]string{})
	out := map[string]interface{}{}
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

// categoryNames lists the workspace's live categories through inventory's own route.
func categoryNames(t *testing.T, helper *testhelpers.ApiTestHelper) []string {
	t.Helper()
	w := helper.DoRequest("GET", "/inventory/categories?limit=100", nil, map[string]string{})
	if w.Code != http.StatusOK {
		t.Fatalf("list categories: %d %s", w.Code, w.Body.String())
	}
	names := []string{}
	for _, row := range decode(t, w.Body.Bytes())["categories"].([]interface{}) {
		names = append(names, row.(map[string]interface{})["name"].(string))
	}
	return names
}

func decodeList(t *testing.T, body []byte) []map[string]interface{} {
	t.Helper()
	var out []map[string]interface{}
	if err := json.Unmarshal(body, &out); err != nil {
		var wrapped struct {
			Data []map[string]interface{} `json:"data"`
		}
		if err := json.Unmarshal(body, &wrapped); err != nil {
			t.Fatalf("decode list: %v: %s", err, body)
		}
		return wrapped.Data
	}
	return out
}

func assertUnique(t *testing.T, names []string) {
	t.Helper()
	seen := map[string]bool{}
	for _, name := range names {
		assert.False(t, seen[name], "category %q twice", name)
		seen[name] = true
	}
}

// D1, D3: GET /modules offers the types whose modules this deployment runs, with their compiled capabilities.
func TestVerticals_ListOffersWhatIsInstalled(t *testing.T) {
	helper := testhelpers.SetupApiTest(t)
	defer helper.Close()
	freshTenant(t, helper)

	w := helper.DoRequest("GET", "/modules", nil, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	body := decode(t, w.Body.Bytes())

	byCode := map[string]map[string]interface{}{}
	for _, row := range body["verticals"].([]interface{}) {
		v := row.(map[string]interface{})
		byCode[v["code"].(string)] = v
	}
	assert.Contains(t, byCode, "hardware")
	assert.Equal(t, []interface{}{"inventory", "sales", "users"}, byCode["hardware"]["modules"])
	assert.NotEmpty(t, byCode["hardware"]["group_name"])
	assert.NotContains(t, byCode, "barber", "booking is not installed")
	assert.NotContains(t, byCode, "intercity_bus")
	assert.NotContains(t, byCode, "radio_taxi")

	capabilities := []string{}
	for _, row := range body["capabilities"].([]interface{}) {
		capabilities = append(capabilities, row.(map[string]interface{})["code"].(string))
	}
	assert.Subset(t, capabilities, []string{"lots", "variants"})
}

// D2, D4: two types in one request turn on their union, store both, seed their categories once.
func TestVerticals_PresetUnionAndIdempotentSeed(t *testing.T) {
	helper := testhelpers.SetupApiTest(t)
	defer helper.Close()
	slug := freshTenant(t, helper)

	code, body := putVertical(helper, slug, map[string]interface{}{"codes": []string{"bakery", "cafe"}, "starter": true})
	assert.Equal(t, http.StatusOK, code, body)
	assert.Subset(t, body["enabled"], []interface{}{"inventory", "sales", "users"})
	assert.Equal(t, []interface{}{"bakery", "cafe"}, body["verticals"])
	assert.Equal(t, "inventory", body["primary_module"], "pos is not installed")
	assert.Equal(t, []interface{}{}, body["capabilities"], "neither type uses a compiled capability")
	seeded := int(body["seeded_categories"].(float64))
	assert.Equal(t, 7, seeded, "bread cakes pastries beverages + hot drinks cold drinks sandwiches")

	names := categoryNames(t, helper)
	assert.Len(t, names, seeded)
	assertUnique(t, names)

	code, body = putVertical(helper, slug, map[string]interface{}{"codes": []string{"bakery", "cafe"}, "starter": true})
	assert.Equal(t, http.StatusOK, code, body)
	assert.Equal(t, float64(0), body["seeded_categories"], "repeating creates nothing")
	assert.Len(t, categoryNames(t, helper), seeded)

	w := helper.DoRequest("GET", "/tenants/me", nil, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	found := false
	for _, row := range decodeList(t, w.Body.Bytes()) {
		if row["slug"] == slug {
			found = true
			assert.Equal(t, []interface{}{"bakery", "cafe"}, row["verticals"])
			assert.Equal(t, "inventory", row["primary_module"])
		}
	}
	assert.True(t, found)
}

// The refusals: unknown type, a module the types do not suggest, a main module left off, no type at all.
func TestVerticals_Refusals(t *testing.T) {
	helper := testhelpers.SetupApiTest(t)
	defer helper.Close()
	slug := freshTenant(t, helper)

	cases := []struct {
		name string
		body map[string]interface{}
		code string
	}{
		{"unknown", map[string]interface{}{"codes": []string{"spaceport"}}, "tenant.vertical.unknown"},
		{"hidden by D1", map[string]interface{}{"codes": []string{"barber"}}, "tenant.vertical.unknown"},
		{"not suggested", map[string]interface{}{"codes": []string{"bakery"}, "extra_modules": []string{"tracking"}}, "tenant.vertical.not-suggested"},
		{"primary off", map[string]interface{}{"codes": []string{"bakery"}, "primary_module": "tracking"}, "tenant.vertical.primary-not-enabled"},
		{"nothing", map[string]interface{}{"codes": []string{}}, "tenant.vertical.required"},
	}
	for _, tc := range cases {
		code, body := putVertical(helper, slug, tc.body)
		assert.Equal(t, http.StatusUnprocessableEntity, code, tc.name)
		assert.Equal(t, tc.code, body["error"], tc.name)
	}

	member := testhelpers.SetupApiTest(t)
	defer member.Close()
	member.Login("orguser@test.local", "OrgUser123!")
	member.SetTenantSlug("test-company")
	code, body := putVertical(member, "test-company", map[string]interface{}{"codes": []string{"bakery"}})
	assert.Equal(t, http.StatusForbidden, code, body)
}

// A re-pick never turns a module off; a suggested module is enabled when sent; the main module changes alone.
func TestVerticals_RepickKeepsModulesAndSwitchesPrimary(t *testing.T) {
	helper := testhelpers.SetupApiTest(t)
	defer helper.Close()
	slug := freshTenant(t, helper)

	w := helper.DoRequest("PUT", "/tenants/"+slug+"/modules/tracking", map[string]interface{}{"is_enabled": true}, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())

	code, body := putVertical(helper, slug, map[string]interface{}{"codes": []string{"ice_cream"}, "extra_modules": []string{"inventory"}})
	assert.Equal(t, http.StatusOK, code, body)
	assert.Subset(t, body["enabled"], []interface{}{"tracking", "inventory", "sales"})
	assert.Equal(t, "sales", body["primary_module"])
	assert.Equal(t, float64(0), body["seeded_categories"], "no starter outside the sign-up")

	code, body = putVertical(helper, slug, map[string]interface{}{"primary_module": "tracking"})
	assert.Equal(t, http.StatusOK, code, body)
	assert.Equal(t, "tracking", body["primary_module"])
	assert.Equal(t, []interface{}{"ice_cream"}, body["verticals"], "the types stay")
}

// D3: a workspace that never stored capabilities has every compiled one on; the sign-up stores its types'
// list, and a later pick only adds to it.
func TestVerticals_CapabilitiesAllUntilStored(t *testing.T) {
	helper := testhelpers.SetupApiTest(t)
	defer helper.Close()
	slug := freshTenant(t, helper)

	w := helper.DoRequest("GET", "/tenants/"+slug+"/settings", nil, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Subset(t, decode(t, w.Body.Bytes())["settings"].(map[string]interface{})["capabilities"], []interface{}{"lots", "variants"})

	code, body := putVertical(helper, slug, map[string]interface{}{"codes": []string{"clothing"}, "starter": true})
	assert.Equal(t, http.StatusOK, code, body)
	assert.Equal(t, []interface{}{"variants"}, body["capabilities"])

	code, body = putVertical(helper, slug, map[string]interface{}{"codes": []string{"butcher"}})
	assert.Equal(t, http.StatusOK, code, body)
	assert.Equal(t, []interface{}{"lots", "variants"}, body["capabilities"])

	w = helper.DoRequest("PATCH", "/tenants/"+slug+"/settings", map[string]interface{}{"capabilities": []string{"variants"}}, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, []interface{}{"variants"}, decode(t, w.Body.Bytes())["settings"].(map[string]interface{})["capabilities"])
}

// A workspace on its own database gets its starter categories there, through the request's tenant pool.
func TestVerticals_DedicatedDatabaseSeedsItsOwn(t *testing.T) {
	helper := testhelpers.SetupApiTest(t)
	defer helper.Close()
	if _, err := helper.LoginAsSuperAdmin(); err != nil {
		t.Fatalf("login as super_admin: %v", err)
	}
	slug := fmt.Sprintf("vertical-own-%d", time.Now().UnixNano())
	w := helper.DoRequest("POST", "/tenants", map[string]interface{}{"slug": slug, "name": "Own Database", "database_url": os.Getenv("DATABASE_URL")}, map[string]string{})
	if w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Fatalf("create tenant: %d %s", w.Code, w.Body.String())
	}
	helper.SetTenantSlug(slug)

	code, body := putVertical(helper, slug, map[string]interface{}{"codes": []string{"hardware"}, "starter": true})
	assert.Equal(t, http.StatusOK, code, body)
	assert.Equal(t, float64(5), body["seeded_categories"])
	names := categoryNames(t, helper)
	assert.Len(t, names, 5)
	assertUnique(t, names)
}
