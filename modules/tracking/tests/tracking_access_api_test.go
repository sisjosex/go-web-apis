//go:build integration
// +build integration

package tracking_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"josex/web/modules/core/testhelpers"
)

// App access granted from Tracking (TRACK-032). Every test works on its own rider, driver or
// organization and its own fresh emails, so the seeded accounts other tests sign in with keep their
// memberships.

func newAccessEmail() string {
	return "access-" + uuid.New().String()[:8] + "@test.local"
}

func accessPerson(email string) map[string]interface{} {
	return map[string]interface{}{"email": email, "first_name": "Ana", "last_name": "Quispe", "phone": "+59170000000"}
}

func createAccessRider(t *testing.T, helper *testhelpers.ApiTestHelper) string {
	t.Helper()
	w := helper.DoRequest("POST", "/tracking/riders", ValidRiderDto(), map[string]string{})
	if w.Code != http.StatusCreated {
		t.Fatalf("create rider: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	return ExtractID(t, ParseResponse(t, w.Body.Bytes()))
}

// TestRiderGuardianAccess - a new email becomes a portal account and the rider's guardian; the same
// email on a second rider links the same account; removing the last link ends the membership.
func TestRiderGuardianAccess(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	first, second := createAccessRider(t, helper), createAccessRider(t, helper)
	email := newAccessEmail()

	w := helper.DoRequest("POST", fmt.Sprintf("/tracking/riders/%s/guardians", first), accessPerson(email), map[string]string{})
	if w.Code != http.StatusCreated {
		t.Fatalf("add guardian: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	granted := ParseResponse(t, w.Body.Bytes())
	userID := granted["user_id"].(string)
	assert.Equal(t, "new", granted["status"])
	assert.Equal(t, email, granted["email"])
	assert.Contains(t, granted["notice"], email, "the notice names the email to sign in with")

	role, active := TenantMembership(t, helper, userID)
	assert.Equal(t, "portal", role)
	assert.True(t, active)

	w = helper.DoRequest("GET", fmt.Sprintf("/tracking/riders/%s/guardians", first), nil, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	guardians := ParseListResponse(t, w.Body.Bytes())
	if assert.Len(t, guardians, 1) {
		assert.Equal(t, userID, guardians[0]["user_id"])
		assert.NotEmpty(t, guardians[0]["notice"])
	}

	// The same email on another rider: linked, no second account.
	w = helper.DoRequest("POST", fmt.Sprintf("/tracking/riders/%s/guardians", second), accessPerson(email), map[string]string{})
	assert.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	again := ParseResponse(t, w.Body.Bytes())
	assert.Equal(t, "linked", again["status"])
	assert.Equal(t, userID, again["user_id"])
	assert.Equal(t, 1, CountUsersByEmail(t, helper, email))

	// By user_id, twice on the same rider: idempotent.
	w = helper.DoRequest("POST", fmt.Sprintf("/tracking/riders/%s/guardians", second), map[string]interface{}{"user_id": userID}, map[string]string{})
	assert.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	// Off the first rider: still linked to the second, so the membership stays.
	w = helper.DoRequest("DELETE", fmt.Sprintf("/tracking/riders/%s/guardians/%s", first, userID), nil, map[string]string{})
	assert.Equal(t, http.StatusNoContent, w.Code, w.Body.String())
	_, active = TenantMembership(t, helper, userID)
	assert.True(t, active, "a guardian of another rider keeps the app")

	// Off the last rider: the membership ends.
	w = helper.DoRequest("DELETE", fmt.Sprintf("/tracking/riders/%s/guardians/%s", second, userID), nil, map[string]string{})
	assert.Equal(t, http.StatusNoContent, w.Code, w.Body.String())
	_, active = TenantMembership(t, helper, userID)
	assert.False(t, active, "the last unlink ends the portal membership")

	w = helper.DoRequest("DELETE", fmt.Sprintf("/tracking/riders/%s/guardians/%s", second, userID), nil, map[string]string{})
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
}

// TestRiderGuardianRefusesOtherLevels - a web account and another app level are refused (D4), and a
// refused grant leaves no membership behind.
func TestRiderGuardianRefusesOtherLevels(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	rider := createAccessRider(t, helper)

	w := helper.DoRequest("POST", fmt.Sprintf("/tracking/riders/%s/guardians", rider), accessPerson("admin@test.local"), map[string]string{})
	assert.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "tracking.access.web-account")

	w = helper.DoRequest("POST", fmt.Sprintf("/tracking/riders/%s/guardians", rider), accessPerson("orguser@test.local"), map[string]string{})
	assert.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "tracking.access.level-mismatch")

	// A rider that does not exist: the new account gets no membership out of the failed link.
	email := newAccessEmail()
	w = helper.DoRequest("POST", fmt.Sprintf("/tracking/riders/%s/guardians", uuid.New()), accessPerson(email), map[string]string{})
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	if userID := UserIDByEmail(t, helper, email); userID != "" {
		_, active := TenantMembership(t, helper, userID)
		assert.False(t, active, "a failed link takes back the membership the grant added")
	}

	w = helper.DoRequest("POST", fmt.Sprintf("/tracking/riders/%s/guardians", rider), map[string]interface{}{"email": newAccessEmail()}, map[string]string{})
	assert.Equal(t, http.StatusBadRequest, w.Code, "a new person needs a name")
}

// TestDriverAccountAccess - PUT gives a driver an account at the driver level, DELETE takes it away
// and ends the membership.
func TestDriverAccountAccess(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	driverID := CreateTestDriver(t, helper)
	email := newAccessEmail()

	w := helper.DoRequest("PUT", fmt.Sprintf("/tracking/drivers/%s/account", driverID), accessPerson(email), map[string]string{})
	if w.Code != http.StatusCreated {
		t.Fatalf("set driver account: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	userID := ParseResponse(t, w.Body.Bytes())["user_id"].(string)
	role, active := TenantMembership(t, helper, userID)
	assert.Equal(t, "driver", role)
	assert.True(t, active)

	w = helper.DoRequest("GET", "/tracking/drivers/"+driverID, nil, map[string]string{})
	driver := ParseResponse(t, w.Body.Bytes())
	assert.Equal(t, userID, driver["user_id"])
	assert.Equal(t, email, driver["user_email"])

	// A portal account is not a driver (D4).
	w = helper.DoRequest("PUT", fmt.Sprintf("/tracking/drivers/%s/account", driverID), accessPerson("portal@test.local"), map[string]string{})
	assert.Equal(t, http.StatusConflict, w.Code, w.Body.String())

	w = helper.DoRequest("DELETE", fmt.Sprintf("/tracking/drivers/%s/account", driverID), nil, map[string]string{})
	assert.Equal(t, http.StatusNoContent, w.Code, w.Body.String())
	_, active = TenantMembership(t, helper, userID)
	assert.False(t, active)

	w = helper.DoRequest("GET", "/tracking/drivers/"+driverID, nil, map[string]string{})
	assert.Nil(t, ParseResponse(t, w.Body.Bytes())["user_id"])
}

// TestOrganizationMemberInvite - POST adds a member found or created at the organization level.
func TestOrganizationMemberInvite(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	organizationID := createOrganization(t, helper)
	body := accessPerson(newAccessEmail())
	body["role"] = "viewer"

	w := helper.DoRequest("POST", fmt.Sprintf("/tracking/organizations/%s/members", organizationID), body, map[string]string{})
	if w.Code != http.StatusCreated {
		t.Fatalf("invite member: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	userID := ParseResponse(t, w.Body.Bytes())["user_id"].(string)
	role, _ := TenantMembership(t, helper, userID)
	assert.Equal(t, "organization", role)

	members := listMembers(t, helper, organizationID)
	if assert.Len(t, members, 1) {
		assert.Equal(t, "viewer", members[0]["role"])
	}

	staff := map[string]interface{}{"user_id": helper.GetUserID(), "role": "admin"}
	w = helper.DoRequest("POST", fmt.Sprintf("/tracking/organizations/%s/members", organizationID), staff, map[string]string{})
	assert.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "tracking.access.web-account")
}
