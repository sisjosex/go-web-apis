//go:build integration
// +build integration

package tenancy_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"josex/web/modules/core/testhelpers"
)

// settingsOf decodes a GET|PATCH /tenants/:slug/settings answer.
func settingsOf(t *testing.T, body []byte) (map[string]interface{}, []map[string]interface{}) {
	t.Helper()
	var out struct {
		Settings map[string]interface{}   `json:"settings"`
		Specs    []map[string]interface{} `json:"specs"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("decode settings: %v: %s", err, body)
	}
	return out.Settings, out.Specs
}

// TestTenantSettings_MergeAndRead - TENANCY-007 step 1: a PATCH changes only the keys it sends, the very
// next GET reads the change (the cached row was dropped), defaults fill what was never set, and the
// tenant list carries the stored settings.
func TestTenantSettings_MergeAndRead(t *testing.T) {
	admin := testhelpers.SetupApiTest(t)
	defer admin.Close()
	admin.Login("admin@test.local", "Admin123!")
	admin.SetTenantSlug("test-company")
	defer admin.DoRequest("PATCH", "/tenants/test-company/settings", map[string]interface{}{
		"general.locale": "es", "tracking.notice.approaching": true,
	}, map[string]string{})

	w := admin.DoRequest("GET", "/tenants/test-company/settings", nil, map[string]string{})
	if !assert.Equal(t, http.StatusOK, w.Code, w.Body.String()) {
		return
	}
	settings, specs := settingsOf(t, w.Body.Bytes())
	assert.Equal(t, "America/La_Paz", settings["general.timezone"], "a key never set reads its default")
	assert.Equal(t, true, settings["tracking.notice.approaching"], "tracking is on, so its keys are offered")
	keys := map[string]bool{}
	for _, s := range specs {
		keys[s["key"].(string)] = true
	}
	assert.True(t, keys["general.locale"] && keys["tracking.notice.boarding"])

	w = admin.DoRequest("PATCH", "/tenants/test-company/settings", map[string]interface{}{"general.locale": "en"}, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	w = admin.DoRequest("PATCH", "/tenants/test-company/settings", map[string]interface{}{"tracking.notice.approaching": false}, map[string]string{})
	if !assert.Equal(t, http.StatusOK, w.Code, w.Body.String()) {
		return
	}
	settings, _ = settingsOf(t, w.Body.Bytes())
	assert.Equal(t, "en", settings["general.locale"], "a merge keeps the keys it was not sent")
	assert.Equal(t, false, settings["tracking.notice.approaching"])

	w = admin.DoRequest("GET", "/tenants/test-company/settings", nil, map[string]string{})
	settings, _ = settingsOf(t, w.Body.Bytes())
	assert.Equal(t, false, settings["tracking.notice.approaching"], "the next read sees the write")
	assert.Equal(t, "en", settings["general.locale"])

	w = admin.DoRequest("GET", "/tenants/me", nil, map[string]string{})
	var tenants []map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &tenants)
	for _, tenant := range tenants {
		if tenant["slug"] == "test-company" {
			stored, _ := tenant["settings"].(map[string]interface{})
			assert.Equal(t, "en", stored["general.locale"], "the tenant list carries the stored settings")
		}
	}
}

// TestTenantSettings_Refusals - an undeclared key or a value its type refuses is 422 and changes
// nothing; a member who is not owner or admin may read the settings but not change them.
func TestTenantSettings_Refusals(t *testing.T) {
	admin := testhelpers.SetupApiTest(t)
	defer admin.Close()
	admin.Login("admin@test.local", "Admin123!")
	admin.SetTenantSlug("test-company")

	cases := []struct {
		name  string
		patch map[string]interface{}
		code  string
	}{
		{"unknown key", map[string]interface{}{"x.y": 1}, "tenant.settings.unknown-key"},
		{"wrong type", map[string]interface{}{"tracking.notice.delay": "no"}, "tenant.settings.invalid"},
		{"not an option", map[string]interface{}{"general.locale": "fr"}, "tenant.settings.invalid"},
	}
	for _, tc := range cases {
		w := admin.DoRequest("PATCH", "/tenants/test-company/settings", tc.patch, map[string]string{})
		assert.Equal(t, http.StatusUnprocessableEntity, w.Code, tc.name)
		var body map[string]interface{}
		_ = json.Unmarshal(w.Body.Bytes(), &body)
		assert.Equal(t, tc.code, body["error"], tc.name)
	}

	member := testhelpers.SetupApiTest(t)
	defer member.Close()
	member.Login("orguser@test.local", "OrgUser123!")
	member.SetTenantSlug("test-company")
	w := member.DoRequest("GET", "/tenants/test-company/settings", nil, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code, "every member reads the settings: %s", w.Body.String())
	w = member.DoRequest("PATCH", "/tenants/test-company/settings", map[string]interface{}{"general.locale": "en"}, map[string]string{})
	assert.Equal(t, http.StatusForbidden, w.Code, "only owner and admin change them: %s", w.Body.String())
}
