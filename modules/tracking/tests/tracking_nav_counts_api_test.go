//go:build integration
// +build integration

package tracking_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	coreJobs "josex/web/modules/core/jobs"
	"josex/web/modules/core/testhelpers"
	trackingJobs "josex/web/modules/tracking/jobs"
	"josex/web/modules/tracking/models"
)

// TRACK-081: the rail's badges in one read, and pushed on nav:{tenant} when they move.

func navCounts(t *testing.T, helper *testhelpers.ApiTestHelper) models.NavCounts {
	t.Helper()
	var counts models.NavCounts
	decodeBody(t, helper.DoRequest("GET", "/tracking/nav-counts", nil, map[string]string{}), &counts)
	return counts
}

// TestNavCounts_AdminCounts - an active alert and the documents expired or due within 30 days are
// counted; one due in 31 days and a resolved alert are not.
func TestNavCounts_AdminCounts(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	routeID := CreateTestRoute(t, helper)
	before := navCounts(t, helper)
	if !assert.NotNil(t, before.AlertsActive) || !assert.NotNil(t, before.DocumentsExpiring) {
		return
	}

	typeID := CreateTestDocumentType(t, helper, "vehicle", false)
	CreateTestDocument(t, helper, "vehicle", TestBusID, typeID, -3)
	CreateTestDocument(t, helper, "vehicle", TestBusID, typeID, 10)
	CreateTestDocument(t, helper, "vehicle", TestBusID, typeID, 31)
	raiseAlert(t, helper, routeID, "delay", "medium", "Heavy rain")
	resolved := raiseAlert(t, helper, routeID, "delay", "low", "Cleared")
	w := helper.DoRequest("PATCH", "/tracking/alerts/"+resolved+"/resolve", nil, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())

	after := navCounts(t, helper)
	assert.Equal(t, *before.AlertsActive+1, *after.AlertsActive)
	assert.Equal(t, *before.DocumentsExpiring+2, *after.DocumentsExpiring)
}

// TestNavCounts_OrganizationUser - an organization user reads its trips only: no alerts, no documents.
func TestNavCounts_OrganizationUser(t *testing.T) {
	org := SetupOrganizationTest(t)
	defer org.Close()

	w := org.DoRequest("GET", "/tracking/nav-counts", nil, map[string]string{})
	expectStatus(t, w, http.StatusOK)
	body := ParseResponse(t, w.Body.Bytes())
	assert.Contains(t, body, "trips_in_progress")
	assert.NotContains(t, body, "alerts_active")
	assert.NotContains(t, body, "documents_expiring")
}

// TestNavCounts_ChannelAccess - only the tenant's operators may listen to nav:{tenant}.
func TestNavCounts_ChannelAccess(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	can := func(email, channel string, guardian, organization bool) bool {
		t.Helper()
		var allowed bool
		if err := helper.DB().QueryRow(context.Background(), `
			SELECT tracking.sp_can_subscribe(p_tenant_id := $1, p_user_id := u.id, p_guardian := $3, p_channel := $2, p_organization := $4)
			FROM auth.users u WHERE u.email = $5`,
			TestTenantID, channel, guardian, organization, email).Scan(&allowed); err != nil {
			t.Fatalf("can subscribe: %v", err)
		}
		return allowed
	}
	own := trackingJobs.NavChannel(TestTenantID)
	assert.True(t, can("admin@test.local", own, false, false), "an operator listens to its tenant")
	assert.False(t, can("admin@test.local", trackingJobs.NavChannel(uuid.NewString()), false, false), "not another tenant's")
	assert.False(t, can("portal@test.local", own, true, false), "a guardian does not")
	assert.False(t, can("orguser@test.local", own, false, true), "an organization user does not")
}

// TestNavCounts_FrameOnAlert - raising and resolving an alert each push the new counts on nav:{tenant}.
func TestNavCounts_FrameOnAlert(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	ctx := context.Background()
	routeID := CreateTestRoute(t, helper)
	before := navCounts(t, helper)

	relay, inspector := startTestRelay(t, helper, TestTenantID)
	defer relay.Shutdown()
	defer inspector.Close()
	registry := coreJobs.NewRegistry()
	lister := coreJobs.NewTenantDirectory(func(context.Context) ([]coreJobs.Tenant, error) { return []coreJobs.Tenant{testTenant()}, nil })
	registry.Handle(trackingJobs.TaskAlertChanged, trackingJobs.AlertChangedHandler(helper.DB(), lister, helper.Valkey(), nil, nil, nil))
	worker, err := coreJobs.StartWorker(helper.Valkey(), registry, 2)
	if err != nil {
		t.Fatalf("worker: %v", err)
	}
	defer worker.Shutdown()

	sub := helper.Valkey().Client().Subscribe(ctx, trackingJobs.NavChannel(TestTenantID))
	defer sub.Close()
	if _, err := sub.Receive(ctx); err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	waitAlerts := func(want int32) {
		t.Helper()
		deadline := time.After(3 * time.Second)
		for {
			select {
			case msg := <-sub.Channel():
				var frame trackingJobs.Frame
				var counts models.NavCounts
				if json.Unmarshal([]byte(msg.Payload), &frame) == nil && frame.Type == trackingJobs.FrameNavCounts &&
					json.Unmarshal(frame.Data, &counts) == nil && counts.AlertsActive != nil && *counts.AlertsActive == want {
					return
				}
			case <-deadline:
				t.Fatalf("no nav_counts frame with %d active alerts", want)
			}
		}
	}

	alertID := raiseAlert(t, helper, routeID, "delay", "medium", "Heavy rain")
	waitAlerts(*before.AlertsActive + 1)
	w := helper.DoRequest("PATCH", "/tracking/alerts/"+alertID+"/resolve", nil, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	waitAlerts(*before.AlertsActive)
}
