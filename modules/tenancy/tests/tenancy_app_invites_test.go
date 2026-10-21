//go:build integration
// +build integration

package tenancy_test

import (
	"context"
	"testing"
	"time"

	coreTestHelpers "josex/web/modules/core/testhelpers"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TRACK-062 D3: the invitation comes with the app access, once per account and business, and the
// account's first sign-in accepts every one of them.

type appInvite struct {
	channel     string
	destination *string
	sentAt      *time.Time
	acceptedAt  *time.Time
}

func grantApp(t *testing.T, helper *coreTestHelpers.ApiTestHelper, tenantID, email, phone, channel string) (string, bool) {
	t.Helper()
	var userID string
	var added bool
	require.NoError(t, helper.DB().QueryRow(context.Background(), `
		SELECT user_id, membership_added FROM tenancy.sp_grant_app_access(
			p_tenant_id := $1, p_user_id := NULL, p_email := $2, p_first_name := 'Ana', p_last_name := 'Rojas',
			p_phone := $3, p_level := 'portal', p_channel := $4)`, tenantID, email, phone, channel).Scan(&userID, &added))
	return userID, added
}

func inviteOf(t *testing.T, helper *coreTestHelpers.ApiTestHelper, tenantID, userID string) *appInvite {
	t.Helper()
	var inv appInvite
	err := helper.DB().QueryRow(context.Background(), `
		SELECT channel, destination, sent_at, accepted_at FROM tenancy.sp_list_app_invites($1, ARRAY[$2::UUID])`,
		tenantID, userID).Scan(&inv.channel, &inv.destination, &inv.sentAt, &inv.acceptedAt)
	if err != nil {
		return nil
	}
	return &inv
}

// TestAppInvites_GrantInTwoBusinessesAcceptOnce - one email granted by two businesses is one account
// with two memberships and two invitations: the email one went out with the grant, the WhatsApp one
// waits to be sent; one accept marks both; a revoke drops its business's invitation.
func TestAppInvites_GrantInTwoBusinessesAcceptOnce(t *testing.T) {
	helper := SetupTenancyTest(t)
	defer helper.Close()
	first := usageTenantID
	second := secondBusiness(t, helper)
	email := "invite-" + uuid.NewString()[:8] + "@test.local"

	userID, added := grantApp(t, helper, first, email, "+59170000001", "email")
	assert.True(t, added)
	again, _ := grantApp(t, helper, second, email, "+59170000001", "whatsapp")
	assert.Equal(t, userID, again, "one account")

	assert.Equal(t, 2, activeMemberships(t, helper, userID))

	byEmail, byWhatsApp := inviteOf(t, helper, first, userID), inviteOf(t, helper, second, userID)
	if assert.NotNil(t, byEmail) && assert.NotNil(t, byWhatsApp) {
		assert.Equal(t, "email", byEmail.channel)
		assert.NotNil(t, byEmail.sentAt, "the access email went out with the grant")
		assert.Equal(t, "whatsapp", byWhatsApp.channel)
		if assert.NotNil(t, byWhatsApp.destination) {
			assert.Equal(t, "+59170000001", *byWhatsApp.destination)
		}
		assert.Nil(t, byWhatsApp.sentAt, "the operator has not sent it yet")
	}
	var sent time.Time
	require.NoError(t, helper.DB().QueryRow(context.Background(),
		`SELECT tenancy.sp_mark_app_invite_sent($1, $2)`, second, userID).Scan(&sent))
	assert.NotNil(t, inviteOf(t, helper, second, userID).sentAt)

	var accepted int
	require.NoError(t, helper.DB().QueryRow(context.Background(), `SELECT tenancy.sp_accept_app_invites($1)`, userID).Scan(&accepted))
	assert.Equal(t, 2, accepted)
	assert.NotNil(t, inviteOf(t, helper, first, userID).acceptedAt)
	assert.NotNil(t, inviteOf(t, helper, second, userID).acceptedAt)

	var revoked bool
	require.NoError(t, helper.DB().QueryRow(context.Background(),
		`SELECT tenancy.sp_revoke_app_access($1, $2, 'portal')`, second, userID).Scan(&revoked))
	assert.True(t, revoked)
	assert.Nil(t, inviteOf(t, helper, second, userID), "a revoked access has no invitation")
	assert.NotNil(t, inviteOf(t, helper, first, userID))
}

// TestAppInvites_SignInAccepts - a password sign-in runs the session hook: the account's open
// invitation is accepted.
func TestAppInvites_SignInAccepts(t *testing.T) {
	helper := SetupTenancyTest(t)
	defer helper.Close()
	second := secondBusiness(t, helper)
	userID, _ := grantApp(t, helper, second, "portal-unlinked@test.local", "", "email")
	assert.Nil(t, inviteOf(t, helper, second, userID).acceptedAt)

	_, err := helper.Login("portal-unlinked@test.local", "Portal123!")
	require.NoError(t, err)
	assert.NotNil(t, inviteOf(t, helper, second, userID).acceptedAt)
}
