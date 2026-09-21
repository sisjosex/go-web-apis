package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"josex/web/config"
	"josex/web/modules/core/jobs"
	"josex/web/modules/core/services"
	tenancyRepos "josex/web/modules/tenancy/repositories"
	trackingJobs "josex/web/modules/tracking/jobs"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

// background is the non-HTTP part of a process: whichever of the worker, the outbox relay and the
// scheduler its role runs. Every field may be nil.
type background struct {
	worker    *jobs.Worker
	relay     *jobs.OutboxRelay
	scheduler *jobs.Scheduler
}

// startBackground starts what the role asks for. In `all` a missing REDIS_URL only disables the jobs,
// so a single container without Valkey behaves as it did before INFRA-001; a worker or scheduler
// without it has nothing to do and refuses to start.
func startBackground(ctx context.Context, mode, role string, db services.DatabaseService, valkey services.ValkeyService) (*background, error) {
	platformAll := role == roleAll && mode == "platform"
	runsWorker := role == roleWorker || platformAll
	runsScheduler := role == roleScheduler || platformAll

	bg := &background{}
	if !runsWorker && !runsScheduler {
		return bg, nil
	}
	if valkey == nil {
		if role == roleAll {
			log.Println("ℹ️  REDIS_URL not set: worker and scheduler disabled")
			return bg, nil
		}
		return nil, fmt.Errorf("role %s needs REDIS_URL", role)
	}

	coreConf := config.ModularAppConfig.Core
	if !coreConf.IsModuleEnabled("tenancy") {
		if role == roleAll {
			log.Println("ℹ️  tenancy module disabled: worker and scheduler disabled")
			return bg, nil
		}
		return nil, errors.New("the worker and the scheduler walk every tenant: enable the tenancy module")
	}

	registry := jobs.NewRegistry()
	relay := jobs.NewOutboxRelay(db, valkey, tenantLister(db), coreConf.OutboxPollInterval)
	registerJobs(registry, db, relay)

	if runsWorker {
		worker, err := jobs.StartWorker(valkey, registry, coreConf.JobsConcurrency)
		if err != nil {
			return nil, fmt.Errorf("worker: %w", err)
		}
		bg.worker = worker
		relay.Start(ctx)
		bg.relay = relay
	}
	if runsScheduler {
		bg.scheduler = jobs.NewScheduler(valkey, registry, jobs.SchedulerLeaseTTL, jobs.SchedulerRenewEvery)
		bg.scheduler.Start(ctx)
		log.Println("✅ Scheduler started, waiting for the lease")
	}
	return bg, nil
}

// registerJobs is where every module's handlers and periodic entries are declared. Worker and
// scheduler build the same registry: the scheduler reads the entries, the worker the handlers.
func registerJobs(registry *jobs.Registry, db services.DatabaseService, relay *jobs.OutboxRelay) {
	daily := 24 * time.Hour
	registry.Handle(jobs.TaskOutboxPurge, relay.Purge)
	registry.Schedule(jobs.Entry{
		Cron: "30 3 * * *", Period: daily,
		Task: asynq.NewTask(jobs.TaskOutboxPurge, nil), Opts: []asynq.Option{asynq.Queue(jobs.QueueLow)},
	})

	if config.ModularAppConfig.Core.IsModuleEnabled("tracking") {
		registry.Handle(trackingJobs.TaskDocumentAlerts, trackingJobs.PassHandler(db, tenantLister(db)))
		registry.Handle(trackingJobs.TaskDocumentDigest, trackingJobs.DigestHandler(services.NewEmailService(), tenantDirectory(db)))
		registry.Schedule(jobs.Entry{
			Cron: trackingJobs.DocumentAlertsCron, Period: daily,
			Task: asynq.NewTask(trackingJobs.TaskDocumentAlerts, nil),
		})
	}
}

// tenantLister answers every tenant with its own database, from the platform database — the walk
// `cli jobs` does too (D2).
func tenantLister(db services.DatabaseService) jobs.TenantLister {
	repo := tenancyRepos.NewTenantRepository(db)
	return func(ctx context.Context) ([]jobs.Tenant, error) {
		// InitDatabase connects in the background; querying before it has would dereference no pool.
		if db.GetPrimaryPool() == nil {
			return nil, errors.New("platform database not connected yet")
		}
		tenants, err := repo.ListTenantsWithCustomDB(ctx)
		if err != nil {
			return nil, err
		}
		out := make([]jobs.Tenant, 0, len(tenants))
		for _, t := range tenants {
			if t.DatabaseURL != nil && *t.DatabaseURL != "" {
				out = append(out, jobs.Tenant{ID: t.ID.String(), DatabaseURL: *t.DatabaseURL})
			}
		}
		return out, nil
	}
}

// tenantDirectory answers a tenant's name and its owners' and admins' addresses. The name comes from
// the same listing the relay walks — one query per digest, the lookup `cli jobs` used to do.
func tenantDirectory(db services.DatabaseService) trackingJobs.TenantDirectory {
	repo := tenancyRepos.NewTenantRepository(db)
	return func(ctx context.Context, tenantID uuid.UUID) (*trackingJobs.TenantContact, error) {
		tenants, err := repo.ListTenantsWithCustomDB(ctx)
		if err != nil {
			return nil, err
		}
		contact := &trackingJobs.TenantContact{Name: tenantID.String()}
		for _, t := range tenants {
			if t.ID == tenantID {
				contact.Name = t.Name
			}
		}
		admins, err := repo.ListTenantAdminEmails(ctx, tenantID)
		if err != nil {
			return nil, err
		}
		for _, a := range admins {
			contact.Emails = append(contact.Emails, a.Email)
		}
		return contact, nil
	}
}

// Shutdown stops the scheduler first — nothing new gets enqueued — then the relay, then drains the
// worker.
func (bg *background) Shutdown() {
	if bg.scheduler != nil {
		bg.scheduler.Shutdown()
	}
	if bg.relay != nil {
		bg.relay.Shutdown()
	}
	if bg.worker != nil {
		bg.worker.Shutdown()
	}
}
