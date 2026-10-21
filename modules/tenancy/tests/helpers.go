//go:build integration
// +build integration

package tenancy_test

import (
	"context"
	"testing"

	coreTestHelpers "josex/web/modules/core/testhelpers"

	"github.com/google/uuid"
)

// markRoleSystem makes a role a system role: no endpoint creates one, and the role SPs refuse to edit them.
func markRoleSystem(t *testing.T, helper *coreTestHelpers.ApiTestHelper, roleID string) {
	t.Helper()
	if _, err := helper.DB().Execute(context.Background(), `UPDATE tenancy.roles SET is_system = TRUE WHERE id = $1`, roleID); err != nil {
		t.Fatalf("mark role system: %v", err)
	}
}

// secondBusiness is a fresh business beside the test one, removed with its rows when the test ends.
func secondBusiness(t *testing.T, helper *coreTestHelpers.ApiTestHelper) string {
	t.Helper()
	id := uuid.NewString()
	if _, err := helper.DB().Execute(context.Background(),
		`INSERT INTO tenancy.tenants (id, slug, name) VALUES ($1, $2, 'Second Operator')`, id, "invites-"+id[:8]); err != nil {
		t.Fatalf("second business: %v", err)
	}
	t.Cleanup(func() {
		_, _ = helper.DB().Execute(context.Background(), `DELETE FROM tenancy.tenants WHERE id = $1`, id)
	})
	return id
}

// activeMemberships counts the businesses the account is an active member of.
func activeMemberships(t *testing.T, helper *coreTestHelpers.ApiTestHelper, userID string) int {
	t.Helper()
	var n int
	if err := helper.DB().QueryRow(context.Background(),
		`SELECT COUNT(*) FROM tenancy.tenant_users WHERE user_id = $1 AND is_active`, userID).Scan(&n); err != nil {
		t.Fatalf("memberships: %v", err)
	}
	return n
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
