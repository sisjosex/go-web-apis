//go:build integration
// +build integration

package billing_test

import (
	"testing"

	coreTestHelpers "josex/web/modules/core/testhelpers"
)

// SetupBillingTest initializes a test helper for billing integration tests.
// Does NOT truncate billing.plan_limits — it contains seeded data from migrations.
// Each test generates unique users to isolate state by user_id.
func SetupBillingTest(t *testing.T) *coreTestHelpers.ApiTestHelper {
	return coreTestHelpers.SetupApiTest(t)
}
