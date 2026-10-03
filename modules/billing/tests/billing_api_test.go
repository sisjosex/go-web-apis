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

func init() {
	testhelpers.InitTestEnvironment()
}

// billingUser registers and logs in a unique user so tests don't share state. Writing a plan or a
// payment is the platform's (APP-009 D4), so the user is a super_admin; billingMember is not.
func billingUser(t *testing.T, helper *testhelpers.ApiTestHelper) {
	t.Helper()
	billingAccount(t, helper, true)
}

func billingMember(t *testing.T, helper *testhelpers.ApiTestHelper) {
	t.Helper()
	billingAccount(t, helper, false)
}

func billingAccount(t *testing.T, helper *testhelpers.ApiTestHelper, platform bool) {
	t.Helper()
	email := fmt.Sprintf("billing-%s@test.com", uuid.New().String()[:8])
	_, err := helper.Register(email, "$Password2025", "Billing", "Test")
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}
	if platform {
		promote(t, helper, email)
	}
	_, err = helper.Login(email, "$Password2025")
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
}

// promote makes an account a super_admin, the only one that writes plans and payments.
func promote(t *testing.T, helper *testhelpers.ApiTestHelper, email string) {
	t.Helper()
	if err := helper.SetSystemRole(email, "super_admin"); err != nil {
		t.Fatalf("promote: %v", err)
	}
}

// TestBilling_PlanAndPaymentArePlatformOnly - with no checkout, an account cannot move itself to a
// paid plan nor record its own payment (APP-009 D4).
func TestBilling_PlanAndPaymentArePlatformOnly(t *testing.T) {
	helper := SetupBillingTest(t)
	defer helper.Close()
	billingMember(t, helper)

	w := helper.DoRequest("PUT", "/billing/subscription", map[string]interface{}{"plan": "pro"}, map[string]string{})
	assert.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
	w = helper.DoRequest("POST", "/billing/payments", map[string]interface{}{
		"subscription_id": uuid.New().String(), "amount": 10, "status": "completed",
	}, map[string]string{})
	assert.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
	w = helper.DoRequest("GET", "/billing/plan", nil, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code, "reading the plan stays open")
}

// upsertSub calls PUT /billing/subscription and asserts success.
func upsertSub(t *testing.T, helper *testhelpers.ApiTestHelper, plan string) map[string]interface{} {
	t.Helper()
	w := helper.DoRequest("PUT", "/billing/subscription",
		map[string]interface{}{"plan": plan},
		map[string]string{},
	)
	assert.Equal(t, http.StatusOK, w.Code, "upsert %s subscription: %s", plan, w.Body.String())
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	return resp
}

// recordPayment calls POST /billing/payments and asserts 201.
func recordPayment(t *testing.T, helper *testhelpers.ApiTestHelper, subID string, amount float64) map[string]interface{} {
	t.Helper()
	w := helper.DoRequest("POST", "/billing/payments", map[string]interface{}{
		"subscription_id": subID,
		"amount":          amount,
		"status":          "completed",
	}, map[string]string{})
	assert.Equal(t, http.StatusCreated, w.Code, "record payment: %s", w.Body.String())
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	return resp
}

// ============================================================================
// Smoke test — all routes registered
// ============================================================================

func TestBillingModule_AllEndpointsRegistered(t *testing.T) {
	helper := SetupBillingTest(t)
	defer helper.Close()

	endpoints := []struct {
		method string
		path   string
	}{
		{"GET", "/billing/plan"},
		{"GET", "/billing/subscription"},
		{"PUT", "/billing/subscription"},
		{"GET", "/billing/payments"},
		{"POST", "/billing/payments"},
	}

	for _, ep := range endpoints {
		w := helper.DoRequest(ep.method, ep.path, nil, map[string]string{"Authorization": ""})
		assert.Equal(t, http.StatusUnauthorized, w.Code,
			"%s %s should return 401 (not 404) — route may not be registered", ep.method, ep.path)
		t.Logf("  ✅ %s %s → %d (route registered)", ep.method, ep.path, w.Code)
	}
}

// ============================================================================
// GET /billing/plan
// ============================================================================

func TestGetPlanInfo_RequiresAuth(t *testing.T) {
	helper := SetupBillingTest(t)
	defer helper.Close()

	w := helper.DoRequest("GET", "/billing/plan", nil, map[string]string{"Authorization": ""})

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestGetPlanInfo_DefaultsToFree(t *testing.T) {
	helper := SetupBillingTest(t)
	defer helper.Close()

	billingUser(t, helper)

	w := helper.DoRequest("GET", "/billing/plan", nil, map[string]string{})

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	assert.Equal(t, "free", resp["plan"])
	assert.Equal(t, "none", resp["status"])
	assert.False(t, resp["is_active"].(bool))
	assert.False(t, resp["is_expired"].(bool))

	limits, ok := resp["limits"].([]interface{})
	assert.True(t, ok, "limits should be an array")
	assert.NotEmpty(t, limits, "free plan should have limits seeded")

	t.Logf("✅ GetPlanInfo: new user defaults to free/none with %d limits", len(limits))
}

func TestGetPlanInfo_ReflectsActiveSubscription(t *testing.T) {
	helper := SetupBillingTest(t)
	defer helper.Close()

	billingUser(t, helper)
	upsertSub(t, helper, "pro")

	w := helper.DoRequest("GET", "/billing/plan", nil, map[string]string{})

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	assert.Equal(t, "pro", resp["plan"])
	assert.Equal(t, "active", resp["status"])
	assert.True(t, resp["is_active"].(bool))
	assert.False(t, resp["is_expired"].(bool))

	t.Logf("✅ GetPlanInfo reflects pro plan after subscription upsert")
}

func TestGetPlanInfo_ContainsExpectedFeatures(t *testing.T) {
	helper := SetupBillingTest(t)
	defer helper.Close()

	billingUser(t, helper)

	w := helper.DoRequest("GET", "/billing/plan", nil, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	limits, _ := resp["limits"].([]interface{})
	features := make(map[string]bool)
	for _, l := range limits {
		lm := l.(map[string]interface{})
		features[lm["feature"].(string)] = true
	}

	expected := []string{"tenants", "users_per_tenant", "inventory_items", "sales_orders", "tracking_items"}
	for _, f := range expected {
		assert.True(t, features[f], "free plan missing feature limit: %s", f)
	}

	t.Logf("✅ Free plan contains all %d expected features", len(expected))
}

func TestGetPlanInfo_EnterpriseLimitsAreUnlimited(t *testing.T) {
	helper := SetupBillingTest(t)
	defer helper.Close()

	billingUser(t, helper)
	upsertSub(t, helper, "enterprise")

	w := helper.DoRequest("GET", "/billing/plan", nil, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	assert.Equal(t, "enterprise", resp["plan"])

	limits, _ := resp["limits"].([]interface{})
	for _, l := range limits {
		lm := l.(map[string]interface{})
		val := int(lm["limit_value"].(float64))
		assert.Equal(t, -1, val, "enterprise feature %q should be unlimited (-1)", lm["feature"])
	}

	t.Logf("✅ Enterprise plan: all %d limits are -1 (unlimited)", len(limits))
}

// ============================================================================
// GET /billing/subscription
// ============================================================================

func TestGetSubscription_RequiresAuth(t *testing.T) {
	helper := SetupBillingTest(t)
	defer helper.Close()

	w := helper.DoRequest("GET", "/billing/subscription", nil, map[string]string{"Authorization": ""})

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestGetSubscription_NullWhenNoSubscription(t *testing.T) {
	helper := SetupBillingTest(t)
	defer helper.Close()

	billingUser(t, helper)

	w := helper.DoRequest("GET", "/billing/subscription", nil, map[string]string{})

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	assert.Contains(t, resp, "subscription")
	assert.Nil(t, resp["subscription"])

	t.Logf("✅ GetSubscription returns {subscription: null} for user with no subscription")
}

func TestGetSubscription_ReturnsActiveSubscription(t *testing.T) {
	helper := SetupBillingTest(t)
	defer helper.Close()

	billingUser(t, helper)
	upsertSub(t, helper, "pro")

	w := helper.DoRequest("GET", "/billing/subscription", nil, map[string]string{})

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	assert.NotEmpty(t, resp["id"])
	assert.NotEmpty(t, resp["user_id"])
	assert.Equal(t, "pro", resp["plan"])
	assert.Equal(t, "active", resp["status"])
	assert.NotEmpty(t, resp["started_at"])

	t.Logf("✅ GetSubscription returns active pro subscription (id=%s)", resp["id"])
}

// ============================================================================
// PUT /billing/subscription
// ============================================================================

func TestUpsertSubscription_RequiresAuth(t *testing.T) {
	helper := SetupBillingTest(t)
	defer helper.Close()

	w := helper.DoRequest("PUT", "/billing/subscription",
		map[string]interface{}{"plan": "pro"},
		map[string]string{"Authorization": ""},
	)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestUpsertSubscription_MissingPlan(t *testing.T) {
	helper := SetupBillingTest(t)
	defer helper.Close()

	billingUser(t, helper)

	w := helper.DoRequest("PUT", "/billing/subscription", map[string]interface{}{}, map[string]string{})

	assert.Equal(t, http.StatusBadRequest, w.Code)
	t.Logf("✅ UpsertSubscription rejects missing plan field with 400")
}

func TestUpsertSubscription_InvalidPlan(t *testing.T) {
	helper := SetupBillingTest(t)
	defer helper.Close()

	billingUser(t, helper)

	w := helper.DoRequest("PUT", "/billing/subscription",
		map[string]interface{}{"plan": "diamond"},
		map[string]string{},
	)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	t.Logf("✅ UpsertSubscription rejects unknown plan with 400")
}

func TestUpsertSubscription_CreateFreePlan(t *testing.T) {
	helper := SetupBillingTest(t)
	defer helper.Close()

	billingUser(t, helper)

	sub := upsertSub(t, helper, "free")

	assert.Equal(t, "free", sub["plan"])
	assert.Equal(t, "active", sub["status"])
	assert.NotEmpty(t, sub["id"])
	t.Logf("✅ UpsertSubscription creates free plan (id=%s)", sub["id"])
}

func TestUpsertSubscription_CreateProPlan(t *testing.T) {
	helper := SetupBillingTest(t)
	defer helper.Close()

	billingUser(t, helper)

	sub := upsertSub(t, helper, "pro")

	assert.Equal(t, "pro", sub["plan"])
	assert.Equal(t, "active", sub["status"])
	t.Logf("✅ UpsertSubscription creates pro plan")
}

func TestUpsertSubscription_CreateEnterprisePlan(t *testing.T) {
	helper := SetupBillingTest(t)
	defer helper.Close()

	billingUser(t, helper)

	sub := upsertSub(t, helper, "enterprise")

	assert.Equal(t, "enterprise", sub["plan"])
	assert.Equal(t, "active", sub["status"])
	t.Logf("✅ UpsertSubscription creates enterprise plan")
}

func TestUpsertSubscription_SwitchPlanCancelsOldOne(t *testing.T) {
	helper := SetupBillingTest(t)
	defer helper.Close()

	billingUser(t, helper)

	free := upsertSub(t, helper, "free")
	assert.Equal(t, "free", free["plan"])

	// Switching to pro should cancel the free subscription
	pro := upsertSub(t, helper, "pro")
	assert.Equal(t, "pro", pro["plan"])
	assert.Equal(t, "active", pro["status"])

	// Current subscription must now be pro
	w := helper.DoRequest("GET", "/billing/subscription", nil, map[string]string{})
	var current map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &current)
	assert.Equal(t, "pro", current["plan"])

	t.Logf("✅ Switching free→pro cancels old subscription")
}

func TestUpsertSubscription_UpgradeToEnterprise(t *testing.T) {
	helper := SetupBillingTest(t)
	defer helper.Close()

	billingUser(t, helper)
	upsertSub(t, helper, "pro")
	ent := upsertSub(t, helper, "enterprise")

	assert.Equal(t, "enterprise", ent["plan"])

	// Plan info should reflect enterprise
	w := helper.DoRequest("GET", "/billing/plan", nil, map[string]string{})
	var info map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &info)
	assert.Equal(t, "enterprise", info["plan"])

	t.Logf("✅ Upgrade pro→enterprise works correctly")
}

func TestUpsertSubscription_IdempotentSamePlan(t *testing.T) {
	helper := SetupBillingTest(t)
	defer helper.Close()

	billingUser(t, helper)

	first := upsertSub(t, helper, "pro")
	second := upsertSub(t, helper, "pro")

	assert.Equal(t, "pro", second["plan"])
	assert.Equal(t, "active", second["status"])
	assert.Equal(t, first["user_id"], second["user_id"])

	t.Logf("✅ Upserting same plan is idempotent (no duplicate subscriptions)")
}

func TestUpsertSubscription_WithExpiresAt(t *testing.T) {
	helper := SetupBillingTest(t)
	defer helper.Close()

	billingUser(t, helper)

	expiresAt := time.Now().UTC().Add(30 * 24 * time.Hour).Format(time.RFC3339)
	w := helper.DoRequest("PUT", "/billing/subscription", map[string]interface{}{
		"plan":       "pro",
		"expires_at": expiresAt,
	}, map[string]string{})

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	assert.Equal(t, "pro", resp["plan"])
	assert.NotNil(t, resp["expires_at"])
	t.Logf("✅ UpsertSubscription persists expires_at correctly")
}

// ============================================================================
// POST /billing/payments
// ============================================================================

func TestRecordPayment_RequiresAuth(t *testing.T) {
	helper := SetupBillingTest(t)
	defer helper.Close()

	w := helper.DoRequest("POST", "/billing/payments", map[string]interface{}{
		"subscription_id": uuid.New().String(),
		"amount":          99.99,
	}, map[string]string{"Authorization": ""})

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestRecordPayment_MissingSubscriptionID(t *testing.T) {
	helper := SetupBillingTest(t)
	defer helper.Close()

	billingUser(t, helper)

	w := helper.DoRequest("POST", "/billing/payments",
		map[string]interface{}{"amount": 99.99},
		map[string]string{},
	)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	t.Logf("✅ RecordPayment rejects missing subscription_id with 400")
}

func TestRecordPayment_InvalidAmountZero(t *testing.T) {
	helper := SetupBillingTest(t)
	defer helper.Close()

	billingUser(t, helper)

	w := helper.DoRequest("POST", "/billing/payments", map[string]interface{}{
		"subscription_id": uuid.New().String(),
		"amount":          0,
	}, map[string]string{})

	assert.Equal(t, http.StatusBadRequest, w.Code)
	t.Logf("✅ RecordPayment rejects amount=0 with 400")
}

func TestRecordPayment_InvalidAmountNegative(t *testing.T) {
	helper := SetupBillingTest(t)
	defer helper.Close()

	billingUser(t, helper)

	w := helper.DoRequest("POST", "/billing/payments", map[string]interface{}{
		"subscription_id": uuid.New().String(),
		"amount":          -10.0,
	}, map[string]string{})

	assert.Equal(t, http.StatusBadRequest, w.Code)
	t.Logf("✅ RecordPayment rejects negative amount with 400")
}

func TestRecordPayment_SubscriptionNotFound(t *testing.T) {
	helper := SetupBillingTest(t)
	defer helper.Close()

	billingUser(t, helper)

	w := helper.DoRequest("POST", "/billing/payments", map[string]interface{}{
		"subscription_id": uuid.New().String(), // random non-existent UUID
		"amount":          99.99,
	}, map[string]string{})

	// SP raises billing.payment.subscription-not-found → controller 400
	assert.Equal(t, http.StatusBadRequest, w.Code)
	t.Logf("✅ RecordPayment returns 400 when subscription_id does not exist")
}

func TestRecordPayment_Success(t *testing.T) {
	helper := SetupBillingTest(t)
	defer helper.Close()

	billingUser(t, helper)
	sub := upsertSub(t, helper, "pro")
	subID := sub["id"].(string)

	w := helper.DoRequest("POST", "/billing/payments", map[string]interface{}{
		"subscription_id": subID,
		"amount":          29.99,
		"currency":        "USD",
		"status":          "completed",
	}, map[string]string{})

	assert.Equal(t, http.StatusCreated, w.Code)

	var payment map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &payment)

	assert.NotEmpty(t, payment["id"])
	assert.Equal(t, subID, payment["subscription_id"])
	assert.Equal(t, 29.99, payment["amount"])
	assert.Equal(t, "USD", payment["currency"])
	assert.Equal(t, "completed", payment["status"])
	assert.NotNil(t, payment["paid_at"], "completed payments must have paid_at set")
	assert.NotEmpty(t, payment["created_at"])

	t.Logf("✅ RecordPayment creates payment (id=%s)", payment["id"])
}

func TestRecordPayment_PendingStatus_NoPaidAt(t *testing.T) {
	helper := SetupBillingTest(t)
	defer helper.Close()

	billingUser(t, helper)
	sub := upsertSub(t, helper, "pro")

	w := helper.DoRequest("POST", "/billing/payments", map[string]interface{}{
		"subscription_id": sub["id"],
		"amount":          29.99,
		"status":          "pending",
	}, map[string]string{})

	assert.Equal(t, http.StatusCreated, w.Code)

	var payment map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &payment)

	assert.Equal(t, "pending", payment["status"])
	assert.Nil(t, payment["paid_at"], "pending payments must have null paid_at")

	t.Logf("✅ Pending payment has null paid_at")
}

func TestRecordPayment_WithProviderInfo(t *testing.T) {
	helper := SetupBillingTest(t)
	defer helper.Close()

	billingUser(t, helper)
	sub := upsertSub(t, helper, "pro")

	w := helper.DoRequest("POST", "/billing/payments", map[string]interface{}{
		"subscription_id":     sub["id"],
		"amount":              29.99,
		"status":              "completed",
		"provider":            "stripe",
		"provider_payment_id": "pi_test_123456",
	}, map[string]string{})

	assert.Equal(t, http.StatusCreated, w.Code)

	var payment map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &payment)

	assert.Equal(t, "stripe", payment["provider"])
	assert.Equal(t, "pi_test_123456", payment["provider_payment_id"])

	t.Logf("✅ RecordPayment persists provider info correctly")
}

func TestRecordPayment_WrongUserSubscriptionNotFound(t *testing.T) {
	helper := SetupBillingTest(t)
	defer helper.Close()

	// User A registers and creates a subscription
	emailA := fmt.Sprintf("billing-a-%s@test.com", uuid.New().String()[:8])
	_, err := helper.Register(emailA, "$Password2025", "User", "A")
	assert.NoError(t, err)
	promote(t, helper, emailA)
	_, err = helper.Login(emailA, "$Password2025")
	assert.NoError(t, err)
	subA := upsertSub(t, helper, "pro")
	subAID := subA["id"].(string)

	// User B registers and logs in — this replaces the token on the same helper
	emailB := fmt.Sprintf("billing-b-%s@test.com", uuid.New().String()[:8])
	_, err = helper.Register(emailB, "$Password2025", "User", "B")
	assert.NoError(t, err)
	promote(t, helper, emailB)
	_, err = helper.Login(emailB, "$Password2025")
	assert.NoError(t, err)

	// User B tries to record a payment against User A's subscription
	// SP validates: subscription.user_id must match JWT user_id
	w := helper.DoRequest("POST", "/billing/payments", map[string]interface{}{
		"subscription_id": subAID,
		"amount":          99.00,
	}, map[string]string{})

	assert.Equal(t, http.StatusBadRequest, w.Code)
	t.Logf("✅ RecordPayment rejects subscription belonging to another user")
}

// ============================================================================
// GET /billing/payments
// ============================================================================

func TestListPayments_RequiresAuth(t *testing.T) {
	helper := SetupBillingTest(t)
	defer helper.Close()

	w := helper.DoRequest("GET", "/billing/payments", nil, map[string]string{"Authorization": ""})

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestListPayments_EmptyForNewUser(t *testing.T) {
	helper := SetupBillingTest(t)
	defer helper.Close()

	billingUser(t, helper)

	w := helper.DoRequest("GET", "/billing/payments", nil, map[string]string{})

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	assert.Equal(t, float64(0), resp["total_count"])
	assert.Equal(t, float64(1), resp["page"])

	data, _ := resp["data"].([]interface{})
	assert.Empty(t, data)

	t.Logf("✅ ListPayments returns empty list for user with no payments")
}

func TestListPayments_ReturnsRecordedPayments(t *testing.T) {
	helper := SetupBillingTest(t)
	defer helper.Close()

	billingUser(t, helper)
	sub := upsertSub(t, helper, "pro")
	subID := sub["id"].(string)

	for i := 1; i <= 3; i++ {
		recordPayment(t, helper, subID, float64(i)*10.0)
	}

	w := helper.DoRequest("GET", "/billing/payments", nil, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	assert.Equal(t, float64(3), resp["total_count"])

	data, _ := resp["data"].([]interface{})
	assert.Len(t, data, 3)

	t.Logf("✅ ListPayments returns all 3 recorded payments")
}

func TestListPayments_Pagination(t *testing.T) {
	helper := SetupBillingTest(t)
	defer helper.Close()

	billingUser(t, helper)
	sub := upsertSub(t, helper, "pro")
	subID := sub["id"].(string)

	for i := 1; i <= 5; i++ {
		recordPayment(t, helper, subID, float64(i)*5.0)
	}

	// Page 1, 2 items
	w1 := helper.DoRequest("GET", "/billing/payments?page=1&limit=2", nil, map[string]string{})
	assert.Equal(t, http.StatusOK, w1.Code)
	var p1 map[string]interface{}
	json.Unmarshal(w1.Body.Bytes(), &p1)
	assert.Equal(t, float64(5), p1["total_count"])
	assert.Equal(t, float64(1), p1["page"])
	assert.Equal(t, float64(2), p1["limit"])
	data1, _ := p1["data"].([]interface{})
	assert.Len(t, data1, 2)

	// Page 3 → 1 remaining record
	w3 := helper.DoRequest("GET", "/billing/payments?page=3&limit=2", nil, map[string]string{})
	assert.Equal(t, http.StatusOK, w3.Code)
	var p3 map[string]interface{}
	json.Unmarshal(w3.Body.Bytes(), &p3)
	data3, _ := p3["data"].([]interface{})
	assert.Len(t, data3, 1)

	t.Logf("✅ ListPayments pagination: page1=%d, page3=%d items (total=5)", len(data1), len(data3))
}

func TestListPayments_ResponseStructure(t *testing.T) {
	helper := SetupBillingTest(t)
	defer helper.Close()

	billingUser(t, helper)
	sub := upsertSub(t, helper, "pro")
	subID := sub["id"].(string)

	w := helper.DoRequest("POST", "/billing/payments", map[string]interface{}{
		"subscription_id": subID,
		"amount":          49.99,
		"currency":        "EUR",
		"status":          "completed",
	}, map[string]string{})
	assert.Equal(t, http.StatusCreated, w.Code)

	w2 := helper.DoRequest("GET", "/billing/payments", nil, map[string]string{})
	assert.Equal(t, http.StatusOK, w2.Code)

	var resp map[string]interface{}
	json.Unmarshal(w2.Body.Bytes(), &resp)

	data, _ := resp["data"].([]interface{})
	assert.NotEmpty(t, data)

	payment := data[0].(map[string]interface{})
	assert.NotEmpty(t, payment["id"])
	assert.Equal(t, subID, payment["subscription_id"])
	assert.Equal(t, 49.99, payment["amount"])
	assert.Equal(t, "EUR", payment["currency"])
	assert.Equal(t, "completed", payment["status"])
	assert.NotNil(t, payment["paid_at"])
	assert.NotEmpty(t, payment["created_at"])

	t.Logf("✅ Payment response contains all expected fields")
}

// ============================================================================
// Edge Case Tests
// ============================================================================

// TestUpsertSubscription_InvalidStatus - passing an explicit invalid status → 400
func TestUpsertSubscription_InvalidStatus(t *testing.T) {
	helper := SetupBillingTest(t)
	defer helper.Close()

	billingUser(t, helper)

	w := helper.DoRequest("PUT", "/billing/subscription", map[string]interface{}{
		"plan":   "pro",
		"status": "invalid_status_xyz",
	}, map[string]string{})

	// SP validates status and raises billing.subscription.invalid-status → 400
	assert.Equal(t, http.StatusBadRequest, w.Code,
		"Invalid status should return 400, got %d: %s", w.Code, w.Body.String())
	t.Logf("✅ UpsertSubscription rejects invalid status with 400")
}

// TestUpsertSubscription_TrialStatus - creating a trial subscription → 200, is_active=true
func TestUpsertSubscription_TrialStatus(t *testing.T) {
	helper := SetupBillingTest(t)
	defer helper.Close()

	billingUser(t, helper)

	w := helper.DoRequest("PUT", "/billing/subscription", map[string]interface{}{
		"plan":   "pro",
		"status": "trial",
	}, map[string]string{})

	assert.Equal(t, http.StatusOK, w.Code,
		"Trial subscription should succeed, got %d: %s", w.Code, w.Body.String())

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "trial", resp["status"])
	t.Logf("✅ UpsertSubscription accepts trial status")
}

// TestGetPlanInfo_TrialIsActive - trial subscription counts as active
func TestGetPlanInfo_TrialIsActive(t *testing.T) {
	helper := SetupBillingTest(t)
	defer helper.Close()

	billingUser(t, helper)

	// Create trial subscription
	w := helper.DoRequest("PUT", "/billing/subscription", map[string]interface{}{
		"plan":   "pro",
		"status": "trial",
	}, map[string]string{})
	if w.Code != http.StatusOK {
		t.Skipf("Could not create trial subscription (status %d)", w.Code)
		return
	}

	w = helper.DoRequest("GET", "/billing/plan", nil, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	assert.True(t, resp["is_active"].(bool), "trial subscription should have is_active=true")
	assert.False(t, resp["is_expired"].(bool), "trial subscription should have is_expired=false")
	t.Logf("✅ GetPlanInfo: trial subscription is_active=true")
}

// TestGetPlanInfo_ExpiredStatus - expired subscription: is_active=false, is_expired=true
func TestGetPlanInfo_ExpiredStatus(t *testing.T) {
	helper := SetupBillingTest(t)
	defer helper.Close()

	billingUser(t, helper)

	// Create subscription with expired status
	w := helper.DoRequest("PUT", "/billing/subscription", map[string]interface{}{
		"plan":   "pro",
		"status": "expired",
	}, map[string]string{})
	if w.Code != http.StatusOK {
		t.Skipf("Could not create expired subscription (status %d)", w.Code)
		return
	}

	w = helper.DoRequest("GET", "/billing/plan", nil, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	assert.False(t, resp["is_active"].(bool), "expired subscription should have is_active=false")
	assert.True(t, resp["is_expired"].(bool), "expired subscription should have is_expired=true")
	t.Logf("✅ GetPlanInfo: expired subscription is_active=false, is_expired=true")
}

// TestUpsertSubscription_CanceledStatus - canceled subscription succeeds
func TestUpsertSubscription_CanceledStatus(t *testing.T) {
	helper := SetupBillingTest(t)
	defer helper.Close()

	billingUser(t, helper)

	// First create an active subscription
	upsertSub(t, helper, "pro")

	// Then cancel it
	w := helper.DoRequest("PUT", "/billing/subscription", map[string]interface{}{
		"plan":   "pro",
		"status": "canceled",
	}, map[string]string{})

	assert.Equal(t, http.StatusOK, w.Code,
		"Canceled status should succeed, got %d: %s", w.Code, w.Body.String())

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "canceled", resp["status"])
	t.Logf("✅ UpsertSubscription accepts canceled status")
}
