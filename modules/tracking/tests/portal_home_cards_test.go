//go:build integration
// +build integration

package tracking_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"josex/web/modules/tracking/models"
)

// ---- MOBILE-038 D1: the guardian home's cards on /me ----

// meRider is the rider riderID as /mobile/portal/me answers it to the seeded guardian, nil when absent.
func meRider(t *testing.T, riderID string) *models.RiderStatusResponse {
	t.Helper()
	portal := SetupPortalTest(t)
	defer portal.Close()
	w := portal.DoRequest("GET", "/mobile/portal/me", nil, map[string]string{})
	if w.Code != http.StatusOK {
		t.Fatalf("me: %d %s", w.Code, w.Body.String())
	}
	var me models.PortalMe
	decodeBody(t, w, &me)
	for _, tenant := range me.Tenants {
		for i := range tenant.Riders {
			if tenant.Riders[i].RiderID.String() == riderID {
				return &tenant.Riders[i]
			}
		}
	}
	return nil
}

// TestPortalMe_HomeCards - a rider a running trip carries answers eta_at and the client's name; a rider
// absent today answers the absence and its direction, and no eta_at.
func TestPortalMe_HomeCards(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	riding, _, _ := guardianRiding(t, helper)

	absent := CreateTestRider(t, helper)
	LinkPortalGuardian(t, helper, absent)
	// The operator's own day, which the home reads the absence by: not UTC's, from 20:00 in Bolivia.
	today := dayOffset(t, organizationZone(t, helper, absent), 0)
	w := helper.DoRequest("POST", "/tracking/riders/"+absent+"/absences",
		map[string]interface{}{"date_from": today, "date_to": today, "direction": "outbound"}, map[string]string{})
	if w.Code != http.StatusCreated {
		t.Fatalf("absence: %d %s", w.Code, w.Body.String())
	}

	if live := meRider(t, riding); assert.NotNil(t, live) {
		assert.NotNil(t, live.EtaAt, "the ETA to the stop the rider waits at")
		if assert.NotNil(t, live.OrganizationName) {
			assert.NotEmpty(t, *live.OrganizationName)
		}
		assert.Nil(t, live.AbsenceID)
	}
	if away := meRider(t, absent); assert.NotNil(t, away) {
		assert.Nil(t, away.EtaAt, "no trip carries an absent rider")
		assert.NotNil(t, away.AbsenceID)
		if assert.NotNil(t, away.AbsenceDirection) {
			assert.Equal(t, "outbound", *away.AbsenceDirection)
		}
		assert.NotNil(t, away.AbsenceCreatedAt)
	}
}
