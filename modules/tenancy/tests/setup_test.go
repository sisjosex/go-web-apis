//go:build integration
// +build integration

package tenancy_test

import (
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"josex/web/modules/core/testhelpers"

	"github.com/stretchr/testify/assert"
)

// TENANCY-008: the first-run checklist, built from the enabled modules and ordered by the business types.

type setupStep struct {
	code string
	done bool
}

func readSetup(t *testing.T, helper *testhelpers.ApiTestHelper, slug string) ([]setupStep, map[string]interface{}) {
	t.Helper()
	w := helper.DoRequest("GET", "/tenants/"+slug+"/setup", nil, map[string]string{})
	if w.Code != http.StatusOK {
		t.Fatalf("setup: %d %s", w.Code, w.Body.String())
	}
	body := decode(t, w.Body.Bytes())
	steps := []setupStep{}
	for _, row := range body["steps"].([]interface{}) {
		s := row.(map[string]interface{})
		steps = append(steps, setupStep{code: s["code"].(string), done: s["done"].(bool)})
	}
	return steps, body
}

func codesOf(steps []setupStep) []string {
	out := make([]string, len(steps))
	for i, s := range steps {
		out[i] = s.code
	}
	return out
}

func doneOf(steps []setupStep, code string) bool {
	for _, s := range steps {
		if s.code == code {
			return s.done
		}
	}
	return false
}

// A hardware store: its data, the catalogue, the team; creating a product marks its step done.
func TestSetup_RetailStepsAndProductDone(t *testing.T) {
	helper := testhelpers.SetupApiTest(t)
	defer helper.Close()
	slug := freshTenant(t, helper)
	code, body := putVertical(helper, slug, map[string]interface{}{"codes": []string{"hardware"}})
	assert.Equal(t, http.StatusOK, code, body)

	steps, out := readSetup(t, helper, slug)
	assert.Equal(t, []string{"business", "products", "categories", "team"}, codesOf(steps))
	assert.Equal(t, float64(0), out["done"])
	assert.Equal(t, float64(4), out["total"])
	assert.Equal(t, float64(12), out["minutes_left"])

	w := helper.DoRequest("POST", "/inventory/products", map[string]interface{}{"sku": fmt.Sprintf("SETUP-%d", time.Now().UnixNano()), "name": "Martillo", "base_price": 45}, map[string]string{})
	assert.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	steps, out = readSetup(t, helper, slug)
	assert.True(t, doneOf(steps, "products"))
	assert.Equal(t, float64(1), out["done"])
}

// D2: two types make one list, the shared steps once, in the first type's order; a disabled module adds nothing.
func TestSetup_UnionOfTypesAndDisabledModules(t *testing.T) {
	helper := testhelpers.SetupApiTest(t)
	defer helper.Close()
	slug := freshTenant(t, helper)

	steps, _ := readSetup(t, helper, slug)
	assert.Equal(t, []string{"business", "team"}, codesOf(steps), "no module on, only the shared steps")

	code, body := putVertical(helper, slug, map[string]interface{}{"codes": []string{"school_transport", "hardware"}})
	assert.Equal(t, http.StatusOK, code, body)
	steps, _ = readSetup(t, helper, slug)
	assert.Equal(t, []string{"business", "companies", "drivers", "vehicles", "routes", "riders", "products", "categories", "team"}, codesOf(steps))

	for _, module := range []string{"sales", "inventory"} {
		w := helper.DoRequest("DELETE", "/tenants/"+slug+"/modules/"+module, nil, map[string]string{})
		assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	}
	steps, _ = readSetup(t, helper, slug)
	assert.NotContains(t, codesOf(steps), "products")

	w := helper.DoRequest("PATCH", "/tenants/"+slug+"/settings", map[string]interface{}{"general.locale": "es"}, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	steps, _ = readSetup(t, helper, slug)
	assert.True(t, doneOf(steps, "business"), "a general setting saved")
}

func TestSetup_OwnerAndAdminOnly(t *testing.T) {
	member := testhelpers.SetupApiTest(t)
	defer member.Close()
	member.Login("orguser@test.local", "OrgUser123!")
	member.SetTenantSlug("test-company")
	w := member.DoRequest("GET", "/tenants/test-company/setup", nil, map[string]string{})
	assert.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
}

// A workspace on its own database reads its steps there: its seeded categories count.
func TestSetup_DedicatedDatabaseReadsItsOwn(t *testing.T) {
	helper := testhelpers.SetupApiTest(t)
	defer helper.Close()
	if _, err := helper.LoginAsSuperAdmin(); err != nil {
		t.Fatalf("login as super_admin: %v", err)
	}
	slug := fmt.Sprintf("setup-own-%d", time.Now().UnixNano())
	w := helper.DoRequest("POST", "/tenants", map[string]interface{}{"slug": slug, "name": "Own Database", "database_url": os.Getenv("DATABASE_URL")}, map[string]string{})
	if w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Fatalf("create tenant: %d %s", w.Code, w.Body.String())
	}
	helper.SetTenantSlug(slug)
	code, body := putVertical(helper, slug, map[string]interface{}{"codes": []string{"hardware"}, "starter": true})
	assert.Equal(t, http.StatusOK, code, body)

	steps, _ := readSetup(t, helper, slug)
	assert.True(t, doneOf(steps, "categories"), "seeded on its own database")
	assert.False(t, doneOf(steps, "products"))
}
