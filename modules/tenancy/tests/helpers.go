//go:build integration
// +build integration

package tenancy_test

import (
	"testing"

	coreTestHelpers "josex/web/modules/core/testhelpers"
)

// CleanTenancyDatabase cleans only tenancy tables while preserving auth and system data
// This is the proper cleanup for tenancy-specific tests
func CleanTenancyDatabase(helper *coreTestHelpers.ApiTestHelper) error {
	return helper.CleanDatabaseForSchemas("tenancy")
}

// SetupTenancyTest initializes test with default cleanup (all non-auth tables)
// Use CleanTenancyDatabase() manually in test if you need tenancy-only cleanup
func SetupTenancyTest(t *testing.T) *coreTestHelpers.ApiTestHelper {
	return coreTestHelpers.SetupApiTest(t)
}
