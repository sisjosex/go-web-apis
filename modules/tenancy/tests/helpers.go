//go:build integration
// +build integration

package tenancy_test

import (
	"context"
	"testing"

	coreTestHelpers "josex/web/modules/core/testhelpers"
)

// markRoleSystem makes a role a system role: no endpoint creates one, and the role SPs refuse to edit them.
func markRoleSystem(t *testing.T, helper *coreTestHelpers.ApiTestHelper, roleID string) {
	t.Helper()
	if _, err := helper.DB().Execute(context.Background(), `UPDATE tenancy.roles SET is_system = TRUE WHERE id = $1`, roleID); err != nil {
		t.Fatalf("mark role system: %v", err)
	}
}

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
