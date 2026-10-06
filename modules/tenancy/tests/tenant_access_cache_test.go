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
	if _, err := member.Register(memberEmail, "$Password2025", "Cache", "Member"); err != nil {
		t.Fatalf("register member: %v", err)
	}
	if _, err := member.Login(memberEmail, "$Password2025"); err != nil {
		t.Fatalf("member login: %v", err)
	}
	memberID := member.GetUserID()
	w = owner.DoRequest("POST", "/tenants/"+slug+"/users", map[string]interface{}{"user_id": memberID, "role": "admin"}, map[string]string{})
	if w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Fatalf("add member: %d %s", w.Code, w.Body.String())
	}

	member.SetTenantSlug(slug)
	w = member.DoRequest("GET", "/tenants/"+slug+"/modules", nil, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code, "a member reads its workspace: %s", w.Body.String())

	if memberID != "" {
		id := memberID
		w = owner.DoRequest("DELETE", "/tenants/"+slug+"/users/"+id, nil, map[string]string{})
		assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
		w = member.DoRequest("GET", "/tenants/"+slug+"/modules", nil, map[string]string{})
		assert.Equal(t, http.StatusForbidden, w.Code, "the next request is refused: %s", w.Body.String())
	} else {
		t.Fatal("the member has no id")
	}
}
