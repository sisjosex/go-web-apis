//go:build integration
// +build integration

package auth_test

import (
	"testing"

	coreTestHelpers "josex/web/modules/core/testhelpers"
)

// CleanAuthDatabase cleans only auth tables (preserves the module itself, clears sessions/OTP records)
// WARNING: This will remove all user session and OTP data!
func CleanAuthDatabase(helper *coreTestHelpers.ApiTestHelper) error {
	return helper.CleanDatabaseForSchemas("auth")
}

// SetupAuthTest initializes the test helper with auth-specific cleanup
func SetupAuthTest(t *testing.T) *coreTestHelpers.ApiTestHelper {
	helper := coreTestHelpers.SetupApiTest(t)
	CleanAuthDatabase(helper) // Clean auth tables on setup
	return helper
}
