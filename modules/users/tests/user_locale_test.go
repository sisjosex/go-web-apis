//go:build integration
// +build integration

package users_test

import (
	"net/http"
	"testing"

	"josex/web/modules/core/testhelpers"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUsers_InviteTakesWorkspaceLocale - TENANCY-007 step 2: an account created inside a workspace
// starts in the workspace's language, read from its settings.
func TestUsers_InviteTakesWorkspaceLocale(t *testing.T) {
	helper := setupSuperAdmin(t)
	defer helper.Close()
	admin := testhelpers.SetupApiTest(t)
	defer admin.Close()
	admin.Login("admin@test.local", "Admin123!")
	w := admin.DoRequest("PATCH", "/tenants/test-company/settings", map[string]interface{}{"general.locale": "en"}, testCompany)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	defer admin.DoRequest("PATCH", "/tenants/test-company/settings", map[string]interface{}{"general.locale": "es"}, testCompany)

	id, _ := createTenantUser(t, helper, "invite-locale")
	assert.Equal(t, "en", accountLocale(t, helper, id))
}
