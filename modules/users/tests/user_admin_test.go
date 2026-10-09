//go:build integration
// +build integration

package users_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"josex/web/config"
	"josex/web/modules/core/testhelpers"
	"josex/web/modules/users/models"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// INFRA-014: the user admin routes each reached with a success, plus their refusals and the
// tenant scope of stats and audit.

var testCompany = map[string]string{"X-Tenant-Slug": "test-company"}

// createTenantUser creates a user in test-company as the super_admin and answers its id and email.
func createTenantUser(t *testing.T, helper *testhelpers.ApiTestHelper, prefix string) (string, string) {
	t.Helper()
	email := fmt.Sprintf("%s-%d@example.com", prefix, time.Now().UnixNano())
	w := helper.DoRequest("POST", "/users", models.CreateUserDto{
		FirstName: "Admin", LastName: "Target", Email: email, Password: "AdminTarget1!",
	}, testCompany)
	require.Contains(t, []int{http.StatusOK, http.StatusCreated}, w.Code, "create user: %s", w.Body.String())
	var created map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	id, _ := created["id"].(string)
	require.NotEmpty(t, id, "the created user has an id: %s", w.Body.String())
	return id, email
}

// outsider signs in a user who belongs to no tenant, sending test-company's slug.
func outsider(t *testing.T) *testhelpers.ApiTestHelper {
	t.Helper()
	helper := testhelpers.SetupApiTest(t)
	email := fmt.Sprintf("users-outsider-%d@test.com", time.Now().UnixNano())
	helper.Register(email, "$Password2025", "Out", "Sider")
	if _, err := helper.Login(email, "$Password2025"); err != nil {
		t.Fatalf("outsider login: %v", err)
	}
	return helper
}

// TestUsers_GetAndUpdate - an admin reads a user and edits it; the edit lands in the audit log.
func TestUsers_GetAndUpdate(t *testing.T) {
	helper := setupSuperAdmin(t)
	defer helper.Close()
	id, email := createTenantUser(t, helper, "admin-edit")

	w := helper.DoRequest("GET", "/users/"+id, nil, testCompany)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), email)

	w = helper.DoRequest("PUT", "/users/"+id, map[string]interface{}{"first_name": "Renamed"}, testCompany)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	w = helper.DoRequest("GET", "/users/"+id, nil, testCompany)
	assert.Contains(t, w.Body.String(), "Renamed", "the edit is stored")

	w = helper.DoRequest("GET", "/users/audit?action=user.updated&search="+email, nil, testCompany)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var audit models.UserAuditListResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &audit))
	require.NotEmpty(t, audit.Entries, "the edit is audited: %s", w.Body.String())
	assert.Equal(t, id, audit.Entries[0].TargetUserID.String())

	w = helper.DoRequest("PUT", "/users/not-a-uuid", map[string]interface{}{"first_name": "X"}, testCompany)
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	w = helper.DoRequest("GET", "/users/"+uuid.NewString(), nil, testCompany)
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())

	stranger := outsider(t)
	w = stranger.DoRequest("PUT", "/users/"+id, map[string]interface{}{"first_name": "Hijack"}, testCompany)
	assert.Equal(t, http.StatusForbidden, w.Code, "a non-member edits nobody: %s", w.Body.String())
	w = stranger.DoRequest("GET", "/users/"+id, nil, testCompany)
	assert.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
}

// TestUsers_Stats - the counts are the workspace's: a new user raises its total, not another's.
func TestUsers_Stats(t *testing.T) {
	helper := setupSuperAdmin(t)
	defer helper.Close()

	stats := func() models.UserStatsResponse {
		w := helper.DoRequest("GET", "/users/stats", nil, testCompany)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		var out models.UserStatsResponse
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
		return out
	}
	before := stats()
	createTenantUser(t, helper, "admin-stats")
	after := stats()
	assert.Equal(t, before.Total+1, after.Total, "one more user in the workspace")

	stranger := outsider(t)
	w := stranger.DoRequest("GET", "/users/stats", nil, testCompany)
	assert.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
}

// TestUsers_AuditIsTenantScoped - another workspace's owner never reads test-company's audit.
func TestUsers_AuditIsTenantScoped(t *testing.T) {
	helper := setupSuperAdmin(t)
	defer helper.Close()
	id, email := createTenantUser(t, helper, "admin-audit")
	w := helper.DoRequest("PUT", "/users/"+id, map[string]interface{}{"last_name": "Audited"}, testCompany)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	owner := outsider(t)
	slug := fmt.Sprintf("users-audit-%d", time.Now().UnixNano())
	w = owner.DoRequest("POST", "/tenants/self-service", map[string]interface{}{"slug": slug, "name": "Audit"}, nil)
	require.Contains(t, []int{http.StatusOK, http.StatusCreated}, w.Code, w.Body.String())

	w = owner.DoRequest("GET", "/users/audit?search="+email, nil, map[string]string{"X-Tenant-Slug": slug})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var audit models.UserAuditListResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &audit))
	assert.Empty(t, audit.Entries, "another workspace's audit is not visible: %s", w.Body.String())

	w = owner.DoRequest("GET", "/users/audit", nil, testCompany)
	assert.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
}

// TestUsers_ResetPassword - an admin sends a reset; an unknown user answers the same, so ids are not probed.
func TestUsers_ResetPassword(t *testing.T) {
	helper := setupSuperAdmin(t)
	defer helper.Close()
	id, _ := createTenantUser(t, helper, "admin-reset")

	w := helper.DoRequest("POST", "/users/"+id+"/reset-password", nil, testCompany)
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())

	w = helper.DoRequest("POST", "/users/"+uuid.NewString()+"/reset-password", nil, testCompany)
	assert.Equal(t, http.StatusOK, w.Code, "an unknown user answers like a known one: %s", w.Body.String())

	w = helper.DoRequest("POST", "/users/not-a-uuid/reset-password", nil, testCompany)
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())

	stranger := outsider(t)
	w = stranger.DoRequest("POST", "/users/"+id+"/reset-password", nil, testCompany)
	assert.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
}

// USERS-013: the user routes reach a user of the caller's workspace only.

// ownWorkspace signs in a new user who owns a fresh workspace and answers it with that workspace's header.
func ownWorkspace(t *testing.T) (*testhelpers.ApiTestHelper, map[string]string) {
	t.Helper()
	owner := outsider(t)
	slug := fmt.Sprintf("users-scope-%d", time.Now().UnixNano())
	w := owner.DoRequest("POST", "/tenants/self-service", map[string]interface{}{"slug": slug, "name": "Scope"}, nil)
	require.Contains(t, []int{http.StatusOK, http.StatusCreated}, w.Code, w.Body.String())
	return owner, map[string]string{"X-Tenant-Slug": slug}
}

// allowDeletion turns ALLOW_USER_DELETION on for one test; .env.test leaves it off. Call it after the
// helper is set up, which loads the config.
func allowDeletion(t *testing.T) {
	t.Helper()
	usersConfig := config.ModularAppConfig.Users
	previous := usersConfig.AllowUserDeletion
	usersConfig.AllowUserDeletion = true
	t.Cleanup(func() { usersConfig.AllowUserDeletion = previous })
}

// TestUsers_OtherWorkspaceReadsAsUnknown - an owner of another workspace gets a test-company user's id
// answered exactly like an id that does not exist, on every route (D1).
func TestUsers_OtherWorkspaceReadsAsUnknown(t *testing.T) {
	helper := setupSuperAdmin(t)
	defer helper.Close()
	allowDeletion(t)
	id, email := createTenantUser(t, helper, "scope-target")
	owner, ownHeader := ownWorkspace(t)

	w := owner.DoRequest("GET", "/users/"+id, nil, ownHeader)
	assert.Equal(t, http.StatusNotFound, w.Code, "read: %s", w.Body.String())
	assert.NotContains(t, w.Body.String(), email)

	w = owner.DoRequest("PUT", "/users/"+id, map[string]interface{}{"first_name": "Hijack"}, ownHeader)
	assert.Equal(t, http.StatusNotFound, w.Code, "edit: %s", w.Body.String())

	w = owner.DoRequest("DELETE", "/users/"+id, nil, ownHeader)
	assert.Equal(t, http.StatusNotFound, w.Code, "delete: %s", w.Body.String())

	// Reset answers an unknown id with 200 so ids are not probed; another workspace's user is unknown.
	w = owner.DoRequest("POST", "/users/"+id+"/reset-password", nil, ownHeader)
	assert.Equal(t, http.StatusOK, w.Code, "reset: %s", w.Body.String())
	time.Sleep(200 * time.Millisecond) // the audit row of a sent reset is written in a goroutine
	w = owner.DoRequest("GET", "/users/audit?action=user.password-reset", nil, ownHeader)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var audit models.UserAuditListResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &audit))
	assert.Empty(t, audit.Entries, "no reset was issued: %s", w.Body.String())

	w = helper.DoRequest("GET", "/users/"+id, nil, testCompany)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.NotContains(t, w.Body.String(), "Hijack", "the user is untouched")
}

// TestUsers_UpdateIgnoresBodyID - the path names the user; an id in the body changes nothing.
func TestUsers_UpdateIgnoresBodyID(t *testing.T) {
	helper := setupSuperAdmin(t)
	defer helper.Close()
	pathID, _ := createTenantUser(t, helper, "scope-path")
	bodyID, _ := createTenantUser(t, helper, "scope-body")

	w := helper.DoRequest("PUT", "/users/"+pathID, map[string]interface{}{"id": bodyID, "first_name": "PathUser"}, testCompany)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), pathID, "the path's user is the one answered")

	w = helper.DoRequest("GET", "/users/"+pathID, nil, testCompany)
	assert.Contains(t, w.Body.String(), "PathUser", "the path's user is updated")
	w = helper.DoRequest("GET", "/users/"+bodyID, nil, testCompany)
	assert.NotContains(t, w.Body.String(), "PathUser", "the body's user is untouched")
}

// sharedUser creates a test-company user and adds it to a second workspace; it answers the user's id,
// email and password plus the second workspace's owner and header.
func sharedUser(t *testing.T, helper *testhelpers.ApiTestHelper) (string, string, string, *testhelpers.ApiTestHelper, map[string]string) {
	t.Helper()
	id, email := createTenantUser(t, helper, "scope-shared")
	owner, ownHeader := ownWorkspace(t)
	w := owner.DoRequest("POST", "/tenants/"+ownHeader["X-Tenant-Slug"]+"/users",
		map[string]interface{}{"user_id": id, "role": "member"}, ownHeader)
	require.Contains(t, []int{http.StatusOK, http.StatusCreated}, w.Code, "add to the second workspace: %s", w.Body.String())
	return id, email, "AdminTarget1!", owner, ownHeader
}

// TestUsers_DeleteSharedUserLeavesOtherWorkspace - deleting a user who also works elsewhere removes it
// from this workspace only; it keeps signing in and the other workspace keeps it (D2).
func TestUsers_DeleteSharedUserLeavesOtherWorkspace(t *testing.T) {
	helper := setupSuperAdmin(t)
	defer helper.Close()
	allowDeletion(t)
	id, email, password, owner, ownHeader := sharedUser(t, helper)

	w := helper.DoRequest("DELETE", "/users/"+id, nil, testCompany)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	w = helper.DoRequest("GET", "/users/"+id, nil, testCompany)
	assert.Equal(t, http.StatusNotFound, w.Code, "gone from test-company: %s", w.Body.String())
	w = helper.DoRequest("GET", "/users?search="+email, nil, testCompany)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.NotContains(t, w.Body.String(), email, "test-company's list no longer has it")

	w = owner.DoRequest("GET", "/users/"+id, nil, ownHeader)
	assert.Equal(t, http.StatusOK, w.Code, "the other workspace keeps it: %s", w.Body.String())

	member := testhelpers.SetupApiTest(t)
	defer member.Close()
	_, err := member.Login(email, password)
	assert.NoError(t, err, "the account still signs in")
}

// TestUsers_SharedAccountLoginIsLocked - on an account in two workspaces the email is refused and the
// name is not (D3); once it is in one workspace only, the email changes.
func TestUsers_SharedAccountLoginIsLocked(t *testing.T) {
	helper := setupSuperAdmin(t)
	defer helper.Close()
	id, email, _, owner, ownHeader := sharedUser(t, helper)

	newEmail := fmt.Sprintf("scope-moved-%d@example.com", time.Now().UnixNano())
	w := helper.DoRequest("PUT", "/users/"+id, map[string]interface{}{"email": newEmail}, testCompany)
	assert.Equal(t, http.StatusConflict, w.Code, "email on a shared account: %s", w.Body.String())
	assert.Contains(t, w.Body.String(), "user.update.shared-account")

	w = helper.DoRequest("PUT", "/users/"+id, map[string]interface{}{"first_name": "Shared", "email": email}, testCompany)
	assert.Equal(t, http.StatusOK, w.Code, "name, with the unchanged email: %s", w.Body.String())

	w = owner.DoRequest("DELETE", "/tenants/"+ownHeader["X-Tenant-Slug"]+"/users/"+id, nil, ownHeader)
	require.Equal(t, http.StatusOK, w.Code, "leave the second workspace: %s", w.Body.String())
	w = helper.DoRequest("PUT", "/users/"+id, map[string]interface{}{"email": newEmail}, testCompany)
	assert.Equal(t, http.StatusOK, w.Code, "email on a single-workspace account: %s", w.Body.String())
}
