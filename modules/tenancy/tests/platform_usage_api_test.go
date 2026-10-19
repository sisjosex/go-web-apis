//go:build integration
// +build integration

package tenancy_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	tenancyModels "josex/web/modules/tenancy/models"
	tenancyRepos "josex/web/modules/tenancy/repositories"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TRACK-013 D3: what each business used per day, read by the platform.

const usageTenantID = "00000000-0000-0000-0000-000000000001"

// TestPlatformUsage_SuperAdminReadsDays - two days written through the SP read back for the test
// business with their peak and mean; a member of the business is refused.
func TestPlatformUsage_SuperAdminReadsDays(t *testing.T) {
	helper := SetupTenancyTest(t)
	defer helper.Close()

	// Days far enough back that no rollup of another test writes them.
	day1 := time.Date(2020, 3, 1, 0, 0, 0, 0, time.UTC)
	day2 := day1.AddDate(0, 0, 1)
	repo := tenancyRepos.NewUsageRepository(helper.DB())
	tenantID := uuid.MustParse(usageTenantID)
	require.NoError(t, repo.UpsertUsageDaily(context.Background(), tenantID, day1, 1, 2, 100, 5))
	require.NoError(t, repo.UpsertUsageDaily(context.Background(), tenantID, day2, 2, 3, 200, 6))
	require.NoError(t, repo.UpsertUsageDaily(context.Background(), tenantID, day2, 3, 3, 200, 6), "a second pass replaces the day")

	_, err := helper.Login("superadmin@test.local", "SuperAdmin123!")
	require.NoError(t, err)
	w := helper.DoRequest("GET", "/platform/usage?from=2020-03-01&to=2020-03-02&tenant_id="+usageTenantID, nil, map[string]string{})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var body tenancyModels.PlatformUsageResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	if assert.Len(t, body.Tenants, 1) {
		got := body.Tenants[0]
		assert.Equal(t, tenantID, got.TenantID)
		assert.Len(t, got.Days, 2)
		assert.Equal(t, 3, got.VehiclesActiveMax)
		assert.Equal(t, 2.0, got.VehiclesActiveAvg)
		assert.Equal(t, 5, got.TripsCompleted)
	}

	w = helper.DoRequest("GET", "/platform/usage?from=2019-01-01&to=2020-03-02", nil, map[string]string{})
	assert.Equal(t, http.StatusBadRequest, w.Code, "over 366 days")
	assert.Contains(t, w.Body.String(), "tenant.usage.range-too-wide")

	_, err = helper.Login("admin@test.local", "Admin123!")
	require.NoError(t, err)
	w = helper.DoRequest("GET", "/platform/usage", nil, map[string]string{})
	assert.Equal(t, http.StatusForbidden, w.Code, "a business owner is not the platform")
}
