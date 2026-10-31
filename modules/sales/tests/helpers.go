//go:build integration
// +build integration

package sales_test

import (
	"context"
	"testing"
	"time"

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

// expireBatch moves a lot's expiry date to last month: the API refuses a past date, and a lot only gets
// there by waiting.
func expireBatch(t *testing.T, helper *coreTestHelpers.ApiTestHelper, batchID string) {
	t.Helper()
	if _, err := helper.DB().Execute(context.Background(),
		`UPDATE inventory.product_batches SET expiry_date = $1 WHERE id = $2`, time.Now().AddDate(0, -1, 0), batchID); err != nil {
		t.Fatalf("expire batch: %v", err)
	}
}
