//go:build integration
// +build integration

package core_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"josex/web/modules/core/jobs"
	"josex/web/modules/core/testhelpers"

	"github.com/hibiken/asynq"
)

// jobsValkey hands a test the shared Valkey with its test database emptied, so no task, unique lock
// or lease from an earlier test is still there.
func jobsValkey(t *testing.T) *testhelpers.ApiTestHelper {
	t.Helper()
	helper := testhelpers.SetupApiTest(t)
	if helper.Valkey() == nil {
		t.Fatal("REDIS_URL is not set in .env.test — the jobs tests need Valkey (make docker-up)")
	}
	if err := helper.Valkey().Client().FlushDB(context.Background()).Err(); err != nil {
		t.Fatalf("Valkey unreachable: %v", err)
	}
	return helper
}

// TestUniqueTaskRunsOnce - the same unique key enqueued twice is refused the second time and the
// handler runs once.
func TestUniqueTaskRunsOnce(t *testing.T) {
	helper := jobsValkey(t)
	defer helper.Close()
	valkey := helper.Valkey()

	var runs atomic.Int32
	registry := jobs.NewRegistry()
	registry.Handle("test:unique", func(context.Context, *asynq.Task) error {
		runs.Add(1)
		return nil
	})

	client := jobs.NewClient(valkey)
	defer client.Close()
	task := asynq.NewTask("test:unique", []byte(`{"id":1}`))
	if _, err := client.Enqueue(task, asynq.Unique(time.Minute)); err != nil {
		t.Fatalf("first enqueue: %v", err)
	}
	if _, err := client.Enqueue(task, asynq.Unique(time.Minute)); !errors.Is(err, asynq.ErrDuplicateTask) {
		t.Fatalf("second enqueue: want ErrDuplicateTask, got %v", err)
	}

	worker, err := jobs.StartWorker(valkey, registry, 2)
	if err != nil {
		t.Fatalf("worker: %v", err)
	}
	defer worker.Shutdown()

	deadline := time.Now().Add(10 * time.Second)
	for runs.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)
	}
	// Leave the worker time to pick up a second copy, were there one.
	time.Sleep(2 * time.Second)
	if got := runs.Load(); got != 1 {
		t.Fatalf("handler ran %d times, want 1", got)
	}
}

// TestSchedulerSingleLeader - two schedulers: exactly one holds the lease and fires, each tick lands
// once, and stopping the holder hands the lease to the other.
func TestSchedulerSingleLeader(t *testing.T) {
	helper := jobsValkey(t)
	defer helper.Close()
	valkey := helper.Valkey()

	registry := jobs.NewRegistry()
	registry.Schedule(jobs.Entry{Cron: "@every 1s", Period: time.Second, Task: asynq.NewTask("test:tick", nil)})

	const ttl, renew = 3 * time.Second, time.Second
	a := jobs.NewScheduler(valkey, registry, ttl, renew)
	b := jobs.NewScheduler(valkey, registry, ttl, renew)
	a.Start(context.Background())
	b.Start(context.Background())
	defer a.Shutdown()
	defer b.Shutdown()

	inspector := asynq.NewInspectorFromRedisClient(valkey.Client())
	defer inspector.Close()

	// Six seconds of ticks: at every sample exactly one of the two fires, and only its entries are
	// registered in Valkey.
	time.Sleep(500 * time.Millisecond)
	start := time.Now()
	for time.Since(start) < 6*time.Second {
		if a.IsLeader() == b.IsLeader() {
			t.Fatalf("leaders: a=%v b=%v, want exactly one", a.IsLeader(), b.IsLeader())
		}
		time.Sleep(500 * time.Millisecond)
	}
	entries, err := inspector.SchedulerEntries()
	if err != nil {
		t.Fatalf("scheduler entries: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("%d scheduler entries registered, want 1 (only the lease holder's)", len(entries))
	}

	// One task per period at most: with both firing there would be about twice as many.
	pending, err := inspector.ListPendingTasks(jobs.QueueDefault, asynq.PageSize(100))
	if err != nil {
		t.Fatalf("pending tasks: %v", err)
	}
	elapsed := int(time.Since(start)/time.Second) + 1
	if len(pending) == 0 || len(pending) > elapsed {
		t.Fatalf("%d ticks enqueued in ~%ds, want between 1 and %d", len(pending), elapsed, elapsed)
	}

	// Stop the holder: the other takes the lease at its next tick, since Shutdown releases it.
	leader, follower := a, b
	if b.IsLeader() {
		leader, follower = b, a
	}
	leader.Shutdown()
	deadline := time.Now().Add(ttl + renew)
	for !follower.IsLeader() && time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)
	}
	if !follower.IsLeader() {
		t.Fatal("the remaining scheduler never took the lease")
	}
}

// TestLease_SingleHolderAndRenewal - while one holder keeps renewing, a second is refused past the
// first TTL; a renewal by the loser changes nothing; a release hands the lease over at once (INFRA-014).
func TestLease_SingleHolderAndRenewal(t *testing.T) {
	helper := jobsValkey(t)
	defer helper.Close()
	ctx := context.Background()
	client := helper.Valkey().Client()
	const ttl = 600 * time.Millisecond

	first := jobs.NewLease(client, "test:lease", ttl)
	second := jobs.NewLease(client, "test:lease", ttl)

	if ok, err := first.Acquire(ctx); err != nil || !ok {
		t.Fatalf("first acquire: ok=%v err=%v", ok, err)
	}
	if ok, err := second.Acquire(ctx); err != nil || ok {
		t.Fatalf("second acquire while held: ok=%v err=%v, want refused", ok, err)
	}

	// Renew at half the TTL twice: the lease outlives its first TTL only because it was kept.
	for i := 0; i < 2; i++ {
		time.Sleep(ttl / 2)
		if ok, err := first.Renew(ctx); err != nil || !ok {
			t.Fatalf("renew %d: ok=%v err=%v", i, ok, err)
		}
	}
	if ok, err := second.Acquire(ctx); err != nil || ok {
		t.Fatalf("second acquire after a renewed TTL: ok=%v err=%v, want refused", ok, err)
	}
	if ok, err := second.Renew(ctx); err != nil || ok {
		t.Fatalf("a renewal by the non-holder: ok=%v err=%v, want refused", ok, err)
	}

	if err := first.Release(ctx); err != nil {
		t.Fatalf("release: %v", err)
	}
	if ok, err := second.Acquire(ctx); err != nil || !ok {
		t.Fatalf("second acquire after release: ok=%v err=%v", ok, err)
	}
	if ok, err := first.Renew(ctx); err != nil || ok {
		t.Fatalf("the old holder renews a lease it lost: ok=%v err=%v, want refused", ok, err)
	}
}
