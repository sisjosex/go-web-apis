package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"strconv"
	"sync"
	"time"

	coreServices "josex/web/modules/core/services"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	// outboxChannel is what tracking.fn_outbox_notify signals on commit.
	outboxChannel = "tracking_outbox"
	// outboxBatch caps one claim: the rows stay locked while they are enqueued, so a batch is a
	// short transaction, never an unbounded one.
	outboxBatch = 100
	// tenantRefreshEvery is how soon a new tenant gets its relay, or a removed one loses it.
	tenantRefreshEvery = 60 * time.Second
	// outboxRetention is how long a published row is kept for inspection before the purge.
	outboxRetention = 7 * 24 * time.Hour

	// TaskOutboxPurge deletes published rows past outboxRetention, in every tenant.
	TaskOutboxPurge = "outbox:purge"
	// outboxTaskPrefix + topic is the task type a row becomes (D3); modules handle their topics.
	outboxTaskPrefix = "outbox:"
)

// OutboxTask is the task type a topic's rows are published as.
func OutboxTask(topic string) string {
	return outboxTaskPrefix + topic
}

// Tenant is what the relay needs of a tenant: whose database it is, and where. Slug names it to a
// phone, which selects the tenant from a push before it opens the rider.
type Tenant struct {
	ID          string
	Slug        string
	DatabaseURL string
}

// TenantLister answers the tenants whose databases carry an outbox. cmd/server builds it from the
// platform database, so core never imports the tenancy module.
type TenantLister func(ctx context.Context) ([]Tenant, error)

// OutboxRelay publishes every tenant's tracking.outbox rows as asynq tasks. One goroutine per tenant
// sleeps on LISTEN and drains on each NOTIFY; the poll interval is the fallback for a notification
// lost while its connection was down.
type OutboxRelay struct {
	db      coreServices.DatabaseService
	client  *asynq.Client
	tenants TenantLister
	poll    time.Duration

	mu      sync.Mutex
	running map[Tenant]context.CancelFunc
	wg      sync.WaitGroup
	cancel  context.CancelFunc
}

func NewOutboxRelay(db coreServices.DatabaseService, valkey coreServices.ValkeyService, tenants TenantLister, poll time.Duration) *OutboxRelay {
	return &OutboxRelay{
		db:      db,
		client:  NewClient(valkey),
		tenants: tenants,
		poll:    poll,
		running: map[Tenant]context.CancelFunc{},
	}
}

// Start runs the tenant refresh loop until Shutdown. The loop outlives ctx's cancellation on purpose:
// the process stops the relay after the HTTP server has drained, not when the signal arrives.
func (r *OutboxRelay) Start(ctx context.Context) {
	ctx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	r.cancel = cancel
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		for {
			wait := tenantRefreshEvery
			if err := r.refresh(ctx); err != nil {
				// Usually the platform pool is still connecting at boot: retry soon, not in a minute.
				log.Printf("⚠️  outbox relay: %v", err)
				wait = r.poll
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(wait):
			}
		}
	}()
	log.Printf("✅ Outbox relay started (poll fallback %s)", r.poll)
}

// Shutdown stops every tenant's relay and waits for the batch in flight to commit or roll back.
func (r *OutboxRelay) Shutdown() {
	if r.cancel != nil {
		r.cancel()
	}
	r.wg.Wait()
	_ = r.client.Close()
}

// refresh starts a relay for each tenant it has not seen and stops the ones no longer listed. A
// tenant is keyed by id and URL, so a moved database gets a fresh relay.
func (r *OutboxRelay) refresh(ctx context.Context) error {
	tenants, err := r.tenants(ctx)
	if err != nil {
		return fmt.Errorf("listing tenants: %w", err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	listed := make(map[Tenant]bool, len(tenants))
	for _, t := range tenants {
		listed[t] = true
		if _, ok := r.running[t]; ok {
			continue
		}
		tenantCtx, cancel := context.WithCancel(ctx)
		r.running[t] = cancel
		r.wg.Add(1)
		go func(t Tenant) {
			defer r.wg.Done()
			r.runTenant(tenantCtx, t)
		}(t)
	}
	for t, cancel := range r.running {
		if !listed[t] {
			cancel()
			delete(r.running, t)
		}
	}
	return nil
}

// runTenant keeps one tenant drained. The LISTEN connection is its own, outside the tenant pool, so
// a relay never holds one of the connections the tenant's requests share.
func (r *OutboxRelay) runTenant(ctx context.Context, t Tenant) {
	for ctx.Err() == nil {
		if err := r.listen(ctx, t); err != nil && ctx.Err() == nil {
			log.Printf("⚠️  outbox relay %s: %v (retrying in %s)", t.ID, err, r.poll)
			select {
			case <-ctx.Done():
			case <-time.After(r.poll):
			}
		}
	}
}

func (r *OutboxRelay) listen(ctx context.Context, t Tenant) error {
	pool, err := r.db.GetPoolForTenant(ctx, t.DatabaseURL)
	if err != nil {
		return err
	}
	conn, err := pgx.Connect(ctx, t.DatabaseURL)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close(context.WithoutCancel(ctx)) }()
	if _, err := conn.Exec(ctx, "LISTEN "+outboxChannel); err != nil {
		return err
	}

	for {
		// Drain first: rows written while no one was listening are published before the first wait.
		if err := r.drain(ctx, pool, t); err != nil {
			return err
		}
		waitCtx, cancel := context.WithTimeout(ctx, r.poll)
		_, err := conn.WaitForNotification(waitCtx)
		cancel()
		if ctx.Err() != nil {
			return nil
		}
		if err != nil && !errors.Is(err, context.DeadlineExceeded) {
			return err
		}
	}
}

// drain publishes batches until one comes back short.
func (r *OutboxRelay) drain(ctx context.Context, pool *pgxpool.Pool, t Tenant) error {
	for {
		claimed, err := r.publishBatch(ctx, pool, t)
		if err != nil || claimed < outboxBatch {
			return err
		}
	}
}

type outboxRow struct {
	id      int64
	topic   string
	payload json.RawMessage
}

// publishBatch claims up to outboxBatch rows, enqueues each as outbox:<topic> with TaskID
// <tenant>:<id>, and stamps the ones that made it — all in one transaction. A crash after the enqueue
// and before the commit leaves the row unpublished; the next pass enqueues it again, and the TaskID
// makes that a no-op while the first task is still in Valkey.
func (r *OutboxRelay) publishBatch(ctx context.Context, pool *pgxpool.Pool, t Tenant) (int, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()

	rows, err := tx.Query(ctx, `SELECT id, topic, payload FROM tracking.sp_outbox_claim(p_limit := $1)`, outboxBatch)
	if err != nil {
		return 0, err
	}
	claimed, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (outboxRow, error) {
		var o outboxRow
		err := row.Scan(&o.id, &o.topic, &o.payload)
		return o, err
	})
	if err != nil {
		return 0, err
	}
	if len(claimed) == 0 {
		return 0, nil
	}

	published := make([]int64, 0, len(claimed))
	var enqueueErr error
	for _, o := range claimed {
		task := asynq.NewTask(OutboxTask(o.topic), o.payload)
		_, err := r.client.EnqueueContext(ctx, task, asynq.TaskID(t.ID+":"+strconv.FormatInt(o.id, 10)))
		if err != nil && !errors.Is(err, asynq.ErrTaskIDConflict) {
			// Valkey is down: stop here, keep what made it, leave the rest for the next pass.
			enqueueErr = fmt.Errorf("enqueue outbox row %d: %w", o.id, err)
			break
		}
		published = append(published, o.id)
	}

	if len(published) > 0 {
		if _, err := tx.Exec(ctx, `SELECT tracking.sp_outbox_mark_published(p_ids := $1)`, published); err != nil {
			return 0, err
		}
		if err := tx.Commit(ctx); err != nil {
			return 0, err
		}
	}
	return len(claimed), enqueueErr
}

// Purge is the outbox:purge handler: every tenant's rows published over 7 days ago. A tenant that
// fails is logged and the rest still run; the task fails, and asynq retries it, only if one did.
func (r *OutboxRelay) Purge(ctx context.Context, _ *asynq.Task) error {
	tenants, err := r.tenants(ctx)
	if err != nil {
		return err
	}
	failed := 0
	for _, t := range tenants {
		pool, err := r.db.GetPoolForTenant(ctx, t.DatabaseURL)
		if err == nil {
			var n int
			err = pool.QueryRow(ctx, `SELECT tracking.sp_outbox_purge(p_older_than := $1)`, outboxRetention).Scan(&n)
			if err == nil && n > 0 {
				slog.Debug("outbox purge", "tenant", t.ID, "purged", n)
			}
		}
		if err != nil {
			log.Printf("⚠️  outbox purge %s: %v", t.ID, err)
			failed++
		}
	}
	if failed > 0 {
		return fmt.Errorf("outbox purge failed for %d of %d tenants", failed, len(tenants))
	}
	return nil
}
