//go:build integration
// +build integration

package tenancy_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"josex/web/modules/core/testhelpers"
)

// TestUserTenantsHideAppLevelsWhileTrackingOff - a guardian's membership exists for the tracking
// module only, so the tenant list leaves it out while the tenant has tracking off (TRACK-032).
func TestUserTenantsHideAppLevelsWhileTrackingOff(t *testing.T) {
	admin := testhelpers.SetupApiTest(t)
	defer admin.Close()
	admin.Login("admin@test.local", "Admin123!")
	admin.SetTenantSlug("test-company")

	toggle := func(enabled bool) {
		w := admin.DoRequest("PUT", "/tenants/test-company/modules/tracking",
			map[string]interface{}{"is_enabled": enabled}, map[string]string{})
		if w.Code != http.StatusOK && w.Code != http.StatusCreated {
			t.Fatalf("set tracking enabled=%v: got %d: %s", enabled, w.Code, w.Body.String())
		}
	}
	toggle(false)
	t.Cleanup(func() { toggle(true) })

	guardian := testhelpers.SetupApiTest(t)
	defer guardian.Close()
	guardian.Login("portal@test.local", "Portal123!")

	read := func() []map[string]interface{} {
		w := guardian.DoRequest("GET", "/tenants/me", nil, map[string]string{})
		if w.Code != http.StatusOK {
			t.Fatalf("my tenants: expected 200, got %d: %s", w.Code, w.Body.String())
		}
		var tenants []map[string]interface{}
		_ = json.Unmarshal(w.Body.Bytes(), &tenants)
		return tenants
	}
	assert.Empty(t, read(), "a portal membership is hidden while tracking is off")

	toggle(true)
	tenants := read()
	if assert.Len(t, tenants, 1) {
		assert.Equal(t, "test-company", tenants[0]["slug"])
	}
}

// TestUserTenantsCarryPermissions - TRACK-041 D1: a member's tenant row lists its roles' permission
// codes, an admin's lists "*".
func TestUserTenantsCarryPermissions(t *testing.T) {
	permissionsOf := func(email, password string) []interface{} {
		helper := testhelpers.SetupApiTest(t)
		defer helper.Close()
		helper.Login(email, password)
		w := helper.DoRequest("GET", "/tenants/me", nil, map[string]string{})
		if w.Code != http.StatusOK {
			t.Fatalf("my tenants: expected 200, got %d: %s", w.Code, w.Body.String())
		}
		var tenants []map[string]interface{}
		_ = json.Unmarshal(w.Body.Bytes(), &tenants)
		for _, tenant := range tenants {
			if tenant["slug"] == "test-company" {
				perms, _ := tenant["permissions"].([]interface{})
				return perms
			}
		}
		t.Fatalf("%s has no test-company membership", email)
		return nil
	}

	assert.Equal(t, []interface{}{"*"}, permissionsOf("admin@test.local", "Admin123!"))
	staff := permissionsOf("orguser@test.local", "OrgUser123!")
	assert.Contains(t, staff, "tracking:riders:read")
	assert.Contains(t, staff, "tracking:routes:read")
	assert.NotContains(t, staff, "tracking:vehicles:read")
	assert.NotContains(t, staff, "*")
}
