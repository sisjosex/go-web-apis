package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"josex/web/config"
	billingJobs "josex/web/modules/billing/jobs"
	billingRepos "josex/web/modules/billing/repositories"
	billingServices "josex/web/modules/billing/services"
	"josex/web/modules/core/jobs"
	"josex/web/modules/core/services"
	geoInterfaces "josex/web/modules/geo/interfaces"
	geoServices "josex/web/modules/geo/services"
	geoRouting "josex/web/modules/geo/services/routing"
	tenancyRepos "josex/web/modules/tenancy/repositories"
	trackingJobs "josex/web/modules/tracking/jobs"
	trackingPush "josex/web/modules/tracking/push"
	trackingRepos "josex/web/modules/tracking/repositories"
	trackingServices "josex/web/modules/tracking/services"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/hibiken/asynq/x/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

// background is the non-HTTP part of a process: whichever of the worker, the outbox relay and the
// scheduler its role runs. Every field may be nil.
type background struct {
	worker    *jobs.Worker
	relay     *jobs.OutboxRelay
	scheduler *jobs.Scheduler
	// positions drains the GPS streams into the tenants' databases (TRACK-010); a worker's job.
	positions *trackingJobs.PositionsConsumer
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
	// One directory per process: every walker and handler resolves tenants from this copy (INFRA-009 D5).
	directory := jobs.NewTenantDirectory(tenantLister(db))
	relay := jobs.NewOutboxRelay(db, valkey, directory, coreConf.OutboxPollInterval)
	// One router for every tracking job and the GPS consumer, so route, trace and ETA calls share its
	// breaker.
	var router geoInterfaces.Router
	if coreConf.IsModuleEnabled("tracking") {
		router = geoRouting.NewRouter(config.ModularAppConfig.Geo)
	}
	registerJobs(ctx, registry, db, valkey, relay, directory, router)

	if runsWorker {
		worker, err := jobs.StartWorker(valkey, registry, coreConf.JobsConcurrency)
		if err != nil {
			return nil, fmt.Errorf("worker: %w", err)
		}
		bg.worker = worker
		// Queue size, latency and failures on /metrics, read from Valkey on each scrape (INFRA-018).
		prometheus.MustRegister(metrics.NewQueueMetricsCollector(asynq.NewInspectorFromRedisClient(valkey.Client())))
		relay.Start(ctx)
		bg.relay = relay
		if coreConf.IsModuleEnabled("tracking") {
			trackingConf := config.ModularAppConfig.Tracking
			// A stop is announced once its ETA is within the notice minutes (TRACK-070 D3).
			eta := trackingServices.NewEtaService(router, geoServices.NewGeoService(nil, router, trackingRepos.NewHistoricalDurations(db), config.ModularAppConfig.Geo), valkey)
			live := trackingServices.NewLiveService(trackingRepos.NewTrackingRepository(db), eta)
			bg.positions = trackingJobs.NewPositionsConsumer(db, valkey, directory, trackingConf.IngestRadii()).
				WithApproachEta(trackingConf.ApproachMinutes, trackingJobs.LiveEta(live))
			bg.positions.Start(ctx)
		}
	}
	if runsScheduler {
		bg.scheduler = jobs.NewScheduler(valkey, registry, jobs.SchedulerLeaseTTL, jobs.SchedulerRenewEvery)
		bg.scheduler.Start(ctx)
		log.Println("✅ Scheduler started, waiting for the lease")
	}
	return bg, nil
}

// registerJobs is where every module's handlers and periodic entries are declared. Worker and
// scheduler build the same registry: the scheduler reads the entries, the worker the handlers. ctx is
// the process's: what a handler keeps for its lifetime, like the push adapter's token source, ends
// with it.
func registerJobs(ctx context.Context, registry *jobs.Registry, db services.DatabaseService, valkey services.ValkeyService, relay *jobs.OutboxRelay, directory *jobs.TenantDirectory, router geoInterfaces.Router) {
	daily := 24 * time.Hour
	registry.Handle(jobs.TaskOutboxPurge, relay.Purge)
	registry.Schedule(jobs.Entry{
		Cron: "30 3 * * *", Period: daily,
		Task: asynq.NewTask(jobs.TaskOutboxPurge, nil), Opts: []asynq.Option{asynq.Queue(jobs.QueueLow)},
	})

	// Paid business plans past their grace fall back to the free limits (BILLING-001 D3).
	registry.Handle(billingJobs.TaskExpire, billingJobs.ExpireHandler(
		billingServices.NewBillingService(billingRepos.NewBillingRepository(db))))
	registry.Schedule(jobs.Entry{
		Cron: billingJobs.ExpireCron, Period: daily,
		Task: asynq.NewTask(billingJobs.TaskExpire, nil), Opts: []asynq.Option{asynq.Queue(jobs.QueueLow)},
	})

	if config.ModularAppConfig.Core.IsModuleEnabled("tracking") {
		registry.Handle(trackingJobs.TaskDocumentAlerts, trackingJobs.PassHandler(db, directory))
		registry.Handle(trackingJobs.TaskDocumentDigest, trackingJobs.DigestHandler(services.NewEmailService(), tenantDirectory(db, directory)))
		geoConf := config.ModularAppConfig.Geo
		// A route's plan changed: rebuild its trips that have not started (TRACK-008 D4) and its planned
		// line by streets (TRACK-028). The daily pass is the same SP over every route, so a missed row is
		// caught the next morning.
		// A driver's day changed (TRACK-030 D3): a frame on its channel, and a silent push when FCM is set.
		jobsClient := jobs.NewClient(valkey)
		signal := trackingJobs.NewDriverSignal(db, jobsClient)
		registry.Handle(trackingJobs.TaskRouteChanged, trackingJobs.RouteChangedHandler(db, directory,
			trackingJobs.PathPlanner{Router: router, FallbackKmh: float64(geoConf.FallbackSpeedKmh)}, signal))
		registry.Handle(trackingJobs.TaskTripsMaterialise, trackingJobs.TripsMaterialiseHandler(db, directory))
		// Trip and alert events become the guardians' notices (TRACK-012): the feed always, the push only
		// with FCM credentials (D2).
		var pushClient *asynq.Client
		pusher, err := trackingPush.New(ctx, config.ModularAppConfig.Tracking)
		if err != nil {
			log.Printf("⚠️  push disabled: %v", err)
		} else if pusher != nil {
			registry.Handle(trackingJobs.TaskPushSend, trackingJobs.PushSendHandler(db, pusher))
			pushClient = jobsClient
		}
		registry.Handle(trackingJobs.TaskDayChanged, trackingJobs.DayChangedHandler(db, valkey, pusher))
		// Trips of today still without riders cancel themselves at their client's lead, and the driver is
		// told when FCM is set (TRACK-069 D1, D2).
		registry.Handle(trackingJobs.TaskTripsCancelEmpty, trackingJobs.TripsCancelEmptyHandler(db, directory, pusher))
		registry.Schedule(jobs.Entry{
			Cron: trackingJobs.TripsCancelEmptyCron, Period: trackingJobs.TripsCancelEmptyPeriod,
			Task: asynq.NewTask(trackingJobs.TaskTripsCancelEmpty, nil),
		})
		// The guardians' live progress notification (MOBILE-024): only with FCM; its ETA from the trip's
		// shared cache, through the same router.
		var progress *trackingJobs.ProgressSignal
		if pushClient != nil {
			progress = trackingJobs.NewProgressSignal(jobsClient)
			eta := trackingServices.NewEtaService(router, geoServices.NewGeoService(nil, router, trackingRepos.NewHistoricalDurations(db), geoConf), valkey)
			registry.Handle(trackingJobs.TaskTripProgress, trackingJobs.TripProgressHandler(db, valkey, pusher,
				trackingServices.NewLiveService(trackingRepos.NewTrackingRepository(db), eta)))
		}
		notify := trackingJobs.NewNotifier(db, pushClient, config.ModularAppConfig.Tracking)
		// A trip moved (TRACK-020 D1): published to everyone watching it (TRACK-025), and a completed
		// one gets its driven path, map-matched through the geo router — the raw line when Valhalla is
		// down (TRACK-010).
		registry.Handle(trackingJobs.TaskTripChanged, trackingJobs.TripChangedHandler(db, directory,
			router, valkey, notify, signal, progress))
		// An alert was raised or resolved (TRACK-004 D1): an `alert` frame to the fleet and the trip.
		registry.Handle(trackingJobs.TaskAlertChanged, trackingJobs.AlertChangedHandler(directory, valkey, notify))
		// A route proposal was asked (TRACK-014): solved by VROOM, on its own breaker.
		registry.Handle(trackingJobs.TaskOptimize, trackingJobs.OptimizeHandler(db, directory, geoRouting.NewOptimizer(geoConf)))
		registry.Handle(trackingJobs.TaskPositionsPartitions, trackingJobs.PositionsPartitionsHandler(db, directory,
			config.ModularAppConfig.Tracking.LocationRetentionDays))
		registry.Schedule(jobs.Entry{
			Cron: trackingJobs.DocumentAlertsCron, Period: daily,
			Task: asynq.NewTask(trackingJobs.TaskDocumentAlerts, nil),
		})
		registry.Schedule(jobs.Entry{
			Cron: trackingJobs.TripsMaterialiseCron, Period: daily,
			Task: asynq.NewTask(trackingJobs.TaskTripsMaterialise, nil),
		})
		registry.Schedule(jobs.Entry{
			Cron: trackingJobs.PositionsPartitionsCron, Period: daily,
			Task: asynq.NewTask(trackingJobs.TaskPositionsPartitions, nil),
		})
		// The reports' aggregates the trip close missed, the per-stop lateness and yesterday's usage
		// in the platform database (TRACK-013 D1, D3).
		registry.Handle(trackingJobs.TaskReportRollups, trackingJobs.ReportRollupsHandler(db,
			tenancyRepos.NewUsageRepository(db), directory))
		registry.Schedule(jobs.Entry{
			Cron: trackingJobs.ReportRollupsCron, Period: daily,
			Task: asynq.NewTask(trackingJobs.TaskReportRollups, nil), Opts: []asynq.Option{asynq.Queue(jobs.QueueLow)},
		})
	}
}

// tenantLister answers every active tenant from the platform database, shared ones (in the platform
// database, DatabaseURL "") included — what the directory holds and `cli jobs` walks too (INFRA-009).
func tenantLister(db services.DatabaseService) jobs.TenantLister {
	repo := tenancyRepos.NewTenantRepository(db)
	return func(ctx context.Context) ([]jobs.Tenant, error) {
		// InitDatabase connects in the background; querying before it has would dereference no pool.
		if db.GetPrimaryPool() == nil {
			return nil, errors.New("platform database not connected yet")
		}
		tenants, err := repo.ListTenantDirectory(ctx)
		if err != nil {
			return nil, err
		}
		out := make([]jobs.Tenant, 0, len(tenants))
		for _, t := range tenants {
			tenant := jobs.Tenant{ID: t.ID.String(), Slug: t.Slug, Name: t.Name}
			if t.DatabaseURL != nil {
				tenant.DatabaseURL = *t.DatabaseURL
			}
			out = append(out, tenant)
		}
		return out, nil
	}
}

// tenantDirectory answers a tenant's name, from the directory in memory, and its owners' and admins'
// addresses — one query per digest.
func tenantDirectory(db services.DatabaseService, directory *jobs.TenantDirectory) trackingJobs.TenantDirectory {
	repo := tenancyRepos.NewTenantRepository(db)
	return func(ctx context.Context, tenantID uuid.UUID) (*trackingJobs.TenantContact, error) {
		contact := &trackingJobs.TenantContact{Name: tenantID.String()}
		tenant, ok, err := directory.Find(ctx, tenantID.String())
		if err != nil {
			return nil, err
		}
		if ok && tenant.Name != "" {
			contact.Name = tenant.Name
		}
		admins, err := repo.ListTenantAdminEmails(ctx, tenantID)
		if err != nil {
			return nil, err
		}
		for _, a := range admins {
			contact.Recipients = append(contact.Recipients, trackingJobs.Recipient{Email: a.Email, Locale: a.Locale})
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
	if bg.positions != nil {
		bg.positions.Shutdown()
	}
	if bg.worker != nil {
		bg.worker.Shutdown()
	}
}
