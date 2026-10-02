//go:build integration
// +build integration

package tracking_test

import (
	"context"
	"encoding/json"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"

	coreJobs "josex/web/modules/core/jobs"
	trackingJobs "josex/web/modules/tracking/jobs"
	"josex/web/modules/tracking/models"
	trackingServices "josex/web/modules/tracking/services"
)

// INFRA-009: tenants sharing one database — the outbox says whose each row is, one relay and one GPS
// loop serve them all, and nothing of one reaches the other.

// TestOutboxTenant_FilledFromTheRoute - a row naming a route is stored under that route's tenant; one
// naming neither a tenant nor a known route is refused, taking its transaction down.
func TestOutboxTenant_FilledFromTheRoute(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	_, tenantID, err := InsertOutboxRow(helper, "route.changed", map[string]interface{}{"route_id": MorningRouteID})
	assert.NoError(t, err)
	assert.Equal(t, TestTenantID, tenantID)

	_, _, err = InsertOutboxRow(helper, "route.changed", map[string]interface{}{"route_id": nil})
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "outbox.tenant")
	}
}

// TestOutboxRelay_TwoTenantsOneDatabase - one relay drains the database both tenants live in, and each
// task carries the tenant of its own row.
func TestOutboxRelay_TwoTenantsOneDatabase(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	otherTenantID, otherRouteID, _ := SeedOtherTenantFleet(t, helper)
	relay, inspector := startTestRelay(t, helper, TestTenantID)
	defer relay.Shutdown()
	defer inspector.Close()

	ids := map[string]int64{}
	for tenant, route := range map[string]string{TestTenantID: MorningRouteID, otherTenantID: otherRouteID} {
		id, stored, err := InsertOutboxRow(helper, "route.changed", map[string]interface{}{"route_id": route})
		if err != nil {
			t.Fatalf("insert: %v", err)
		}
		assert.Equal(t, tenant, stored)
		ids[tenant] = id
	}

	for tenant, id := range ids {
		taskID := tenant + ":" + strconv.FormatInt(id, 10)
		waitFor(t, 2*time.Second, "task "+taskID, func() bool {
			_, err := inspector.GetTaskInfo(coreJobs.QueueDefault, taskID)
			return err == nil
		})
	}
}

// TestOutboxRelay_PendingRowsSurviveARestart - rows written while no relay runs are published once,
// by the next one to start.
func TestOutboxRelay_PendingRowsSurviveARestart(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	ctx := context.Background()
	relay, inspector := startTestRelay(t, helper, TestTenantID)
	relay.Shutdown()
	defer inspector.Close()

	id, _, err := InsertOutboxRow(helper, "route.changed", map[string]interface{}{"route_id": MorningRouteID})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	directory := coreJobs.NewTenantDirectory(func(context.Context) ([]coreJobs.Tenant, error) { return []coreJobs.Tenant{testTenant()}, nil })
	restarted := coreJobs.NewOutboxRelay(helper.DB(), helper.Valkey(), directory, time.Minute)
	restarted.Start(ctx)
	defer restarted.Shutdown()

	taskID := TestTenantID + ":" + strconv.FormatInt(id, 10)
	waitFor(t, 2*time.Second, "the pending row's task", func() bool {
		_, err := inspector.GetTaskInfo(coreJobs.QueueDefault, taskID)
		return err == nil
	})
	pending, err := inspector.ListPendingTasks(coreJobs.QueueDefault, asynq.PageSize(500))
	assert.NoError(t, err)
	published := 0
	for _, task := range pending {
		if task.ID == taskID {
			published++
		}
	}
	assert.Equal(t, 1, published)
}

// sharedTenants is a directory where both tenants live in the platform database (DatabaseURL "").
func sharedTenants(ids ...string) *coreJobs.TenantDirectory {
	tenants := make([]coreJobs.Tenant, 0, len(ids))
	for _, id := range ids {
		tenants = append(tenants, coreJobs.Tenant{ID: id, Slug: id[:8]})
	}
	return coreJobs.NewTenantDirectory(func(context.Context) ([]coreJobs.Tenant, error) { return tenants, nil })
}

// TestPositionsConsumer_SharedStreamTwoTenants - the API queues both shared tenants' points on
// gps:shared, one loop stores each under its own tenant, and a shared tenant's pre-INFRA-009 stream
// is drained and deleted.
func TestPositionsConsumer_SharedStreamTwoTenants(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	ctx := context.Background()
	client := helper.Valkey().Client()
	if err := client.FlushDB(ctx).Err(); err != nil {
		t.Fatalf("Valkey unreachable: %v", err)
	}
	otherTenantID, _, otherVehicleID := SeedOtherTenantFleet(t, helper)
	vehicleID := CreateTestVehicleOf(t, helper, MainCompanyID)
	at := time.Now().UTC().Add(-time.Minute)

	// Shared tenants queue through the API's path: no database URL in the request context.
	ingest := trackingServices.NewPositionIngest(nil, helper.Valkey(), 1000, models.IngestRadii{ArrivalM: 50, ApproachM: 800})
	for tenant, vehicle := range map[string]string{TestTenantID: vehicleID, otherTenantID: otherVehicleID} {
		path, err := ingest.Ingest(ctx, uuid.MustParse(tenant), []models.StoredPoint{
			{VehicleID: uuid.MustParse(vehicle), RecordedAt: at, Lat: -17.39, Lng: -66.15},
		})
		assert.NoError(t, err)
		assert.Equal(t, models.IngestPathQueued, path)
	}
	assert.Equal(t, int64(2), client.XLen(ctx, trackingServices.GPSSharedStream).Val())

	// A point left on the seeded tenant's old stream, untagged, as the API queued it before.
	raw, _ := json.Marshal(models.StoredPoint{VehicleID: uuid.MustParse(vehicleID), RecordedAt: at.Add(time.Second), Lat: -17.391, Lng: -66.151})
	legacy := trackingServices.GPSStream(TestTenantID)
	if err := client.XAdd(ctx, &redis.XAddArgs{Stream: legacy, Values: []string{trackingServices.GPSStreamField, string(raw)}}).Err(); err != nil {
		t.Fatalf("xadd legacy: %v", err)
	}

	consumer := trackingJobs.NewPositionsConsumer(helper.DB(), helper.Valkey(), sharedTenants(TestTenantID, otherTenantID), models.IngestRadii{ArrivalM: 50, ApproachM: 800})
	consumer.Start(ctx)
	defer consumer.Shutdown()

	waitFor(t, 5*time.Second, "both tenants' points stored, the legacy one too", func() bool {
		return VehiclePositionCount(t, helper, vehicleID) == 2 && VehiclePositionCount(t, helper, otherVehicleID) == 1
	})
	waitFor(t, 5*time.Second, "the legacy stream deleted", func() bool {
		return client.Exists(ctx, legacy).Val() == 0
	})
}

// TestTripsMaterialise_SharedTenant - the daily pass walks a tenant of the shared database (no URL of
// its own), which the walk skipped before INFRA-009: its route's 15 days of trips are built.
func TestTripsMaterialise_SharedTenant(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	routeID, _ := createTripRoute(t, helper, "07:00")

	handler := trackingJobs.TripsMaterialiseHandler(helper.DB(), sharedTenants(TestTenantID))
	if err := handler(context.Background(), asynq.NewTask(trackingJobs.TaskTripsMaterialise, nil)); err != nil {
		t.Fatalf("pass: %v", err)
	}
	assert.Equal(t, 15, routeTripShape(t, helper, routeID).Trips)
}
