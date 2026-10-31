//go:build integration
// +build integration

package inventory_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	coreTestHelpers "josex/web/modules/core/testhelpers"
	"josex/web/modules/inventory/utils"

	"github.com/stretchr/testify/require"
)

// tenantToday is test-company's own date, the one the batch SPs read: it never set a zone, so the default.
func tenantToday() time.Time {
	return utils.DateIn(time.Now(), "America/La_Paz")
}

// expireBatch moves a lot's expiry date: the API refuses a past date, and a lot only gets there by waiting.
func expireBatch(t *testing.T, helper *coreTestHelpers.ApiTestHelper, batchID string, expiry time.Time) {
	t.Helper()
	if _, err := helper.DB().Execute(context.Background(),
		`UPDATE inventory.product_batches SET expiry_date = $1 WHERE id = $2`, expiry, batchID); err != nil {
		t.Fatalf("expire batch: %v", err)
	}
}

// removeStockRow deletes a SKU's product_stock row, the broken state sp_record_movement has to refuse.
func removeStockRow(t *testing.T, helper *coreTestHelpers.ApiTestHelper, productID string) {
	t.Helper()
	if _, err := helper.DB().Execute(context.Background(),
		`DELETE FROM inventory.product_stock WHERE product_id = $1`, productID); err != nil {
		t.Fatalf("remove stock row: %v", err)
	}
}

// setStockQuantity writes a quantity straight to the row, so a test can watch the database's own CHECK refuse it.
func setStockQuantity(t *testing.T, helper *coreTestHelpers.ApiTestHelper, productID string, quantity float64) error {
	t.Helper()
	_, err := helper.DB().Execute(context.Background(),
		`UPDATE inventory.product_stock SET current_quantity = $1 WHERE product_id = $2`, quantity, productID)
	return err
}

// ownerOfAnotherTenant signs in the owner of a fresh tenant that is not test-company.
func ownerOfAnotherTenant(t *testing.T) *coreTestHelpers.ApiTestHelper {
	t.Helper()
	other := coreTestHelpers.SetupApiTest(t)
	email := fmt.Sprintf("inv-other-%d@test.com", time.Now().UnixNano())
	_, err := other.Register(email, "$Password2025", "Other", "Owner")
	require.NoError(t, err)
	_, err = other.Login(email, "$Password2025")
	require.NoError(t, err)
	slug := fmt.Sprintf("inv-other-%d", time.Now().UnixNano())
	w := other.DoRequest("POST", "/tenants/self-service", map[string]interface{}{"name": "Other", "slug": slug}, nil)
	require.Contains(t, []int{http.StatusOK, http.StatusCreated}, w.Code, "create tenant: %s", w.Body.String())
	other.SetTenantSlug(slug)
	w = other.DoRequest("PUT", "/tenants/"+slug+"/modules/inventory", map[string]interface{}{"is_enabled": true}, nil)
	require.Equal(t, http.StatusOK, w.Code, "enable inventory: %s", w.Body.String())
	return other
}

// asMember signs in a fresh plain member of test-company holding a role with exactly perms.
func asMember(t *testing.T, helper *coreTestHelpers.ApiTestHelper, perms ...string) *coreTestHelpers.ApiTestHelper {
	t.Helper()
	const tenant = "test-company"
	email := fmt.Sprintf("inv-member-%d@test.com", time.Now().UnixNano())
	member := coreTestHelpers.SetupApiTest(t)
	_, err := member.Register(email, "$Password2025", "Inv", "Member")
	require.NoError(t, err)
	_, err = member.Login(email, "$Password2025")
	require.NoError(t, err)
	member.SetTenantSlug(tenant)

	w := helper.DoRequest("POST", "/tenants/"+tenant+"/users", map[string]interface{}{"user_id": member.GetUserID(), "role": "member"}, nil)
	require.Contains(t, []int{http.StatusOK, http.StatusCreated}, w.Code, "add member: %s", w.Body.String())

	if len(perms) > 0 {
		w = helper.DoRequest("POST", "/tenants/"+tenant+"/roles", map[string]interface{}{"name": "inv-" + email}, nil)
		require.Equal(t, http.StatusCreated, w.Code, "create role: %s", w.Body.String())
		var role struct {
			ID string `json:"id"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &role))
		w = helper.DoRequest("PUT", "/tenants/"+tenant+"/roles/"+role.ID+"/permissions", map[string]interface{}{"permissions": perms}, nil)
		require.Equal(t, http.StatusNoContent, w.Code, "set permissions: %s", w.Body.String())
		w = helper.DoRequest("POST", "/tenants/"+tenant+"/users/"+member.GetUserID()+"/roles/"+role.ID, nil, nil)
		require.Equal(t, http.StatusNoContent, w.Code, "assign role: %s", w.Body.String())
	}
	return member
}

// CleanInventoryDatabase cleans only inventory tables while preserving auth and system data
// This is the proper cleanup for inventory-specific tests
func CleanInventoryDatabase(helper *coreTestHelpers.ApiTestHelper) error {
	return helper.CleanDatabaseForSchemas("inventory")
}

// SetupInventoryTest initializes test, authenticates as super_admin, and sets the test tenant.
// All inventory operations require a tenant context.
func SetupInventoryTest(t *testing.T) *coreTestHelpers.ApiTestHelper {
	helper := coreTestHelpers.SetupApiTest(t)
	if _, err := helper.LoginAsSuperAdmin(); err != nil {
		t.Logf("⚠️  inventory test setup: login failed: %v", err)
	}
	helper.SetTenantSlug("test-company")
	return helper
}
