//go:build integration
// +build integration

package tracking_test

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"

	"josex/web/config"
	"josex/web/modules/core/jobs"
	"josex/web/modules/core/testhelpers"
)

// startTestRelay runs the outbox relay against the test database as if it were one tenant's. The
// poll fallback is set far beyond the test's deadlines, so anything published within them was woken
// by the commit's NOTIFY.
func startTestRelay(t *testing.T, helper *testhelpers.ApiTestHelper, tenantID string) (*jobs.OutboxRelay, *asynq.Inspector) {
	t.Helper()
	valkey := helper.Valkey()
	if valkey == nil {
		t.Fatal("REDIS_URL is not set in .env.test — the outbox tests need Valkey (make docker-up)")
	}
	ctx := context.Background()
	if err := valkey.Client().FlushDB(ctx).Err(); err != nil {
		t.Fatalf("Valkey unreachable: %v", err)
	}

	tenant := jobs.Tenant{ID: tenantID, DatabaseURL: config.ModularAppConfig.Core.DatabaseURL}
	lister := func(context.Context) ([]jobs.Tenant, error) { return []jobs.Tenant{tenant}, nil }
	relay := jobs.NewOutboxRelay(helper.DB(), valkey, lister, time.Minute)
	relay.Start(ctx)

	// The relay drains before its first wait: once nothing is unpublished it is listening.
	waitFor(t, 10*time.Second, "the relay to drain the rows other tests left", func() bool {
		var n int
		err := helper.DB().QueryRow(ctx, `SELECT count(*) FROM tracking.outbox WHERE published_at IS NULL`).Scan(&n)
		return err == nil && n == 0
	})
	return relay, asynq.NewInspectorFromRedisClient(valkey.Client())
}

func waitFor(t *testing.T, timeout time.Duration, what string, done func() bool) time.Duration {
	t.Helper()
	start := time.Now()
	for !done() {
		if time.Since(start) > timeout {
			t.Fatalf("timed out after %s waiting for %s", timeout, what)
		}
		time.Sleep(20 * time.Millisecond)
	}
	return time.Since(start)
}

// TestOutboxRelay_CommittedRowOnly - the outbox's whole promise: a committed row is an
// outbox:<topic> task within a second, stamped published; a row whose transaction rolled back never
// reaches Valkey.
func TestOutboxRelay_CommittedRowOnly(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	ctx := context.Background()
	tenantID := uuid.NewString()
	relay, inspector := startTestRelay(t, helper, tenantID)
	defer relay.Shutdown()
	defer inspector.Close()
	rolledBack := uuid.NewString()

	tx, err := helper.DB().BeginTransaction(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO tracking.outbox (topic, payload) VALUES ('document.alerts', jsonb_build_object('marker', $1::text))`, rolledBack); err != nil {
		t.Fatalf("insert in the rolled-back transaction: %v", err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	var id int64
	if err := helper.DB().QueryRow(ctx, `INSERT INTO tracking.outbox (topic, payload) VALUES ('document.alerts', '{"marker":"committed"}') RETURNING id`).Scan(&id); err != nil {
		t.Fatalf("insert committed row: %v", err)
	}

	taskID := tenantID + ":" + strconv.FormatInt(id, 10)
	var info *asynq.TaskInfo
	elapsed := waitFor(t, time.Second, "the committed row's task", func() bool {
		info, err = inspector.GetTaskInfo(jobs.QueueDefault, taskID)
		return err == nil
	})
	if info.Type != "outbox:document.alerts" {
		t.Fatalf("task type %q, want outbox:document.alerts", info.Type)
	}
	waitFor(t, time.Second-elapsed, "published_at on the committed row", func() bool {
		var published bool
		err := helper.DB().QueryRow(ctx, `SELECT published_at IS NOT NULL FROM tracking.outbox WHERE id = $1`, id).Scan(&published)
		return err == nil && published
	})

	pending, err := inspector.ListPendingTasks(jobs.QueueDefault, asynq.PageSize(100))
	if err != nil {
		t.Fatalf("pending tasks: %v", err)
	}
	for _, task := range pending {
		if strings.Contains(string(task.Payload), rolledBack) {
			t.Fatalf("the rolled-back row was published as %s", task.ID)
		}
	}
}

// routeChangedRows is how many route.changed rows the outbox holds, which is what "this write left
// exactly one behind" is measured against.
func routeChangedRows(t *testing.T, helper *testhelpers.ApiTestHelper) int {
	t.Helper()
	var n int
	if err := helper.DB().QueryRow(context.Background(),
		`SELECT count(*) FROM tracking.outbox WHERE topic = 'route.changed'`).Scan(&n); err != nil {
		t.Fatalf("count route.changed rows: %v", err)
	}
	return n
}

// TestOutboxRouteChanged_OneRowPerAcceptedWrite - TRACK-007 step 4: a stop list that changed leaves
// exactly one route.changed row, which the relay publishes as outbox:route.changed; a write the SP
// refuses leaves none, because the raise takes the insert down with it.
func TestOutboxRouteChanged_OneRowPerAcceptedWrite(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	ctx := context.Background()

	// Publish before the relay starts, so its own row is drained with the rest and the PUT below is
	// the only write still to be published.
	routeID := CreateTestRoute(t, helper)
	versionID := PublishRouteVersion(t, helper, routeID, "2027-08-02", []map[string]interface{}{
		{"stop_place_id": CentralStationID, "sequence": 1},
		{"stop_place_id": SchoolAStopID, "sequence": 2},
	})

	tenantID := uuid.NewString()
	relay, inspector := startTestRelay(t, helper, tenantID)
	defer relay.Shutdown()
	defer inspector.Close()
	before := routeChangedRows(t, helper)

	w := helper.DoRequest("PUT", "/tracking/routes/"+routeID+"/versions/"+versionID+"/stops",
		map[string]interface{}{"stops": []map[string]interface{}{
			{"stop_place_id": SchoolAStopID, "sequence": 1},
			{"stop_place_id": CentralStationID, "sequence": 2},
		}}, map[string]string{})
	if w.Code != http.StatusOK {
		t.Fatalf("reorder stops: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	if got := routeChangedRows(t, helper) - before; got != 1 {
		t.Fatalf("expected the reorder to leave 1 route.changed row, got %d", got)
	}
	var id int64
	var changedRouteID, dateFrom string
	var dateTo *string
	if err := helper.DB().QueryRow(ctx, `
		SELECT o.id, o.payload->>'route_id', o.payload->>'date_from', o.payload->>'date_to'
		  FROM tracking.outbox o
		 WHERE o.topic = 'route.changed'
		 ORDER BY o.id DESC
		 LIMIT 1`).Scan(&id, &changedRouteID, &dateFrom, &dateTo); err != nil {
		t.Fatalf("read the route.changed row: %v", err)
	}
	assert.Equal(t, routeID, changedRouteID)
	assert.Equal(t, "2027-08-02", dateFrom, "the version's first day")
	assert.Nil(t, dateTo, "a published version has no end until the next one closes it")

	taskID := tenantID + ":" + strconv.FormatInt(id, 10)
	var info *asynq.TaskInfo
	var err error
	waitFor(t, time.Second, "the reorder's task", func() bool {
		info, err = inspector.GetTaskInfo(jobs.QueueDefault, taskID)
		return err == nil
	})
	assert.Equal(t, "outbox:route.changed", info.Type)

	// A refused write leaves nothing: the seeded version's first day has passed, so the SP raises.
	after := routeChangedRows(t, helper)
	w = helper.DoRequest("PUT", "/tracking/routes/"+MorningRouteID+"/versions/"+MorningVersionID+"/stops",
		map[string]interface{}{"stops": []map[string]interface{}{
			{"stop_place_id": CentralStationID, "sequence": 1},
		}}, map[string]string{})

	assert.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	assert.Equal(t, after, routeChangedRows(t, helper), "a refused write must leave no outbox row")
}
