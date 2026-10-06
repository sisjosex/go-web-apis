//go:build integration
// +build integration

package billing_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"josex/web/modules/core/testhelpers"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// BILLING-001: plans per business, paid by QR and confirmed by the platform.

// ownBusiness has a new member create a workspace and answers its slug; the helper then sends it.
func ownBusiness(t *testing.T, helper *testhelpers.ApiTestHelper) (string, string) {
	t.Helper()
	email := fmt.Sprintf("plan-%s@test.com", uuid.NewString()[:8])
	if _, err := helper.Register(email, "$Password2025", "Plan", "Owner"); err != nil {
		t.Fatalf("register: %v", err)
	}
	if _, err := helper.Login(email, "$Password2025"); err != nil {
		t.Fatalf("login: %v", err)
	}
	slug := fmt.Sprintf("plan-%d", time.Now().UnixNano())
	w := helper.DoRequest("POST", "/tenants/self-service", map[string]interface{}{"slug": slug, "name": "Plan"}, map[string]string{})
	if w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Fatalf("create tenant: %d %s", w.Code, w.Body.String())
	}
	helper.SetTenantSlug(slug)
	return slug, email
}

func body(t *testing.T, raw []byte) map[string]interface{} {
	t.Helper()
	var out map[string]interface{}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode: %v: %s", err, raw)
	}
	return out
}

// TestTenantPlan_FreeWithPrices - a new business is on free, with the free limits and the price list.
func TestTenantPlan_FreeWithPrices(t *testing.T) {
	helper := SetupBillingTest(t)
	defer helper.Close()
	ownBusiness(t, helper)

	w := helper.DoRequest("GET", "/billing/tenant/plan", nil, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	plan := body(t, w.Body.Bytes())
	assert.Equal(t, "free", plan["effective_plan"])
	assert.EqualValues(t, 50, plan["limits"].(map[string]interface{})["tracking_riders"])
	assert.NotEmpty(t, plan["prices"])
}

// TestTenantPlan_NoticeThenConfirmExtendsByCycle - "Ya pagué" waits as notified; the platform's
// confirmation puts the business on pro until a year from now for an annual payment.
func TestTenantPlan_NoticeThenConfirmExtendsByCycle(t *testing.T) {
	helper := SetupBillingTest(t)
	defer helper.Close()
	_, email := ownBusiness(t, helper)

	w := helper.DoRequest("POST", "/billing/payments/notice", map[string]interface{}{
		"plan": "pro", "cycle": "annual", "reference": "QR-123456",
	}, map[string]string{})
	assert.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	payment := body(t, w.Body.Bytes())
	assert.Equal(t, "notified", payment["status"])
	assert.EqualValues(t, 2000, payment["amount"])

	w = helper.DoRequest("POST", fmt.Sprintf("/billing/payments/%s/confirm", payment["id"]), nil, map[string]string{})
	assert.Equal(t, http.StatusForbidden, w.Code, "only the platform confirms")

	promote(t, helper, email)
	if _, err := helper.Login(email, "$Password2025"); err != nil {
		t.Fatalf("login: %v", err)
	}
	w = helper.DoRequest("POST", fmt.Sprintf("/billing/payments/%s/confirm", payment["id"]), nil, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())

	w = helper.DoRequest("GET", "/billing/tenant/plan", nil, map[string]string{})
	plan := body(t, w.Body.Bytes())
	assert.Equal(t, "pro", plan["effective_plan"])
	assert.Equal(t, "annual", plan["cycle"])
	end, err := time.Parse(time.RFC3339, plan["period_end"].(string))
	if assert.NoError(t, err) {
		assert.WithinDuration(t, time.Now().AddDate(1, 0, 0), end, 48*time.Hour)
	}

	w = helper.DoRequest("POST", fmt.Sprintf("/billing/payments/%s/confirm", payment["id"]), nil, map[string]string{})
	assert.Equal(t, http.StatusConflict, w.Code, "a payment is confirmed once")
}

// TestTenantUsage_ListsEveryLimitedFeature - usage names riders, vehicles, products and members.
func TestTenantUsage_ListsEveryLimitedFeature(t *testing.T) {
	helper := SetupBillingTest(t)
	defer helper.Close()
	ownBusiness(t, helper)

	w := helper.DoRequest("GET", "/billing/usage", nil, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	features := map[string]float64{}
	for _, row := range body(t, w.Body.Bytes())["features"].([]interface{}) {
		f := row.(map[string]interface{})
		features[f["feature"].(string)] = f["used"].(float64)
	}
	assert.Contains(t, features, "tracking_riders")
	assert.EqualValues(t, 1, features["users_per_tenant"], "the owner")
}

// TestTenantLimit_VehicleRefusedAtFreeLimit - a free business creates its three vehicles; the fourth
// is 403 billing.limit-reached naming the feature and the limit (D2).
func TestTenantLimit_VehicleRefusedAtFreeLimit(t *testing.T) {
	helper := SetupBillingTest(t)
	defer helper.Close()
	slug, _ := ownBusiness(t, helper)

	w := helper.DoRequest("PUT", "/tenants/"+slug+"/modules/tracking", map[string]interface{}{"is_enabled": true}, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	w = helper.DoRequest("POST", "/tracking/companies", map[string]interface{}{
		"name": "Transportes Plan", "phone": "70000000", "address": "Av. Heroínas 1",
	}, map[string]string{})
	if !assert.Equal(t, http.StatusCreated, w.Code, w.Body.String()) {
		return
	}
	company := body(t, w.Body.Bytes())["id"]

	vehicle := func(n int) int {
		w := helper.DoRequest("POST", "/tracking/vehicles", map[string]interface{}{
			"company_id": company, "plate_number": fmt.Sprintf("PLN-%d%s", n, uuid.NewString()[:4]),
			"vehicle_type": "van", "capacity": 12, "status": "active",
		}, map[string]string{})
		if n == 4 {
			refusal := body(t, w.Body.Bytes())
			assert.Equal(t, "billing.limit-reached", refusal["error"])
			detail := refusal["detail"].(map[string]interface{})
			assert.Equal(t, "tracking_vehicles", detail["feature"])
			assert.EqualValues(t, 3, detail["limit"])
		}
		return w.Code
	}
	for n := 1; n <= 3; n++ {
		assert.Equal(t, http.StatusCreated, vehicle(n))
	}
	assert.Equal(t, http.StatusForbidden, vehicle(4))
}
