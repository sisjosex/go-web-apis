//go:build integration
// +build integration

package sales_test

import (
	"testing"

	coreTestHelpers "josex/web/modules/core/testhelpers"
)

// CleanSalesDatabase cleans only sales tables while preserving auth and system data
func CleanSalesDatabase(helper *coreTestHelpers.ApiTestHelper) error {
	return helper.CleanDatabaseForSchemas("sales")
}

// SetupSalesTest initializes test with default cleanup
func SetupSalesTest(t *testing.T) *coreTestHelpers.ApiTestHelper {
	return coreTestHelpers.SetupApiTest(t)
}
