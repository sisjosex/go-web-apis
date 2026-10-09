//go:build integration
// +build integration

package billing_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"josex/web/modules/billing/jobs"
	billingRepos "josex/web/modules/billing/repositories"
	billingServices "josex/web/modules/billing/services"
	"josex/web/modules/core/testhelpers"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// INFRA-014: the daily billing:expire job — a paid plan past its end plus the grace falls back to free;
// one still inside the grace keeps its plan.

// paidUntil gives a fresh business a pro plan ending at end, set by hand as the platform does, and
// answers a helper signed in as its owner.
func paidUntil(t *testing.T, end time.Time) *testhelpers.ApiTestHelper {
	t.Helper()
	helper := SetupBillingTest(t)
	slug, email := ownBusiness(t, helper)
	asPlatform(t, helper, email)

	w := helper.DoRequest("GET", "/billing/admin/subscriptions?search="+slug, nil, map[string]string{})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	rows := body(t, w.Body.Bytes())["subscriptions"].([]interface{})
	require.Len(t, rows, 1)
	tenantID := rows[0].(map[string]interface{})["tenant_id"].(string)

	w = helper.DoRequest("PUT", "/billing/admin/tenants/"+tenantID+"/subscription", map[string]interface{}{
		"plan": "pro", "cycle": "monthly", "status": "active",
		"period_end": end.UTC().Format(time.RFC3339), "reason": "INFRA-014 expiry test",
	}, map[string]string{})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	return helper
}

func effectivePlan(t *testing.T, helper *testhelpers.ApiTestHelper) string {
	t.Helper()
	w := helper.DoRequest("GET", "/billing/tenant/plan", nil, map[string]string{})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	plan, _ := body(t, w.Body.Bytes())["effective_plan"].(string)
	return plan
}

// TestExpireHandler_LapsedPlanFallsBackToFree - ten days past its end the plan is expired and the
// business is back on free; three days past it, inside the seven-day grace, it is still pro.
func TestExpireHandler_LapsedPlanFallsBackToFree(t *testing.T) {
	lapsed := paidUntil(t, time.Now().AddDate(0, 0, -10))
	defer lapsed.Close()
	inGrace := paidUntil(t, time.Now().AddDate(0, 0, -3))
	defer inGrace.Close()

	service := billingServices.NewBillingService(billingRepos.NewBillingRepository(lapsed.DB()))
	require.NoError(t, jobs.ExpireHandler(service)(context.Background(), nil))

	assert.Equal(t, "free", effectivePlan(t, lapsed), "past the grace: back on the free limits")
	assert.Equal(t, "pro", effectivePlan(t, inGrace), "inside the grace: the plan holds")

	// Idempotent: a second run expires nothing more and changes nothing.
	require.NoError(t, jobs.ExpireHandler(service)(context.Background(), nil))
	assert.Equal(t, "pro", effectivePlan(t, inGrace))
}
