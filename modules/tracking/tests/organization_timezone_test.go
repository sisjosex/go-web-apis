//go:build integration
// +build integration

package tracking_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOrganizationTimezone_DefaultsToWorkspace - TENANCY-007 step 2: a client created without a zone
// takes the workspace's `general.timezone`; one sent keeps its own.
func TestOrganizationTimezone_DefaultsToWorkspace(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	w := helper.DoRequest("PATCH", "/tenants/test-company/settings", map[string]interface{}{"general.timezone": "America/Santiago"}, map[string]string{})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	defer helper.DoRequest("PATCH", "/tenants/test-company/settings", map[string]interface{}{"general.timezone": "America/La_Paz"}, map[string]string{})

	client := createPinnedClient(t, helper, map[string]interface{}{"timezone": ""})
	assert.Equal(t, "America/Santiago", client["timezone"], "no zone sent: the workspace's")
	client = createPinnedClient(t, helper, nil)
	assert.Equal(t, "America/Lima", client["timezone"], "a zone sent is kept")
}
