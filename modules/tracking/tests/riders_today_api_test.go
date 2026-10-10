//go:build integration
// +build integration

package tracking_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

// MOBILE-026 D1: the guardian home reads every linked rider's status in one call.

type ridersToday struct {
	Riders []struct {
		RiderID   string `json:"rider_id"`
		RiderName string `json:"rider_name"`
	} `json:"riders"`
}

// TestRidersToday_GuardianGetsEachLinkedRider - the seeded guardian is linked to John: the home lists
// him with the same status fields GET /riders/{id}/status answers.
func TestRidersToday_GuardianGetsEachLinkedRider(t *testing.T) {
	helper := SetupPortalTest(t)
	defer helper.Close()

	w := helper.DoRequest("GET", "/tracking/riders/today", nil, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var body ridersToday
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	ids := []string{}
	for _, r := range body.Riders {
		ids = append(ids, r.RiderID)
		assert.NotEmpty(t, r.RiderName)
	}
	assert.Contains(t, ids, TestRiderJohnID)
}

// TestRidersToday_UnlinkedGuardianReadsEmpty - no rider linked yet is an empty home, not a refusal.
func TestRidersToday_UnlinkedGuardianReadsEmpty(t *testing.T) {
	helper := SetupUnlinkedPortalTest(t)
	defer helper.Close()

	w := helper.DoRequest("GET", "/tracking/riders/today", nil, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.JSONEq(t, `{"riders":[]}`, w.Body.String())
}

// TestRidersToday_StaffIsRefused - for an operator "my riders" is the whole tenant: no home lists that.
func TestRidersToday_StaffIsRefused(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	w := helper.DoRequest("GET", "/tracking/riders/today", nil, map[string]string{})
	assert.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
}
