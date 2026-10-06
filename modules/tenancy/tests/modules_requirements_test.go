//go:build integration
// +build integration

package tenancy_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"josex/web/modules/core/testhelpers"

	"github.com/stretchr/testify/assert"
)

// TENANCY-003: modules declare what they require, and every module route checks its module.

// freshTenant signs a new user in and creates their own workspace, with no module on; the helper then
// sends its slug. The seeded tenant is left alone: other packages' tests rely on its modules.
func freshTenant(t *testing.T, helper *testhelpers.ApiTestHelper) string {
	t.Helper()
	stamp := time.Now().UnixNano()
	email := fmt.Sprintf("modules-%d@test.com", stamp)
	slug := fmt.Sprintf("modules-%d", stamp)
	helper.Register(email, "$Password2025", "Mod", "Ules")
	helper.Login(email, "$Password2025")
	w := helper.DoRequest("POST", "/tenants/self-service", map[string]interface{}{"slug": slug, "name": "Modules"}, map[string]string{})
	if w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Fatalf("create tenant: %d %s", w.Code, w.Body.String())
	}
	helper.SetTenantSlug(slug)
	return slug
}

func decode(t *testing.T, body []byte) map[string]interface{} {
	t.Helper()
	var out map[string]interface{}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("decode: %v: %s", err, body)
	}
	return out
}

// TestModules_ListHasRequirementsAndVerticals - GET /modules names each module's requirements and the
// business types.
func TestModules_ListHasRequirementsAndVerticals(t *testing.T) {
	helper := testhelpers.SetupApiTest(t)
	defer helper.Close()
	freshTenant(t, helper)

	w := helper.DoRequest("GET", "/modules", nil, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	body := decode(t, w.Body.Bytes())

	requires := map[string][]interface{}{}
	for _, row := range body["modules"].([]interface{}) {
		m := row.(map[string]interface{})
		reqs, _ := m["requires"].([]interface{})
		requires[m["code"].(string)] = reqs
	}
	assert.Contains(t, requires["sales"], "inventory")

	codes := []string{}
	for _, row := range body["verticals"].([]interface{}) {
		codes = append(codes, row.(map[string]interface{})["code"].(string))
	}
	assert.Contains(t, codes, "transport")
}

// TestModules_EnableTakesRequirementsAndDisableRefusesWhileNeeded - enabling sales turns inventory on
// in the same write; inventory then cannot be turned off while sales is on (409).
func TestModules_EnableTakesRequirementsAndDisableRefusesWhileNeeded(t *testing.T) {
	helper := testhelpers.SetupApiTest(t)
	defer helper.Close()
	slug := freshTenant(t, helper)

	w := helper.DoRequest("PUT", "/tenants/"+slug+"/modules/sales", map[string]interface{}{"is_enabled": true}, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Subset(t, decode(t, w.Body.Bytes())["enabled"], []interface{}{"inventory", "sales"})

	w = helper.DoRequest("DELETE", "/tenants/"+slug+"/modules/inventory", nil, map[string]string{})
	assert.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	body := decode(t, w.Body.Bytes())
	assert.Equal(t, "tenant.module.required-by", body["error"])
	assert.Contains(t, body["detail"].(map[string]interface{})["required_by"], "sales")

	w = helper.DoRequest("DELETE", "/tenants/"+slug+"/modules/sales", nil, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	w = helper.DoRequest("DELETE", "/tenants/"+slug+"/modules/inventory", nil, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code, "nothing needs it any more: %s", w.Body.String())
}

// TestModules_RouteGatedAndCacheDroppedOnWrite - inventory answers 403 to a workspace without it, and
// 200 right after it is enabled: the write drops the cached list in this process (D2).
func TestModules_RouteGatedAndCacheDroppedOnWrite(t *testing.T) {
	helper := testhelpers.SetupApiTest(t)
	defer helper.Close()
	slug := freshTenant(t, helper)

	w := helper.DoRequest("GET", "/inventory/products", nil, map[string]string{})
	assert.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
	assert.Equal(t, "tenant.module.not-enabled", decode(t, w.Body.Bytes())["error"])

	w = helper.DoRequest("PUT", "/tenants/"+slug+"/modules/inventory", map[string]interface{}{"is_enabled": true}, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())

	w = helper.DoRequest("GET", "/inventory/products", nil, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
}
