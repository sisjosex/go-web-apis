//go:build integration
// +build integration

package auth_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	coreTestHelpers "josex/web/modules/core/testhelpers"
)

// pushDeviceRows counts the account's rows for deviceID, and the rows holding token anywhere.
func pushDeviceRows(t *testing.T, helper *coreTestHelpers.ApiTestHelper, email, deviceID, token string) (int, int) {
	t.Helper()
	var mine, withToken int
	err := helper.DB().QueryRow(t.Context(), `
		SELECT (SELECT count(*) FROM auth.push_devices d JOIN auth.users u ON u.id = d.user_id
		         WHERE u.email = $1 AND d.device_id = $2),
		       (SELECT count(*) FROM auth.push_devices WHERE token = $3)`, email, deviceID, token).Scan(&mine, &withToken)
	if err != nil {
		t.Fatalf("count push devices: %v", err)
	}
	return mine, withToken
}

// TestPushDevices_UpsertAndForget - TRACK-012 step 2: the same POST twice leaves one row, a rotated
// token replaces the old one, the token moves when another account registers it, and DELETE forgets
// the device (twice is not an error).
func TestPushDevices_UpsertAndForget(t *testing.T) {
	helper := coreTestHelpers.SetupApiTest(t)
	defer helper.Close()
	if _, err := helper.Login("admin@test.local", "Admin123!"); err != nil {
		t.Fatalf("login: %v", err)
	}
	t.Cleanup(func() {
		_, _ = helper.DB().Execute(t.Context(), `DELETE FROM auth.push_devices WHERE device_id LIKE 'test-device-%'`)
	})
	body := map[string]interface{}{"device_id": "test-device-1", "platform": "android", "token": "tok-a"}

	for i := 0; i < 2; i++ {
		w := helper.DoRequest("POST", "/mobile/devices", body, map[string]string{})
		assert.Equal(t, http.StatusNoContent, w.Code, w.Body.String())
	}
	mine, withToken := pushDeviceRows(t, helper, "admin@test.local", "test-device-1", "tok-a")
	assert.Equal(t, 1, mine, "the same POST twice leaves one row")
	assert.Equal(t, 1, withToken)

	body["token"] = "tok-b"
	w := helper.DoRequest("POST", "/mobile/devices", body, map[string]string{})
	assert.Equal(t, http.StatusNoContent, w.Code, w.Body.String())
	_, withOld := pushDeviceRows(t, helper, "admin@test.local", "test-device-1", "tok-a")
	assert.Equal(t, 0, withOld, "a rotated token replaces the old one")

	w = helper.DoRequest("POST", "/mobile/devices", map[string]interface{}{"device_id": "test-device-1", "platform": "web", "token": "x"}, map[string]string{})
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())

	other := coreTestHelpers.SetupApiTest(t)
	defer other.Close()
	if _, err := other.Login("portal@test.local", "Portal123!"); err != nil {
		t.Fatalf("login portal: %v", err)
	}
	w = other.DoRequest("POST", "/mobile/devices", map[string]interface{}{"device_id": "test-device-2", "platform": "ios", "token": "tok-b"}, map[string]string{})
	assert.Equal(t, http.StatusNoContent, w.Code, w.Body.String())
	mine, withToken = pushDeviceRows(t, helper, "admin@test.local", "test-device-1", "tok-b")
	assert.Equal(t, 0, mine, "the token moved to the account that registered it last")
	assert.Equal(t, 1, withToken)

	for i := 0; i < 2; i++ {
		w = other.DoRequest("DELETE", "/mobile/devices/test-device-2", nil, map[string]string{})
		assert.Equal(t, http.StatusNoContent, w.Code, w.Body.String())
	}
	_, withToken = pushDeviceRows(t, helper, "portal@test.local", "test-device-2", "tok-b")
	assert.Equal(t, 0, withToken)

	helper.ClearToken()
	w = helper.DoRequest("POST", "/mobile/devices", body, map[string]string{})
	assert.Equal(t, http.StatusUnauthorized, w.Code, w.Body.String())
}
