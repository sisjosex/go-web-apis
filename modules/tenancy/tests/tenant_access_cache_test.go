//go:build integration
// +build integration

package tenancy_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"josex/web/modules/core/testhelpers"

	"github.com/stretchr/testify/assert"
)

// INFRA-011 D2: a member's access is cached per request chain, and removing the member drops it.

// TestTenantAccess_RemovedMemberRefusedOnNextRequest - a member reads the workspace (its access now
// cached); the owner removes them; their very next request is refused.
func TestTenantAccess_RemovedMemberRefusedOnNextRequest(t *testing.T) {
	owner := testhelpers.SetupApiTest(t)
	defer owner.Close()
	member := testhelpers.SetupApiTest(t)
	defer member.Close()

	stamp := time.Now().UnixNano()
	ownerEmail := fmt.Sprintf("cache-owner-%d@test.com", stamp)
	memberEmail := fmt.Sprintf("cache-member-%d@test.com", stamp)
	slug := fmt.Sprintf("cache-%d", stamp)

	owner.Register(ownerEmail, "$Password2025", "Cache", "Owner")
	owner.Login(ownerEmail, "$Password2025")
	w := owner.DoRequest("POST", "/tenants/self-service", map[string]interface{}{"slug": slug, "name": "Cache"}, map[string]string{})
	if w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Fatalf("create tenant: %d %s", w.Code, w.Body.String())
	}
	registered, _ := member.Register(memberEmail, "$Password2025", "Cache", "Member")
	w = owner.DoRequest("POST", "/tenants/"+slug+"/users", map[string]interface{}{"email": memberEmail, "role": "admin"}, map[string]string{})
	if w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Fatalf("add member: %d %s", w.Code, w.Body.String())
	}
	memberID := registered["id"]
	if user, ok := registered["user"].(map[string]interface{}); ok {
		memberID = user["id"]
	}

	member.Login(memberEmail, "$Password2025")
	member.SetTenantSlug(slug)
	w = member.DoRequest("GET", "/tenants/"+slug+"/modules", nil, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code, "a member reads its workspace: %s", w.Body.String())

	if id, ok := memberID.(string); ok {
		w = owner.DoRequest("DELETE", "/tenants/"+slug+"/users/"+id, nil, map[string]string{})
		assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
		w = member.DoRequest("GET", "/tenants/"+slug+"/modules", nil, map[string]string{})
		assert.Equal(t, http.StatusForbidden, w.Code, "the next request is refused: %s", w.Body.String())
	} else {
		t.Fatalf("register answered no user id: %v", registered)
	}
}
