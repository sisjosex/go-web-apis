//go:build integration
// +build integration

package auth_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"josex/web/modules/core/testhelpers"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// ============================================================================
// SESSION HARDENING (MOBILE-011 step 2)
// ============================================================================

// resetTokenFor reads the account's pending reset token: the endpoint only emails it.
func resetTokenFor(t *testing.T, helper *testhelpers.ApiTestHelper, email string) string {
	t.Helper()

	var token string
	// check:raw-sql the token only ever leaves by email; no endpoint or SP hands it back
	err := helper.DB().QueryRow(context.Background(), `
		SELECT t.token
		FROM auth.password_reset_tokens t
		INNER JOIN auth.users u ON u.id = t.user_id
		WHERE LOWER(u.email) = LOWER($1)
	`, email).Scan(&token)
	if err != nil {
		t.Fatalf("read reset token for %s: %v", email, err)
	}
	return token
}

// signIn opens a session on a device of its own and returns its tokens, leaving the helper's own
// session untouched.
func signIn(t *testing.T, helper *testhelpers.ApiTestHelper, email, password string) (access, refresh string) {
	t.Helper()

	body := map[string]interface{}{"email": email, "password": password, "device_id": testDeviceID()}
	w := helper.DoRequest("POST", "/auth/login", body, map[string]string{})
	if w.Code != http.StatusOK {
		t.Fatalf("login %s: expected 200, got %d: %s", email, w.Code, w.Body.String())
	}
	var resp map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	return resp["access_token"], resp["refresh_token"]
}

func refreshStatus(helper *testhelpers.ApiTestHelper, refresh string) int {
	w := helper.DoRequest("POST", "/auth/refresh_token", map[string]interface{}{"refresh_token": refresh}, map[string]string{})
	return w.Code
}

func TestOtpVerifyUnknownEmailOpensNothing(t *testing.T) {
	helper := testhelpers.SetupApiTest(t)
	defer helper.Close()

	email := "nobody-" + uuid.New().String()[:8] + "@test.com"
	const otpCode = "112233"
	var otpID uuid.UUID
	if err := helper.DB().QueryRow(context.Background(),
		`SELECT otp_id FROM auth.sp_request_otp($1, 'email', $2)`, email, otpCode).Scan(&otpID); err != nil {
		t.Fatalf("issue OTP for %s: %v", email, err)
	}

	body := map[string]interface{}{
		"destination": email,
		"otp_code":    otpCode,
		"channel":     "email",
		"device_id":   testDeviceID(),
	}
	w := helper.DoRequest("POST", "/auth/otp/verify", body, map[string]string{"X-Client-Type": "mobile"})

	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"error":"otp.invalid"`)
	assert.NotContains(t, w.Body.String(), "access_token")
}

func TestChangePasswordEndsOtherSessions(t *testing.T) {
	helper := testhelpers.SetupApiTest(t)
	defer helper.Close()

	email := "change-ends-" + uuid.New().String()[:8] + "@test.com"
	helper.Register(email, "$Password2025", "Change", "Ends")
	mine, err := helper.Login(email, "$Password2025")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	_, otherRefresh := signIn(t, helper, email, "$Password2025")

	w := helper.DoRequest("PUT", "/auth/password", map[string]interface{}{
		"password_current": "$Password2025",
		"password_new":     "$NewPassword2025",
	}, map[string]string{})
	if w.Code != http.StatusOK {
		t.Fatalf("change password: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	assert.Equal(t, http.StatusUnauthorized, refreshStatus(helper, otherRefresh), "the other phone is signed out")
	assert.Equal(t, http.StatusOK, refreshStatus(helper, mine["refresh_token"].(string)), "the changing phone stays in")
}

func TestResetPasswordEndsEverySession(t *testing.T) {
	helper := testhelpers.SetupApiTest(t)
	defer helper.Close()

	email := "reset-ends-" + uuid.New().String()[:8] + "@test.com"
	helper.Register(email, "$Password2025", "Reset", "Ends")
	_, refresh := signIn(t, helper, email, "$Password2025")

	if w := helper.DoRequest("POST", "/auth/password/reset", map[string]interface{}{"email": email}, map[string]string{}); w.Code != http.StatusOK {
		t.Fatalf("request reset: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	w := helper.DoRequest("PUT", "/auth/password/reset", map[string]interface{}{
		"token":        resetTokenFor(t, helper, email),
		"password_new": "$NewPassword2025",
	}, map[string]string{})
	if w.Code != http.StatusOK {
		t.Fatalf("reset: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	assert.Equal(t, http.StatusUnauthorized, refreshStatus(helper, refresh))
}

func TestRefreshRenewsRefreshToken(t *testing.T) {
	helper := testhelpers.SetupApiTest(t)
	defer helper.Close()

	email := "rotate-" + uuid.New().String()[:8] + "@test.com"
	helper.Register(email, "$Password2025", "Rotate", "Token")
	_, refresh := signIn(t, helper, email, "$Password2025")

	w := helper.DoRequest("POST", "/auth/refresh_token", map[string]interface{}{"refresh_token": refresh}, map[string]string{})
	if w.Code != http.StatusOK {
		t.Fatalf("refresh: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &resp)

	assert.NotEmpty(t, resp["access_token"])
	if assert.NotEmpty(t, resp["refresh_token"]) {
		assert.Equal(t, http.StatusOK, refreshStatus(helper, resp["refresh_token"]), "the renewed token refreshes")
	}
}

func TestSessionsMarkTheCurrentOne(t *testing.T) {
	helper := testhelpers.SetupApiTest(t)
	defer helper.Close()

	email := "current-" + uuid.New().String()[:8] + "@test.com"
	helper.Register(email, "$Password2025", "Current", "Session")
	signIn(t, helper, email, "$Password2025")
	if _, err := helper.Login(email, "$Password2025"); err != nil {
		t.Fatalf("login: %v", err)
	}

	w := helper.DoRequest("GET", "/auth/sessions", nil, map[string]string{})
	if w.Code != http.StatusOK {
		t.Fatalf("sessions: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var sessions []map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &sessions)

	current := 0
	for _, s := range sessions {
		if s["is_current"] == true {
			current++
		}
	}
	assert.GreaterOrEqual(t, len(sessions), 2)
	assert.Equal(t, 1, current, w.Body.String())
}

func TestLogoutSessionErrorsAreNotServerErrors(t *testing.T) {
	helper := testhelpers.SetupApiTest(t)
	defer helper.Close()

	email := "logout-codes-" + uuid.New().String()[:8] + "@test.com"
	helper.Register(email, "$Password2025", "Logout", "Codes")
	signIn(t, helper, email, "$Password2025")
	if _, err := helper.Login(email, "$Password2025"); err != nil {
		t.Fatalf("login: %v", err)
	}

	w := helper.DoRequest("DELETE", "/auth/sessions/"+uuid.New().String(), nil, map[string]string{})
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())

	w = helper.DoRequest("GET", "/auth/sessions", nil, map[string]string{})
	var sessions []map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &sessions)
	var other string
	for _, s := range sessions {
		if s["is_current"] != true {
			other = s["session_id"].(string)
			break
		}
	}
	if other == "" {
		t.Fatalf("no second session in %s", w.Body.String())
	}

	w = helper.DoRequest("DELETE", "/auth/sessions/"+other, nil, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	w = helper.DoRequest("DELETE", "/auth/sessions/"+other, nil, map[string]string{})
	assert.Contains(t, []int{http.StatusBadRequest, http.StatusNotFound}, w.Code, w.Body.String())
}
