//go:build integration
// +build integration

package tracking_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"

	"josex/web/config"
	coreJobs "josex/web/modules/core/jobs"
	trackingJobs "josex/web/modules/tracking/jobs"
)

// TestNotifySettings_SwitchedOff - TENANCY-007 step 2 (D2): a workspace that switched boarding notices
// off still gets the feed row and the trip's `done` event, but no push-send. The tenant comes from the
// directory with its own database URL, as a dedicated-database tenant does: the setting is read from
// the directory, never from the tenant's database.
func TestNotifySettings_SwitchedOff(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	routeID, riderID := createTripRoute(t, helper, "07:00")
	LinkRiderGuardians(t, helper, riderID, "portal@test.local")
	today := dayOffset(t, "UTC", 0)
	materialiseRoute(t, helper, routeID, today, today)
	tripID, _ := tripOn(t, helper, routeID, today, "07:00")

	relay, inspector := startTestRelay(t, helper, TestTenantID)
	defer relay.Shutdown()
	defer inspector.Close()
	tenant := testTenant()
	tenant.Settings = map[string]any{"tracking.notice.boarding": false}
	lister := coreJobs.NewTenantDirectory(func(context.Context) ([]coreJobs.Tenant, error) { return []coreJobs.Tenant{tenant}, nil })
	registry := coreJobs.NewRegistry()
	notify := trackingJobs.NewNotifier(helper.DB(), coreJobs.NewClient(helper.Valkey()), config.ModularAppConfig.Tracking)
	registry.Handle(trackingJobs.TaskTripChanged, trackingJobs.TripChangedHandler(helper.DB(), lister, nil, nil, notify, nil, nil))
	worker, err := coreJobs.StartWorker(helper.Valkey(), registry, 2)
	if err != nil {
		t.Fatalf("worker: %v", err)
	}
	defer worker.Shutdown()

	detail := tripRequest(t, helper, "POST", "/tracking/trips/"+tripID+"/start", nil, http.StatusOK)
	tripRequest(t, helper, "POST", "/tracking/trip-stop-tasks/"+taskOf(t, detail, riderID, "pickup")+"/done",
		map[string]interface{}{"client_op_id": uuid.NewString()}, http.StatusOK)

	var boarded, started map[string]string
	waitFor(t, 5*time.Second, "the boarded and started rows", func() bool {
		boarded = noticeRows(t, helper, riderID, "boarded")
		started = noticeRows(t, helper, riderID, "trip_started")
		return len(boarded) == 1 && len(started) == 1
	})
	_, err = inspector.GetTaskInfo(coreJobs.QueueCritical, boarded["portal@test.local"])
	assert.ErrorIs(t, err, asynq.ErrTaskNotFound, "a switched-off notice is never pushed")
	_, err = inspector.GetTaskInfo(coreJobs.QueueCritical, started["portal@test.local"])
	assert.NoError(t, err, "a notice no setting governs is still pushed")

	assert.Equal(t, 1, TripEventCount(t, helper, uuid.MustParse(tripID), "done"), "the trip's history keeps the event")
}
