//go:build integration
// +build integration

package inventory_test

import (
	"testing"

	coreTestHelpers "josex/web/modules/core/testhelpers"
)

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
