//go:build integration
// +build integration

package tracking_test

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

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
