//go:build integration
// +build integration

package tracking_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// TRACK-050 D2: the tracking home's setup assistant asks what the tenant has set up yet.

// TestTrackingSetup_FreshTenantThenCompany - a tenant with nothing has every step open; its first
// company closes only the company step, and nothing of another tenant counts.
func TestTrackingSetup_FreshTenantThenCompany(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	tenantID := uuid.NewString()

	assert.Equal(t, [5]bool{false, false, false, false, false}, TrackingSetupFlags(t, helper, tenantID), "a fresh tenant has nothing")

	SeedTenantCompany(t, helper, tenantID)
	assert.Equal(t, [5]bool{true, false, false, false, false}, TrackingSetupFlags(t, helper, tenantID), "the first company closes the company step")
}

// TestTrackingSetup_Endpoint - the seeded test tenant has every piece, read over GET /tracking/setup.
func TestTrackingSetup_Endpoint(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	w := helper.DoRequest("GET", "/tracking/setup", nil, map[string]string{})
	expectStatus(t, w, http.StatusOK)
	body := ParseResponse(t, w.Body.Bytes())
	for _, key := range []string{"companies", "vehicles", "routes", "riders"} {
		assert.Equal(t, true, body[key], key)
	}
	_, hasDrivers := body["drivers"]
	assert.True(t, hasDrivers, "drivers is answered")
}
