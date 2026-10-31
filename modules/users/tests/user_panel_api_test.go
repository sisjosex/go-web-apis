//go:build integration
// +build integration

package users_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"josex/web/modules/users/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// USERS-015: what the Usuarios page reads, and the invitation with only an email.

func listedUser(t *testing.T, list models.UserListResponse, id string) *models.UserListItem {
	t.Helper()
	for i := range list.Users {
		if list.Users[i].ID == id {
			return &list.Users[i]
		}
	}
	return nil
}

func listUsers(t *testing.T, get func(string) (int, []byte), query string) models.UserListResponse {
	t.Helper()
	code, body := get("/users?limit=100&" + query)
	require.Equal(t, http.StatusOK, code, string(body))
	var list models.UserListResponse
	require.NoError(t, json.Unmarshal(body, &list))
	return list
}

// TestUserList_PendingRoleAndModules - someone invited with only an email comes back pending, named after
// the email, with the role and the modules it reaches; the filters find them.
func TestUserList_PendingRoleAndModules(t *testing.T) {
	helper := setupSuperAdmin(t)
	defer helper.Close()
	get := func(path string) (int, []byte) {
		w := helper.DoRequest("GET", path, nil, testCompany)
		return w.Code, w.Body.Bytes()
	}
	w := helper.DoRequest("POST", "/tenants/test-company/roles", map[string]interface{}{"name": fmt.Sprintf("Panel %d", time.Now().UnixNano())}, testCompany)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var role struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &role))
	w = helper.DoRequest("PUT", "/tenants/test-company/roles/"+role.ID+"/permissions", map[string]interface{}{"permissions": []string{"tracking:routes:read"}}, testCompany)
	require.Equal(t, http.StatusNoContent, w.Code, w.Body.String())

	email := fmt.Sprintf("ana-%d@test.local", time.Now().UnixNano())
	w = helper.DoRequest("POST", "/users", map[string]interface{}{"email": email, "tenant_role": "member", "role_ids": []string{role.ID}}, testCompany)
	require.Contains(t, []int{http.StatusOK, http.StatusCreated}, w.Code, w.Body.String())
	var created struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))

	row := listedUser(t, listUsers(t, get, "role_id="+role.ID), created.ID)
	require.NotNil(t, row, "the role filter finds them")
	assert.True(t, row.InvitationPending)
	assert.Nil(t, row.LastSignInAt)
	require.NotNil(t, row.Role)
	assert.Equal(t, role.ID, row.Role.ID.String())
	assert.Equal(t, 1, row.RoleCount)
	assert.Contains(t, row.Modules, "tracking")
	require.NotNil(t, row.FirstName)
	assert.Contains(t, email, *row.FirstName)

	assert.NotNil(t, listedUser(t, listUsers(t, get, "pending=true"), created.ID), "the pending filter finds them")
	self := listedUser(t, listUsers(t, get, "level=admin"), helper.GetUserID())
	if self != nil {
		assert.False(t, self.InvitationPending, "the caller signed in")
		assert.NotNil(t, self.LastSignInAt)
	}

	code, body := get("/users/stats")
	require.Equal(t, http.StatusOK, code, string(body))
	var stats models.UserStatsResponse
	require.NoError(t, json.Unmarshal(body, &stats))
	assert.GreaterOrEqual(t, stats.Pending, int64(1))
	counts := map[string]int64{}
	for _, c := range stats.ByRole {
		counts[c.Key] = c.Count
	}
	assert.Equal(t, int64(1), counts[role.ID])
	assert.Contains(t, counts, "admin")
	assert.Contains(t, counts, "organization")

	code, _ = get("/users?role_id=nope")
	assert.Equal(t, http.StatusBadRequest, code)
}

// TestResendInvitation - a person who never signed in gets it again; once they did it is a 409; a person of
// another workspace is a 404.
func TestResendInvitation(t *testing.T) {
	helper := setupSuperAdmin(t)
	defer helper.Close()
	id, email := createTenantUser(t, helper, "resend")

	w := helper.DoRequest("POST", "/users/"+id+"/invitation", nil, testCompany)
	assert.Equal(t, http.StatusAccepted, w.Code, w.Body.String())

	owner, ownHeader := ownWorkspace(t)
	defer owner.Close()
	w = owner.DoRequest("POST", "/users/"+id+"/invitation", nil, ownHeader)
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())

	member := outsiderLogin(t, email, "AdminTarget1!")
	defer member.Close()
	w = helper.DoRequest("POST", "/users/"+id+"/invitation", nil, testCompany)
	assert.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "user.invitation.already-active")
}
