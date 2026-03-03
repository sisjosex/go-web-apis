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

// SetupInventoryTest initializes test with default cleanup (all non-auth tables)
// Use CleanInventoryDatabase() manually in test if you need inventory-only cleanup
func SetupInventoryTest(t *testing.T) *coreTestHelpers.ApiTestHelper {
	return coreTestHelpers.SetupApiTest(t)
}
