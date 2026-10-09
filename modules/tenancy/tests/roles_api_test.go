//go:build integration
// +build integration

package tenancy_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"josex/web/modules/core/testhelpers"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// INFRA-014: the role and permission routes, each reached with a success, every SP refusal, and a
// role of one workspace looked up through another.

// roleFixture is an owner's fresh workspace with a plain member in it and one custom role.
type roleFixture struct {
	owner    *testhelpers.ApiTestHelper
	member   *testhelpers.ApiTestHelper
	slug     string
	memberID string
	roleID   string
}

func newRoleFixture(t *testing.T) roleFixture {
	t.Helper()
	owner := testhelpers.SetupApiTest(t)
	slug := freshTenant(t, owner)

	member := testhelpers.SetupApiTest(t)
	email := fmt.Sprintf("role-member-%d@test.com", time.Now().UnixNano())
	if _, err := member.Register(email, "$Password2025", "Role", "Member"); err != nil {
		t.Fatalf("register member: %v", err)
	}
	if _, err := member.Login(email, "$Password2025"); err != nil {
		t.Fatalf("member login: %v", err)
	}
	memberID := member.GetUserID()
	w := owner.DoRequest("POST", "/tenants/"+slug+"/users", map[string]interface{}{"user_id": memberID, "role": "member"}, nil)
	require.Contains(t, []int{http.StatusOK, http.StatusCreated}, w.Code, "add member: %s", w.Body.String())
	member.SetTenantSlug(slug)

	return roleFixture{owner: owner, member: member, slug: slug, memberID: memberID, roleID: createRole(t, owner, slug, "Cashiers")}
}

func (f roleFixture) path(suffix string) string {
	return "/tenants/" + f.slug + suffix
}

func createRole(t *testing.T, helper *testhelpers.ApiTestHelper, slug, name string) string {
	t.Helper()
	w := helper.DoRequest("POST", "/tenants/"+slug+"/roles", map[string]interface{}{"name": name}, nil)
	require.Equal(t, http.StatusCreated, w.Code, "create role: %s", w.Body.String())
	id, _ := decode(t, w.Body.Bytes())["id"].(string)
	require.NotEmpty(t, id, "the created role has an id")
	return id
}

// TestPermissions_ListCatalogFilteredByModule - the catalog lists translated permissions; ?module keeps one module's.
func TestPermissions_ListCatalogFilteredByModule(t *testing.T) {
	helper := testhelpers.SetupApiTest(t)
	defer helper.Close()
	freshTenant(t, helper)

	w := helper.DoRequest("GET", "/permissions", nil, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var all []map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &all))
	assert.NotEmpty(t, all, "the catalog is not empty")

	w = helper.DoRequest("GET", "/permissions?module=inventory", nil, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var inventory []map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &inventory))
	assert.NotEmpty(t, inventory)
	assert.Less(t, len(inventory), len(all), "the filter narrows the catalog")
	for _, p := range inventory {
		assert.Equal(t, "inventory", p["module"], "only inventory permissions: %v", p)
		assert.NotEmpty(t, p["name"], "the name is translated: %v", p)
	}

	helper.ClearToken()
	w = helper.DoRequest("GET", "/permissions", nil, nil)
	assert.Equal(t, http.StatusUnauthorized, w.Code, "the catalog needs a session")
}

// TestRoles_ListAndGet - a member reads the roles; an unknown or malformed id is refused.
func TestRoles_ListAndGet(t *testing.T) {
	f := newRoleFixture(t)

	w := f.member.DoRequest("GET", f.path("/roles"), nil, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), f.roleID, "the list holds the role")

	w = f.member.DoRequest("GET", f.path("/roles/"+f.roleID), nil, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, "Cashiers", decode(t, w.Body.Bytes())["name"])

	w = f.owner.DoRequest("GET", f.path("/roles/"+uuid.NewString()), nil, nil)
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "tenant.role.not-found")

	w = f.owner.DoRequest("GET", f.path("/roles/not-a-uuid"), nil, nil)
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
}

// TestRoles_Update - renames a role; a taken name, a system role, a member and an unknown role are refused.
func TestRoles_Update(t *testing.T) {
	f := newRoleFixture(t)
	createRole(t, f.owner, f.slug, "Stockers")

	w := f.owner.DoRequest("PATCH", f.path("/roles/"+f.roleID), map[string]interface{}{"name": "Tellers", "description": "Front desk"}, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, "Tellers", decode(t, w.Body.Bytes())["name"])

	w = f.owner.DoRequest("PATCH", f.path("/roles/"+f.roleID), map[string]interface{}{"name": "Stockers"}, nil)
	assert.Equal(t, http.StatusConflict, w.Code, "a taken name: %s", w.Body.String())

	w = f.owner.DoRequest("PATCH", f.path("/roles/"+uuid.NewString()), map[string]interface{}{"name": "Ghost"}, nil)
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())

	w = f.owner.DoRequest("PATCH", f.path("/roles/not-a-uuid"), map[string]interface{}{"name": "Ghost"}, nil)
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())

	w = f.member.DoRequest("PATCH", f.path("/roles/"+f.roleID), map[string]interface{}{"name": "Mine"}, nil)
	assert.Equal(t, http.StatusForbidden, w.Code, "a member edits no role: %s", w.Body.String())

	markRoleSystem(t, f.owner, f.roleID)
	w = f.owner.DoRequest("PATCH", f.path("/roles/"+f.roleID), map[string]interface{}{"name": "Renamed"}, nil)
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "tenant.role.update.system-role")
}

// TestRoles_SetPermissions - replaces a role's permissions; the role then reads them back.
func TestRoles_SetPermissions(t *testing.T) {
	f := newRoleFixture(t)
	perms := []string{"inventory:products:read", "inventory:products:write"}

	w := f.owner.DoRequest("PUT", f.path("/roles/"+f.roleID+"/permissions"), map[string]interface{}{"permissions": perms}, nil)
	require.Equal(t, http.StatusNoContent, w.Code, w.Body.String())

	w = f.owner.DoRequest("GET", f.path("/roles/"+f.roleID), nil, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.ElementsMatch(t, perms, decode(t, w.Body.Bytes())["permissions"], "the role holds exactly the set sent")

	w = f.owner.DoRequest("PUT", f.path("/roles/"+f.roleID+"/permissions"), map[string]interface{}{}, nil)
	assert.Equal(t, http.StatusBadRequest, w.Code, "permissions is required: %s", w.Body.String())

	w = f.owner.DoRequest("PUT", f.path("/roles/"+uuid.NewString()+"/permissions"), map[string]interface{}{"permissions": perms}, nil)
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())

	w = f.member.DoRequest("PUT", f.path("/roles/"+f.roleID+"/permissions"), map[string]interface{}{"permissions": perms}, nil)
	assert.Equal(t, http.StatusForbidden, w.Code, w.Body.String())

	markRoleSystem(t, f.owner, f.roleID)
	w = f.owner.DoRequest("PUT", f.path("/roles/"+f.roleID+"/permissions"), map[string]interface{}{"permissions": perms}, nil)
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
}

// TestRoles_AssignListRevoke - a role given to a member shows on both sides and goes when revoked.
func TestRoles_AssignListRevoke(t *testing.T) {
	f := newRoleFixture(t)
	assign := f.path("/users/" + f.memberID + "/roles/" + f.roleID)

	w := f.owner.DoRequest("POST", assign, nil, nil)
	require.Equal(t, http.StatusNoContent, w.Code, w.Body.String())

	w = f.owner.DoRequest("GET", f.path("/roles/"+f.roleID+"/users"), nil, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), f.memberID, "the role lists its member")

	w = f.member.DoRequest("GET", f.path("/users/"+f.memberID+"/roles"), nil, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), f.roleID, "the member holds the role")

	w = f.owner.DoRequest("DELETE", assign, nil, nil)
	require.Equal(t, http.StatusNoContent, w.Code, w.Body.String())
	w = f.owner.DoRequest("GET", f.path("/roles/"+f.roleID+"/users"), nil, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.NotContains(t, w.Body.String(), f.memberID, "revoked: the role no longer lists them")

	w = f.owner.DoRequest("POST", f.path("/users/"+uuid.NewString()+"/roles/"+f.roleID), nil, nil)
	assert.Equal(t, http.StatusNotFound, w.Code, "a non-member: %s", w.Body.String())
	assert.Contains(t, w.Body.String(), "tenant.user.not-found")

	w = f.owner.DoRequest("POST", f.path("/users/"+f.memberID+"/roles/"+uuid.NewString()), nil, nil)
	assert.Equal(t, http.StatusNotFound, w.Code, "an unknown role: %s", w.Body.String())
	assert.Contains(t, w.Body.String(), "tenant.role.not-found")

	w = f.owner.DoRequest("POST", f.path("/users/not-a-uuid/roles/"+f.roleID), nil, nil)
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	w = f.owner.DoRequest("DELETE", f.path("/users/"+f.memberID+"/roles/not-a-uuid"), nil, nil)
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())

	w = f.member.DoRequest("POST", assign, nil, nil)
	assert.Equal(t, http.StatusForbidden, w.Code, "a member grants no role: %s", w.Body.String())
	w = f.member.DoRequest("DELETE", assign, nil, nil)
	assert.Equal(t, http.StatusForbidden, w.Code, w.Body.String())

	w = f.owner.DoRequest("GET", f.path("/roles/"+uuid.NewString()+"/users"), nil, nil)
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
}

// TestRoles_Delete - a deleted role is gone; a system role, a member and an unknown role are refused.
func TestRoles_Delete(t *testing.T) {
	f := newRoleFixture(t)

	w := f.member.DoRequest("DELETE", f.path("/roles/"+f.roleID), nil, nil)
	assert.Equal(t, http.StatusForbidden, w.Code, w.Body.String())

	w = f.owner.DoRequest("DELETE", f.path("/roles/"+f.roleID), nil, nil)
	require.Equal(t, http.StatusNoContent, w.Code, w.Body.String())
	w = f.owner.DoRequest("GET", f.path("/roles/"+f.roleID), nil, nil)
	assert.Equal(t, http.StatusNotFound, w.Code, "the role is gone: %s", w.Body.String())

	w = f.owner.DoRequest("DELETE", f.path("/roles/"+uuid.NewString()), nil, nil)
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())

	system := createRole(t, f.owner, f.slug, "Built-in")
	markRoleSystem(t, f.owner, system)
	w = f.owner.DoRequest("DELETE", f.path("/roles/"+system), nil, nil)
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "tenant.role.delete.system-role")
}

// TestRoles_CrossTenantIsNotFound - one workspace's role does not exist through another's slug.
func TestRoles_CrossTenantIsNotFound(t *testing.T) {
	f := newRoleFixture(t)
	other := testhelpers.SetupApiTest(t)
	otherSlug := freshTenant(t, other)
	otherPath := "/tenants/" + otherSlug

	w := other.DoRequest("GET", otherPath+"/roles/"+f.roleID, nil, nil)
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	w = other.DoRequest("PATCH", otherPath+"/roles/"+f.roleID, map[string]interface{}{"name": "Taken"}, nil)
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	w = other.DoRequest("DELETE", otherPath+"/roles/"+f.roleID, nil, nil)
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	w = other.DoRequest("GET", otherPath+"/roles/"+f.roleID+"/users", nil, nil)
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	w = other.DoRequest("POST", otherPath+"/users/"+other.GetUserID()+"/roles/"+f.roleID, nil, nil)
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())

	w = other.DoRequest("GET", f.path("/roles"), nil, map[string]string{"X-Tenant-Slug": f.slug})
	assert.Equal(t, http.StatusForbidden, w.Code, "an outsider reads no role of the workspace: %s", w.Body.String())
}

// TestUpdateUserRole - the owner promotes a member, who can then manage roles; refusals per the SP.
func TestUpdateUserRole(t *testing.T) {
	f := newRoleFixture(t)
	rolePath := f.path("/users/" + f.memberID + "/role")

	w := f.member.DoRequest("PATCH", rolePath, map[string]interface{}{"role": "admin"}, nil)
	assert.Equal(t, http.StatusForbidden, w.Code, "a member promotes no one: %s", w.Body.String())

	w = f.owner.DoRequest("PATCH", rolePath, map[string]interface{}{"role": "admin"}, nil)
	require.Equal(t, http.StatusNoContent, w.Code, w.Body.String())
	createRole(t, f.member, f.slug, "Made by the new admin")

	w = f.member.DoRequest("PATCH", f.path("/users/"+f.owner.GetUserID()+"/role"), map[string]interface{}{"role": "owner"}, nil)
	assert.Equal(t, http.StatusForbidden, w.Code, "an admin makes no owner: %s", w.Body.String())

	w = f.owner.DoRequest("PATCH", f.path("/users/"+f.owner.GetUserID()+"/role"), map[string]interface{}{"role": "member"}, nil)
	assert.Equal(t, http.StatusBadRequest, w.Code, "nobody changes their own role: %s", w.Body.String())

	w = f.owner.DoRequest("PATCH", rolePath, map[string]interface{}{"role": "emperor"}, nil)
	assert.Equal(t, http.StatusBadRequest, w.Code, "an unknown access level: %s", w.Body.String())

	w = f.owner.DoRequest("PATCH", f.path("/users/"+uuid.NewString()+"/role"), map[string]interface{}{"role": "member"}, nil)
	assert.Equal(t, http.StatusNotFound, w.Code, "a non-member: %s", w.Body.String())

	w = f.owner.DoRequest("PATCH", f.path("/users/not-a-uuid/role"), map[string]interface{}{"role": "member"}, nil)
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
}
