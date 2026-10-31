//go:build integration
// +build integration

package tracking_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"josex/web/modules/core/testhelpers"
)

// issueRiderAccessCode gives riderID a code and answers it.
func issueRiderAccessCode(t *testing.T, helper *testhelpers.ApiTestHelper, riderID string) string {
	t.Helper()
	w := helper.DoRequest("POST", "/tracking/riders/"+riderID+"/access-code", nil, map[string]string{})
	if w.Code != http.StatusOK {
		t.Fatalf("issue access code: %d %s", w.Code, w.Body.String())
	}
	var code struct {
		Code string `json:"code"`
	}
	decodeBody(t, w, &code)
	return code.Code
}

// TestRiderAccessCode_OneUse - a code readies the account and is used once; the card's code opens nothing.
func TestRiderAccessCode_OneUse(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	riderID := CreateTestRider(t, helper)
	code := issueRiderAccessCode(t, helper, riderID)
	email := map[string]string{"email": "qr-passenger-one@test.local"}

	w := helper.DoRequest("POST", "/public/rider-access/"+code+"/check", email, map[string]string{})
	assert.Equal(t, http.StatusNoContent, w.Code, w.Body.String())

	w = helper.DoRequest("POST", "/public/rider-access/"+code+"/redeem", nil, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())

	w = helper.DoRequest("POST", "/public/rider-access/"+code+"/redeem", nil, map[string]string{})
	assert.Equal(t, http.StatusConflict, w.Code)
	w = helper.DoRequest("POST", "/public/rider-access/"+code+"/check", email, map[string]string{})
	assert.Equal(t, http.StatusConflict, w.Code)

	w = helper.DoRequest("GET", "/tracking/riders/"+riderID+"/qr-token", nil, map[string]string{})
	var card struct {
		QRToken string `json:"qr_token"`
	}
	decodeBody(t, w, &card)
	w = helper.DoRequest("POST", fmt.Sprintf("/public/rider-access/%s/check", card.QRToken), email, map[string]string{})
	assert.Equal(t, http.StatusNotFound, w.Code, "a card's code opens nothing")
}

// TestRiderAccessCode_RevokedAndReplaced - a revoked code and one a newer code replaced both answer 410.
func TestRiderAccessCode_RevokedAndReplaced(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	riderID := CreateTestRider(t, helper)
	first := issueRiderAccessCode(t, helper, riderID)
	second := issueRiderAccessCode(t, helper, riderID)

	w := helper.DoRequest("POST", "/public/rider-access/"+first+"/redeem", nil, map[string]string{})
	assert.Equal(t, http.StatusGone, w.Code, "a new code revokes the last")

	w = helper.DoRequest("DELETE", "/tracking/riders/"+riderID+"/access-code", nil, map[string]string{})
	assert.Equal(t, http.StatusNoContent, w.Code)
	w = helper.DoRequest("POST", "/public/rider-access/"+second+"/redeem", nil, map[string]string{})
	assert.Equal(t, http.StatusGone, w.Code)
}

// TestRiderAccessCode_Expired - past its seven days a code answers 410.
func TestRiderAccessCode_Expired(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	riderID := CreateTestRider(t, helper)
	code := issueRiderAccessCode(t, helper, riderID)
	ExpireRiderAccessCode(t, helper, riderID)
	w := helper.DoRequest("POST", "/public/rider-access/"+code+"/check", map[string]string{"email": "qr-passenger-late@test.local"}, map[string]string{})
	assert.Equal(t, http.StatusGone, w.Code)
}
