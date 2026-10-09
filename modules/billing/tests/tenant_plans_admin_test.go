//go:build integration
// +build integration

package billing_test

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"josex/web/modules/core/testhelpers"

	"github.com/stretchr/testify/assert"
)

// BILLING-002: the checkout with no field to type, and the platform's queue, list and hand adjustment.

func notice(t *testing.T, helper *testhelpers.ApiTestHelper, cycle string) map[string]interface{} {
	t.Helper()
	w := helper.DoRequest("POST", "/billing/payments/notice", map[string]interface{}{"plan": "pro", "cycle": cycle}, map[string]string{})
	if w.Code != http.StatusCreated {
		t.Fatalf("notice: %d %s", w.Code, w.Body.String())
	}
	return body(t, w.Body.Bytes())
}

// asPlatform makes the member a super_admin and signs them in again, so the token says so.
func asPlatform(t *testing.T, helper *testhelpers.ApiTestHelper, email string) {
	t.Helper()
	promote(t, helper, email)
	if _, err := helper.Login(email, "$Password2025"); err != nil {
		t.Fatalf("login: %v", err)
	}
}

// TestNotice_CodeAndOnePending - D2: "Ya pagué" sends no reference and the business's code is stored;
// a second notice while the first is under review updates it; the plan read shows the code and the notice.
func TestNotice_CodeAndOnePending(t *testing.T) {
	helper := SetupBillingTest(t)
	defer helper.Close()
	slug, _ := ownBusiness(t, helper)

	first := notice(t, helper, "monthly")
	assert.Equal(t, strings.ToUpper(slug), first["reference"])
	second := notice(t, helper, "annual")
	assert.Equal(t, first["id"], second["id"], "one pending notice per business")
	assert.EqualValues(t, 2000, second["amount"])

	w := helper.DoRequest("GET", "/billing/tenant/plan", nil, map[string]string{})
	plan := body(t, w.Body.Bytes())
	assert.Equal(t, strings.ToUpper(slug), plan["payment_code"])
	pending, _ := plan["pending_payment"].(map[string]interface{})
	if assert.NotNil(t, pending) {
		assert.Equal(t, first["id"], pending["id"])
		assert.Equal(t, "annual", pending["cycle"])
	}
	price := plan["prices"].([]interface{})[0].(map[string]interface{})
	_, hasQR := price["qr_image_url"]
	assert.True(t, hasQR, "every price says its QR, null when it has none")
}

// TestReject_LeavesThePlan - D3: the platform turns a notice down with a reason; it is no longer
// pending, the plan is unchanged, and it cannot be turned down twice.
func TestReject_LeavesThePlan(t *testing.T) {
	helper := SetupBillingTest(t)
	defer helper.Close()
	_, email := ownBusiness(t, helper)
	payment := notice(t, helper, "monthly")
	path := fmt.Sprintf("/billing/payments/%s/reject", payment["id"])

	w := helper.DoRequest("POST", path, map[string]interface{}{"reason": "No llegó la transferencia"}, map[string]string{})
	assert.Equal(t, http.StatusForbidden, w.Code, "only the platform rejects")

	asPlatform(t, helper, email)
	w = helper.DoRequest("POST", path, map[string]interface{}{"reason": "No llegó la transferencia"}, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, "rejected", body(t, w.Body.Bytes())["status"])

	w = helper.DoRequest("POST", path, map[string]interface{}{"reason": "Otra vez"}, map[string]string{})
	assert.Equal(t, http.StatusConflict, w.Code)

	plan := body(t, helper.DoRequest("GET", "/billing/tenant/plan", nil, map[string]string{}).Body.Bytes())
	assert.Nil(t, plan["pending_payment"])
	assert.Equal(t, "free", plan["effective_plan"])
}

// TestAdjust_RecordedWithReason - D3: the businesses list finds the business; the platform sets its plan
// by hand with a reason, which is recorded once; a member who is not the platform is refused.
func TestAdjust_RecordedWithReason(t *testing.T) {
	helper := SetupBillingTest(t)
	defer helper.Close()
	slug, email := ownBusiness(t, helper)

	w := helper.DoRequest("GET", "/billing/admin/subscriptions?search="+slug, nil, map[string]string{})
	assert.Equal(t, http.StatusForbidden, w.Code)

	asPlatform(t, helper, email)
	w = helper.DoRequest("GET", "/billing/admin/subscriptions?search="+slug, nil, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	list := body(t, w.Body.Bytes())
	rows := list["subscriptions"].([]interface{})
	if !assert.Len(t, rows, 1) {
		return
	}
	row := rows[0].(map[string]interface{})
	assert.Equal(t, "free", row["plan"])
	tenantID := row["tenant_id"].(string)

	end := time.Now().AddDate(0, 3, 0).UTC().Format(time.RFC3339)
	adjust := map[string]interface{}{"plan": "pro", "cycle": "monthly", "status": "active", "period_end": end, "reason": ""}
	w = helper.DoRequest("PUT", "/billing/admin/tenants/"+tenantID+"/subscription", adjust, map[string]string{})
	assert.Equal(t, http.StatusBadRequest, w.Code, "a reason is required")

	adjust["reason"] = "Cortesía por la migración"
	w = helper.DoRequest("PUT", "/billing/admin/tenants/"+tenantID+"/subscription", adjust, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, "pro", body(t, w.Body.Bytes())["effective_plan"])

	var adjustments int
	// check:raw-sql the adjustment rows have no read endpoint yet (BILLING-002 Out: no history view)
	err := helper.DB().QueryRow(context.Background(),
		`SELECT count(*) FROM billing.subscription_adjustments WHERE tenant_id = $1 AND reason = $2`,
		tenantID, "Cortesía por la migración").Scan(&adjustments)
	if assert.NoError(t, err) {
		assert.Equal(t, 1, adjustments)
	}
}
