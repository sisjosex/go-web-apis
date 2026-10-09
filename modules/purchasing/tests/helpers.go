//go:build integration
// +build integration

package purchasing_test

import (
	"context"
	"testing"

	coreTestHelpers "josex/web/modules/core/testhelpers"
)

// testCompanyID is the seeded tenant (modules/tenancy/tests/fixtures/seed_test_tenant.sql).
const testCompanyID = "00000000-0000-0000-0000-000000000001"

// addRFQResponse records a supplier's quote on an RFQ: suppliers answer outside the API, so no
// endpoint creates one.
func addRFQResponse(t *testing.T, helper *coreTestHelpers.ApiTestHelper, rfqID, supplierID string, total float64) string {
	t.Helper()
	var id string
	err := helper.DB().QueryRow(context.Background(),
		`SELECT id FROM purchasing.sp_add_rfq_response($1, $2, $3, $4, 7, 30, NULL)`,
		testCompanyID, rfqID, supplierID, total,
	).Scan(&id)
	if err != nil {
		t.Fatalf("add rfq response: %v", err)
	}
	return id
}
