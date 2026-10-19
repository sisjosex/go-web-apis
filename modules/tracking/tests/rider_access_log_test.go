//go:build integration
// +build integration

package tracking_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"

	coreJobs "josex/web/modules/core/jobs"
	trackingJobs "josex/web/modules/tracking/jobs"
	"josex/web/modules/tracking/models"
)

// TRACK-063: who outside the operator read a rider, and the contact retention.

// ---- step 1: the log and its writers ----

// TestRiderAccessLog_GuardianReadsCountInOneRow - D1: a guardian's status read twice in the hour is one
// row with reads = 2; the operator's same read leaves none (D2).
func TestRiderAccessLog_GuardianReadsCountInOneRow(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	riderID := CreateTestRider(t, helper)
	LinkPortalGuardian(t, helper, riderID)
	clearAccessLog(t, helper, riderID)

	w := helper.DoRequest("GET", "/tracking/riders/"+riderID+"/status", nil, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Empty(t, accessRows(t, helper, riderID), "the operator's read is not logged")

	portal := SetupPortalTest(t)
	defer portal.Close()
	for range 2 {
		w = portal.DoRequest("GET", "/tracking/riders/"+riderID+"/status", nil, map[string]string{})
		assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	}
	assert.Equal(t, []accessRow{{"portal", "status", 2}}, accessRows(t, helper, riderID))
}

// TestRiderAccessLog_EachPortalRead - the home logs 'today' for each of the guardian's riders and not
// 'status' again; the live map and the absences log their own action; an organization user's read is
// 'organization'.
func TestRiderAccessLog_EachPortalRead(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	riderID := CreateTestRider(t, helper)
	LinkPortalGuardian(t, helper, riderID)
	clearAccessLog(t, helper, riderID)

	portal := SetupPortalTest(t)
	defer portal.Close()
	for _, path := range []string{"/tracking/riders/today", "/tracking/riders/" + riderID + "/live", "/tracking/riders/" + riderID + "/absences"} {
		w := portal.DoRequest("GET", path, nil, map[string]string{})
		assert.Equal(t, http.StatusOK, w.Code, path+": "+w.Body.String())
	}
	assert.ElementsMatch(t, []accessRow{{"portal", "today", 1}, {"portal", "live", 1}, {"portal", "absences", 1}},
		accessRows(t, helper, riderID))

	org := SetupOrganizationTest(t)
	defer org.Close()
	clearAccessLog(t, helper, riderID)
	w := org.DoRequest("GET", "/tracking/riders/"+riderID+"/status", nil, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, []accessRow{{"organization", "status", 1}}, accessRows(t, helper, riderID))
}

// TestRiderAccessLog_DeactivationStampsTheClock - D3: turning a rider inactive stamps deactivated_at,
// active again clears it.
func TestRiderAccessLog_DeactivationStampsTheClock(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	riderID := CreateTestRider(t, helper)

	deactivated := func() bool { return RiderDeactivated(t, helper, riderID) }
	w := helper.DoRequest("PATCH", "/tracking/riders/"+riderID, map[string]interface{}{"is_active": false}, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.True(t, deactivated())

	w = helper.DoRequest("PATCH", "/tracking/riders/"+riderID, map[string]interface{}{"is_active": true}, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.False(t, deactivated())
}

// ---- step 2: the staff read ----

// TestRiderAccessLog_StaffReadsTheTrail - the operator lists the guardian's read with their email; an
// organization user is refused, and a rider of no tenant of the caller is not found.
func TestRiderAccessLog_StaffReadsTheTrail(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	riderID := CreateTestRider(t, helper)
	LinkPortalGuardian(t, helper, riderID)
	clearAccessLog(t, helper, riderID)

	portal := SetupPortalTest(t)
	defer portal.Close()
	w := portal.DoRequest("GET", "/tracking/riders/"+riderID+"/status", nil, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())

	w = helper.DoRequest("GET", "/tracking/riders/"+riderID+"/audit?page=1&page_size=20", nil, map[string]string{})
	if !assert.Equal(t, http.StatusOK, w.Code, w.Body.String()) {
		return
	}
	var page models.ListRiderAccessLogResponse
	decodeBody(t, w, &page)
	assert.EqualValues(t, 1, page.TotalCount)
	if assert.Len(t, page.Entries, 1) {
		e := page.Entries[0]
		assert.Equal(t, "portal@test.local", *e.Email)
		assert.Equal(t, "portal", e.Level)
		assert.Equal(t, "status", e.Action)
		assert.EqualValues(t, 1, e.Reads)
	}

	// A period before the read has nothing.
	w = helper.DoRequest("GET", "/tracking/riders/"+riderID+"/audit?from=2020-01-01&to=2020-01-31", nil, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"entries":[]`)

	org := SetupOrganizationTest(t)
	defer org.Close()
	w = org.DoRequest("GET", "/tracking/riders/"+riderID+"/audit", nil, map[string]string{})
	assert.Equal(t, http.StatusForbidden, w.Code, w.Body.String())

	w = helper.DoRequest("GET", "/tracking/riders/"+uuid.NewString()+"/audit", nil, map[string]string{})
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
}

// ---- step 3: the retention job ----

// TestContactRetention_ClearsPastTheTenantRetention - D3: a rider inactive for 91 days loses its contact
// data and its guardian's link but keeps its name; one inactive since yesterday is untouched; a tenant
// whose settings say 30 days clears one inactive for 31. Log rows past 365 days go, recent ones stay.
func TestContactRetention_ClearsPastTheTenantRetention(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	ctx := context.Background()
	old, recent, month := CreateTestRider(t, helper), CreateTestRider(t, helper), CreateTestRider(t, helper)
	LinkPortalGuardian(t, helper, old)
	inactiveSince(t, helper, old, 91)
	inactiveSince(t, helper, recent, 1)
	inactiveSince(t, helper, month, 31)
	clearAccessLog(t, helper, recent)
	SeedAccessLogAges(t, helper, recent)

	run := func(tenant coreJobs.Tenant) {
		t.Helper()
		lister := coreJobs.NewTenantDirectory(func(context.Context) ([]coreJobs.Tenant, error) { return []coreJobs.Tenant{tenant}, nil })
		assert.NoError(t, trackingJobs.ContactRetentionHandler(helper.DB(), lister, 90, 365)(ctx, asynq.NewTask(trackingJobs.TaskContactRetention, nil)))
	}
	run(testTenant())
	assert.True(t, anonymised(t, helper, old))
	assert.Equal(t, 1, ClearedContacts(t, helper, old), "the guardian is unlinked")
	assert.False(t, anonymised(t, helper, recent))
	assert.False(t, anonymised(t, helper, month), "31 days is inside the default 90")
	assert.Equal(t, []accessRow{{"portal", "live", 1}}, accessRows(t, helper, recent), "the 400-day-old row is pruned")

	tenant := testTenant()
	tenant.Settings = map[string]any{"tracking.contact_retention_days": float64(30)}
	run(tenant)
	assert.True(t, anonymised(t, helper, month))
	assert.False(t, anonymised(t, helper, recent))
}
