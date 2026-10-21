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
	"josex/web/modules/tracking/models"
)

// TRACK-062: the families of an organization or of a selection of riders get the app in one action.

func strp(s string) *string { return &s }

func listInvites(t *testing.T, helper *testhelpers.ApiTestHelper, organizationID, query string) models.ListOrganizationInvitesResponse {
	t.Helper()
	w := helper.DoRequest("GET", "/tracking/organizations/"+organizationID+"/invites"+query, nil, map[string]string{})
	if w.Code != http.StatusOK {
		t.Fatalf("list invites %s: %d %s", query, w.Code, w.Body.String())
	}
	var res models.ListOrganizationInvitesResponse
	decodeBody(t, w, &res)
	return res
}

// TestInvites_OrganizationEveryone - an organization's contacts: one person guarding two siblings
// (email and phone), one with only an email, one with only a phone and one with neither → two
// invited, the phone-only one skipped no-email and the bare one no-destination. The WhatsApp one is
// pending until marked sent, the emailed one went out; both siblings' rows carry the account.
func TestInvites_OrganizationEveryone(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	org := createOrganization(t, helper)
	rider := func() string { return createRider(t, helper, map[string]interface{}{"organization_id": org}) }
	sibling1, sibling2, single, phoneOnly, bare := rider(), rider(), rider(), rider(), rider()
	parent := "invite-" + uuid.NewString()[:8] + "@test.local"
	other := "invite-" + uuid.NewString()[:8] + "@test.local"
	phone := "+5917" + uuid.NewString()[:7]
	guardianContact(t, helper, sibling1, strp("Lucía Paz Rojas"), strp(phone), strp(parent))
	guardianContact(t, helper, sibling2, strp("Lucía Paz"), nil, strp(parent))
	guardianContact(t, helper, single, strp("Mario"), nil, strp(other))
	phoneContact := guardianContact(t, helper, phoneOnly, strp("Rosa"), strp("+59171111111"), nil)
	bareContact := guardianContact(t, helper, bare, strp("Sin datos"), nil, nil)

	w := helper.DoRequest("POST", "/tracking/invites", map[string]interface{}{"organization_id": org}, map[string]string{})
	if w.Code != http.StatusOK {
		t.Fatalf("invite: %d %s", w.Code, w.Body.String())
	}
	var result models.InviteResult
	decodeBody(t, w, &result)
	assert.Equal(t, 2, result.Invited)
	reasons := map[string]string{}
	for _, s := range result.Skipped {
		reasons[s.ContactID.String()] = s.Reason
	}
	assert.Equal(t, map[string]string{phoneContact: "no-email", bareContact: "no-destination"}, reasons)
	channels := map[string]models.InviteItem{}
	for _, item := range result.Items {
		channels[item.Channel] = item
		assert.NotEmpty(t, item.Message)
	}
	whatsapp, email := channels["whatsapp"], channels["email"]
	assert.Equal(t, "pending", whatsapp.Status)
	assert.Equal(t, "sent", email.Status)

	assert.Equal(t, 2, ContactsLinkedTo(t, helper, whatsapp.UserID.String(), sibling1, sibling2), "one account, both siblings")

	all := listInvites(t, helper, org, "")
	assert.EqualValues(t, 2, all.TotalCount)
	assert.EqualValues(t, 1, listInvites(t, helper, org, "?status=pending").TotalCount)

	w = helper.DoRequest("POST", fmt.Sprintf("/tracking/invites/%s/sent", whatsapp.UserID), nil, map[string]string{})
	assert.Equal(t, http.StatusNoContent, w.Code, w.Body.String())
	assert.EqualValues(t, 2, listInvites(t, helper, org, "?status=sent").TotalCount)

	w = helper.DoRequest("POST", fmt.Sprintf("/tracking/invites/%s/resend", email.UserID), nil, map[string]string{})
	if assert.Equal(t, http.StatusOK, w.Code, w.Body.String()) {
		var msg models.InviteMessage
		decodeBody(t, w, &msg)
		assert.Contains(t, msg.Message, other)
	}
	assert.Equal(t, http.StatusNotFound, helper.DoRequest("POST", "/tracking/invites/"+uuid.NewString()+"/sent", nil, map[string]string{}).Code)

	w = helper.DoRequest("GET", "/tracking/riders/"+sibling1+"/guardians", nil, map[string]string{})
	var guardians []models.RiderGuardian
	decodeBody(t, w, &guardians)
	found := false
	for _, g := range guardians {
		if g.UserID != nil && *g.UserID == whatsapp.UserID && g.Invite != nil {
			found = true
			assert.Equal(t, "sent", g.Invite.Status)
			assert.Equal(t, "whatsapp", g.Invite.Channel)
		}
	}
	assert.True(t, found, "the family row carries its invitation")

	w = helper.DoRequest("POST", "/tracking/invites", map[string]interface{}{}, map[string]string{})
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
}

// TestInvites_OrganizationAdminSelection - an organization admin invites a selection of its own
// riders; another organization is not found to it.
func TestInvites_OrganizationAdminSelection(t *testing.T) {
	staff := SetupTrackingTest(t)
	defer staff.Close()
	own := CreateTestRider(t, staff)
	email := "invite-" + uuid.NewString()[:8] + "@test.local"
	guardianContact(t, staff, own, strp("Elena Vargas"), nil, strp(email))
	foreign := createOrganization(t, staff)
	setOrgUserMembership(t, staff, "admin", nil)
	org := SetupOrganizationTest(t)
	defer org.Close()

	w := org.DoRequest("POST", "/tracking/invites", map[string]interface{}{"rider_ids": []string{own}}, map[string]string{})
	if assert.Equal(t, http.StatusOK, w.Code, w.Body.String()) {
		var result models.InviteResult
		decodeBody(t, w, &result)
		assert.Equal(t, 1, result.Invited)
	}
	assert.Equal(t, http.StatusOK, org.DoRequest("GET", "/tracking/organizations/"+MainSchoolID+"/invites", nil, map[string]string{}).Code)
	w = org.DoRequest("POST", "/tracking/invites", map[string]interface{}{"organization_id": foreign}, map[string]string{})
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
}
