//go:build integration
// +build integration

package auth_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"josex/web/config"
	"josex/web/modules/core/testhelpers"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const googleTestClientID = "test-client.apps.googleusercontent.com"

// TestMain stands a fake Google token endpoint up and points the auth config at it before the
// shared router is built, so /login/google exchanges real HTTP without reaching Google (APP-016).
// A code is `test:<email>` (verified), `unverified:<email>` or `wrongaud:<email>`; anything else is refused.
func TestMain(m *testing.M) {
	fake := httptest.NewServer(http.HandlerFunc(fakeGoogleToken))
	auth := config.GetConfig().Auth
	auth.EnableGoogleAuth = true
	auth.GoogleClientId = googleTestClientID
	auth.GoogleClientSecret = "test-secret"
	auth.GoogleTokenURL = fake.URL
	code := m.Run()
	fake.Close()
	os.Exit(code)
}

func fakeGoogleToken(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	kind, email, ok := strings.Cut(r.PostForm.Get("code"), ":")
	if !ok || r.PostForm.Get("client_secret") != "test-secret" || r.PostForm.Get("redirect_uri") != "postmessage" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
		return
	}
	audience := googleTestClientID
	if kind == "wrongaud" {
		audience = "someone-else.apps.googleusercontent.com"
	}
	claims := jwt.MapClaims{
		"iss":            "https://accounts.google.com",
		"aud":            audience,
		"sub":            "google-" + email,
		"email":          email,
		"email_verified": kind != "unverified",
		"given_name":     "Gina",
		"family_name":    "Google",
		"exp":            time.Now().Add(time.Hour).Unix(),
	}
	idToken, _ := jwt.NewWithClaims(jwt.SigningMethodNone, claims).SignedString(jwt.UnsafeAllowNoneSignatureType)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"access_token": "fake-access",
		"token_type":   "Bearer",
		"expires_in":   3600,
		"id_token":     idToken,
	})
}

// googleLogin signs in with a fake code and returns the account's id from its profile.
func googleLogin(t *testing.T, helper *testhelpers.ApiTestHelper, code string) string {
	t.Helper()
	w := helper.DoRequest("POST", "/auth/login/google", map[string]any{"code": code, "device_id": testDeviceID()}, map[string]string{})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var tokens map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &tokens))
	require.NotEmpty(t, tokens["refresh_token"])

	profile := helper.DoRequest("GET", "/auth/profile", nil, map[string]string{"Authorization": "Bearer " + tokens["access_token"].(string)})
	require.Equal(t, http.StatusOK, profile.Code, profile.Body.String())
	var me map[string]any
	require.NoError(t, json.Unmarshal(profile.Body.Bytes(), &me))
	return me["id"].(string)
}

// TestLoginGoogleCreatesThenReusesAccount - a never-seen address gets an account (D4), the next sign-in reuses it.
func TestLoginGoogleCreatesThenReusesAccount(t *testing.T) {
	helper := testhelpers.SetupApiTest(t)
	defer helper.Close()

	email := "google-" + uuid.New().String() + "@test.com"
	first := googleLogin(t, helper, "test:"+email)
	second := googleLogin(t, helper, "test:"+email)

	assert.NotEmpty(t, first)
	assert.Equal(t, first, second)
}

// TestLoginGoogleExistingPasswordAccount - an address registered with a password signs into that same account.
func TestLoginGoogleExistingPasswordAccount(t *testing.T) {
	helper := testhelpers.SetupApiTest(t)
	defer helper.Close()

	email := "google-pw-" + uuid.New().String() + "@test.com"
	_, err := helper.Register(email, "$Password2025", "Pat", "Word")
	require.NoError(t, err)
	_, err = helper.Login(email, "$Password2025")
	require.NoError(t, err)
	profile := helper.DoRequest("GET", "/auth/profile", nil, map[string]string{})
	var me map[string]any
	require.NoError(t, json.Unmarshal(profile.Body.Bytes(), &me))
	helper.ClearToken()

	assert.Equal(t, me["id"], googleLogin(t, helper, "test:"+email))
}

// TestLoginGoogleRefused - a code Google refuses, an unverified address and a token for another app are all 401.
func TestLoginGoogleRefused(t *testing.T) {
	helper := testhelpers.SetupApiTest(t)
	defer helper.Close()

	email := "google-no-" + uuid.New().String() + "@test.com"
	for _, code := range []string{"garbage", "unverified:" + email, "wrongaud:" + email} {
		w := helper.DoRequest("POST", "/auth/login/google", map[string]any{"code": code, "device_id": testDeviceID()}, map[string]string{})
		assert.Equal(t, http.StatusUnauthorized, w.Code, code)
		assert.Contains(t, w.Body.String(), "user.login.google-invalid", code)
	}
}

// TestLoginGoogleMissingDevice - the session needs a device id; without one the body is refused.
func TestLoginGoogleMissingDevice(t *testing.T) {
	helper := testhelpers.SetupApiTest(t)
	defer helper.Close()

	w := helper.DoRequest("POST", "/auth/login/google", map[string]any{"code": "test:nodevice@test.com"}, map[string]string{})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}
