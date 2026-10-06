package routes

import (
	"context"
	"encoding/json"

	"josex/web/config"
	authControllers "josex/web/modules/auth/controllers"
	authMW "josex/web/modules/auth/middleware"
	authRepos "josex/web/modules/auth/repositories"
	authRoutes "josex/web/modules/auth/routes"
	authServices "josex/web/modules/auth/services"
	billingControllers "josex/web/modules/billing/controllers"
	billingInterfaces "josex/web/modules/billing/interfaces"
	billingRepos "josex/web/modules/billing/repositories"
	billingRoutes "josex/web/modules/billing/routes"
	billingServices "josex/web/modules/billing/services"
	coreControllers "josex/web/modules/core/controllers"
	coreJobs "josex/web/modules/core/jobs"
	coreMiddleware "josex/web/modules/core/middleware"
	coreModels "josex/web/modules/core/models"
	coreRoutes "josex/web/modules/core/routes"
	coreServices "josex/web/modules/core/services"
	"josex/web/modules/core/services/storage"
	geoControllers "josex/web/modules/geo/controllers"
	geoInterfaces "josex/web/modules/geo/interfaces"
	geoRepos "josex/web/modules/geo/repositories"
	geoRoutes "josex/web/modules/geo/routes"
	geoServices "josex/web/modules/geo/services"
	geoRouting "josex/web/modules/geo/services/routing"
	importControllers "josex/web/modules/import/controllers"
	importRoutes "josex/web/modules/import/routes"
	importServices "josex/web/modules/import/services"
	inventoryRoutes "josex/web/modules/inventory/routes"
	inventoryServices "josex/web/modules/inventory/services"
	purchasingRepos "josex/web/modules/purchasing/repositories"
	purchasingRoutes "josex/web/modules/purchasing/routes"
	purchasingServices "josex/web/modules/purchasing/services"
	salesControllers "josex/web/modules/sales/controllers"
	salesRepos "josex/web/modules/sales/repositories"
	salesRoutes "josex/web/modules/sales/routes"
	salesServices "josex/web/modules/sales/services"
	tenancyControllers "josex/web/modules/tenancy/controllers"
	tenancyInterfaces "josex/web/modules/tenancy/interfaces"
	tenancyMW "josex/web/modules/tenancy/middleware"
	tenancyModels "josex/web/modules/tenancy/models"
	tenancyRepos "josex/web/modules/tenancy/repositories"
	tenancyRoutes "josex/web/modules/tenancy/routes"
	tenancyServices "josex/web/modules/tenancy/services"
	trackingControllers "josex/web/modules/tracking/controllers"
	"josex/web/modules/tracking/realtime"
	trackingRepos "josex/web/modules/tracking/repositories"
	trackingRoutes "josex/web/modules/tracking/routes"
	trackingServices "josex/web/modules/tracking/services"
	userControllers "josex/web/modules/users/controllers"
	userInterfaces "josex/web/modules/users/interfaces"
	userRepos "josex/web/modules/users/repositories"
	userRoutes "josex/web/modules/users/routes"
	userServices "josex/web/modules/users/services"
	"log"
	"time"

	coreValidators "josex/web/modules/core/validators"
	_ "josex/web/modules/inventory"
	_ "josex/web/modules/purchasing"
	_ "josex/web/modules/sales"
	_ "josex/web/modules/tracking"
	_ "josex/web/modules/users"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/ua-parser/uap-go/uaparser"
)

// routeDeps is what the module registrations below share.
type routeDeps struct {
	engine         *gin.Engine
	apiV1          *gin.RouterGroup
	db             coreServices.DatabaseService
	jwt            authServices.JWTService
	authMiddleware gin.HandlerFunc
	email          coreServices.EmailService
	media          coreServices.MediaService
	users          userInterfaces.UserService
	userAudit      userInterfaces.UserAuditService
	// documents is the private bucket (INFRA-007 D2); purposes are what an upload ticket may be for.
	documents storage.ObjectStore
	purposes  *storage.Purposes
	// router is the one Valhalla adapter of the process, so geo and the tracking ETA share its breaker.
	router geoInterfaces.Router
	valkey coreServices.ValkeyService
	// hub is the WebSocket gateway (TRACK-025); nil when this process runs none.
	hub *realtime.Hub
}

// tenantChains are what business routes mount behind. Without tenancy both are
// no-ops, so business routes still register, and modules is nil.
type tenantChains struct {
	tenant gin.HandlerFunc
	// Same chain, but a portal guardian passes the mobile-only refusal (TRACK-017 D1). Only the
	// routes a guardian may read are registered behind it.
	portal gin.HandlerFunc
	// The GPS ingest's two ways in (TRACK-010): a driver's session — the chain with the driver's
	// mobile-only refusal lifted — and a device token, which replaces auth and slug altogether.
	driver gin.HandlerFunc
	device gin.HandlerFunc
	// The realtime socket's: guardians and drivers both pass, each to its own channels (TRACK-030).
	socket  gin.HandlerFunc
	modules tenancyInterfaces.ModuleService
}

// withModule is tenant followed by the module's check (TENANCY-003 D2): a workspace without the module
// enabled gets 403 tenant.module.not-enabled. The list comes from the in-process cache, so the check
// costs no query on a warm tenant. Without tenancy it is tenant unchanged.
func (chains tenantChains) withModule(tenant gin.HandlerFunc, code string) gin.HandlerFunc {
	if chains.modules == nil {
		return tenant
	}
	load, require := tenancyMW.LoadTenantModules(chains.modules), tenancyMW.RequireModule(code)
	return func(c *gin.Context) {
		tenant(c)
		if c.IsAborted() {
			return
		}
		load(c)
		if c.IsAborted() {
			return
		}
		require(c)
	}
}

// SetupRoutes mounts the HTTP API. valkey is nil when REDIS_URL is unset; hub is nil when this
// process runs no WebSocket gateway (every role but all and realtime).
func SetupRoutes(r *gin.Engine, dbService coreServices.DatabaseService, valkey coreServices.ValkeyService, hub *realtime.Hub) {
	d := baseDeps(r, dbService, valkey, hub, false)

	billingService := registerAuthAndBilling(d)
	registerJobsMonitor(d, valkey)
	chains := registerTenancy(d, billingService)
	registerTracking(d, chains, valkey)
	inventorySvcs := registerBusinessModules(d, chains)
	registerImport(d, chains, inventorySvcs)
	registerGeo(d, chains)
	coreRoutes.RegisterStorageRoutes(d.apiV1, coreControllers.NewStorageController(d.purposes), d.authMiddleware, chains.tenant)

	registerSwagger(r)
}

// SetupRealtimeRoutes mounts what the realtime role serves (TRACK-025): the probes and the WebSocket
// endpoint, behind the same auth and tenant chain as the API — nothing else.
func SetupRealtimeRoutes(r *gin.Engine, dbService coreServices.DatabaseService, valkey coreServices.ValkeyService, hub *realtime.Hub) {
	// Its readiness is Valkey's: without pub/sub a gateway has nothing to relay.
	d := baseDeps(r, dbService, valkey, hub, true)
	chains := tenancyChains(d)
	if hub == nil || !config.ModularAppConfig.Core.IsModuleEnabled("tracking") {
		return
	}
	socket := trackingSocket(d, chains, liveService(d, valkey))
	d.apiV1.GET("/tracking/ws", socket...)
}

// baseDeps is what every role that serves HTTP shares: validators, the probes (registered before any
// middleware, so a probe is never rate-limited — INFRA-002), the stored-file client, the global middleware
// and the services modules are built from.
func baseDeps(r *gin.Engine, dbService coreServices.DatabaseService, valkey coreServices.ValkeyService, hub *realtime.Hub, needsValkey bool) routeDeps {
	coreValidators.RegisterValidations()
	coreRoutes.RegisterHealthRoutes(r, dbService, valkey, needsValkey)

	authConf := config.ModularAppConfig.Auth
	jwtService := authServices.NewJWTService(
		authConf.JWTSecretKey,
		authConf.JWTRefreshKey,
		authConf.JWTExpiration,
		authConf.JWTRefreshExpiration,
	)

	// Stored files (INFRA-007): one client, so every bucket shares its keep-alive pool to the store.
	coreConf := config.ModularAppConfig.Core
	store, err := storage.NewClient(coreConf.StorageEndpoint, coreConf.StorageAccessKey, coreConf.StorageSecretKey)
	if err != nil {
		log.Fatalf("❌ %v", err)
	}

	useGlobalMiddleware(r, valkey)

	media := store.Bucket(coreConf.StorageMediaBucket)
	purposes := storage.NewPurposes()
	purposes.Register(coreServices.NewAvatarPurpose(media))

	return routeDeps{
		engine:         r,
		apiV1:          r.Group("/api/v1"),
		db:             dbService,
		jwt:            jwtService,
		authMiddleware: authMW.AuthMiddleware(jwtService),
		email:          coreServices.NewEmailService(),
		media:          coreServices.NewMediaService(media),
		documents:      store.Bucket(coreConf.StorageDocumentsBucket),
		purposes:       purposes,
		users:          userServices.NewUserService(userRepos.NewUserRepository(dbService)),
		userAudit:      userServices.NewUserAuditService(userRepos.NewUserAuditRepository(dbService)),
		router:         geoRouting.NewRouter(config.ModularAppConfig.Geo),
		valkey:         valkey,
		hub:            hub,
	}
}

// useGlobalMiddleware applies to every route registered after it.
func useGlobalMiddleware(r *gin.Engine, valkey coreServices.ValkeyService) {
	// CORS — must be registered before the rate limiter so preflight OPTIONS
	// requests are handled before they hit the limiter.
	// If-None-Match in, ETag out: a cached read (TRACK-028's route line) is revalidated by the page itself.
	r.Use(cors.New(cors.Config{
		AllowOrigins:     config.ModularAppConfig.Core.AllowedOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "Accept-Language", "X-Tenant-Slug", "If-None-Match"},
		ExposeHeaders:    []string{"ETag"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// Limitador de solicitudes — RATE_LIMIT_PER_SECOND por IP con ráfagas de RATE_LIMIT_BURST, compartido entre
	// réplicas cuando hay Valkey. La suite de tests comparte un solo router entre
	// todos los tests, así que necesita un límite alto para no rechazarse a sí misma.
	coreConf := config.ModularAppConfig.Core
	r.Use(coreMiddleware.RateLimit(valkey, coreConf.RateLimitPerSecond, coreConf.RateLimitBurst))

	// Language middleware - detects lang from ?lang=es or Accept-Language header
	r.Use(coreMiddleware.LanguageMiddleware())
}

// registerAuthAndBilling mounts the modules every server has. Billing is always
// initialized: plan info is needed cross-module.
func registerAuthAndBilling(d routeDeps) billingInterfaces.BillingService {
	// Try to load regexes from standard location
	// If not found (e.g., in tests), use a basic parser
	parser, err := uaparser.New("./config/regexes.yaml")
	if err != nil {
		log.Printf("⚠️  Warning: Could not load UA parser regexes: %v (using nil parser)", err)
		parser = nil
	}

	authService := authServices.NewAuthService(authRepos.NewAuthRepository(d.db))
	authController := authControllers.NewAuthController(authService, d.jwt, d.email, parser, d.db)
	sessionController := authControllers.NewSessionController(authService)
	authRoutes.RegisterAuthRoutes(d.apiV1, authController, sessionController, d.jwt)
	authRoutes.RegisterOtpRoutes(d.apiV1, d.db, config.ModularAppConfig.Auth, d.jwt)
	authRoutes.RegisterPushDeviceRoutes(d.apiV1, authControllers.NewPushDeviceController(
		authServices.NewPushDeviceService(authRepos.NewPushDeviceRepository(d.db))), d.jwt)

	// Billing module — always enabled, user-scoped (no tenant required)
	billingService := billingServices.NewBillingService(billingRepos.NewBillingRepository(d.db))
	billingController := billingControllers.NewBillingController(billingService)
	billingRoutes.RegisterBillingRoutes(d.apiV1, billingController, d.authMiddleware, d.jwt)
	log.Println("✅ Billing module routes registered")

	return billingService
}

// registerJobsMonitor mounts asynqmon — the queues of every tenant, so platform
// admins only (INFRA-001). Nothing without Valkey.
func registerJobsMonitor(d routeDeps, valkey coreServices.ValkeyService) {
	if valkey == nil {
		return
	}
	jobsMonitor := d.engine.Group(coreJobs.MonitorPath, d.authMiddleware, authMW.RequireSystemRole(coreModels.SystemRoleSuperAdmin))
	jobsMonitor.Any("/*path", gin.WrapH(coreJobs.Monitor(valkey)))
}

// registerTenancy mounts tenancy, users and the platform group when tenancy is
// enabled, and returns the tenant chains business routes mount behind.
func registerTenancy(d routeDeps, billingService billingInterfaces.BillingService) tenantChains {
	chains := tenancyChains(d)
	if chains.modules == nil {
		return chains
	}

	tenantService := tenancyServices.NewTenantService(tenancyRepos.NewTenantRepository(d.db), d.db)
	tenantController := tenancyControllers.NewTenantController(tenantService, billingService)
	permissionController := tenancyControllers.NewPermissionController(tenancyServices.NewPermissionService())
	roleController := tenancyControllers.NewRoleController(tenancyServices.NewRoleService(tenancyRepos.NewRoleRepository(d.db)))
	moduleController := tenancyControllers.NewModuleController(chains.modules)

	// Register tenant, permission, role and module routes
	tenancyRoutes.RegisterTenantRoutes(d.apiV1, tenantController, tenantService, d.jwt)
	tenancyRoutes.RegisterPermissionRoutes(d.apiV1, permissionController, d.jwt)
	tenancyRoutes.RegisterRoleRoutes(d.apiV1, roleController, tenantService, d.jwt)
	tenancyRoutes.RegisterModuleRoutes(d.apiV1, moduleController, tenantService, chains.modules, d.jwt)

	// Users module — tenant-scoped: requires X-Tenant-Slug + owner/admin role
	userController := userControllers.NewUserController(d.users, d.email, d.userAudit)
	userAuditController := userControllers.NewUserAuditController(d.userAudit)
	userRoutes.RegisterUserRoutes(d.apiV1, userController, userAuditController, d.jwt, chains.tenant)

	// Platform routes — cross-tenant, super_admin only, no tenant scope
	platform := d.apiV1.Group("/platform")
	platform.Use(d.authMiddleware, authMW.RequireSystemRole(coreModels.SystemRoleSuperAdmin))
	// Register platform-admin routes here as modules are added:
	// platformRoutes.RegisterPlatformRoutes(platform, ...)

	log.Println("✅ Tenancy module enabled and routes registered")
	return chains
}

// tenancyChains builds the tenant chains without registering any route: the API and the realtime
// role both mount behind them. Without tenancy both are no-ops and modules is nil.
func tenancyChains(d routeDeps) tenantChains {
	noop := gin.HandlerFunc(func(c *gin.Context) { c.Next() })
	chains := tenantChains{tenant: noop, portal: noop, socket: noop}
	tenancyConf := config.ModularAppConfig.Tenancy
	if !config.ModularAppConfig.Core.IsModuleEnabled("tenancy") || tenancyConf == nil || !tenancyConf.Enabled {
		return chains
	}
	// TenantMiddlewareFromHeader always requires X-Tenant-Slug; super_admin can switch to any
	// tenant, regular users must be members.
	tenantService := tenancyServices.NewTenantService(tenancyRepos.NewTenantRepository(d.db), d.db)
	chains.modules = tenancyServices.NewModuleService(tenancyRepos.NewModuleRepository(d.db))
	chains.tenant = tenancyMW.TenantMiddlewareFromHeader(tenantService)
	chains.portal = tenancyMW.TenantMiddlewareFromHeader(tenantService, tenancyModels.RolePortal)
	chains.driver = tenancyMW.TenantMiddlewareFromHeader(tenantService, tenancyModels.RoleDriver)
	chains.socket = tenancyMW.TenantMiddlewareFromHeader(tenantService, tenancyModels.RolePortal, tenancyModels.RoleDriver)
	chains.device = tenancyMW.GPSDeviceMiddleware(tenancyRepos.NewGPSDeviceRepository(d.db))
	return chains
}

// liveService is the snapshots and the ETA (TRACK-025 D1): the geo router for the one route call,
// geo's own ETA for its fallbacks, Valkey for the 30 s cache.
func liveService(d routeDeps, valkey coreServices.ValkeyService) *trackingServices.LiveService {
	geoConf := config.ModularAppConfig.Geo
	eta := trackingServices.NewEtaService(d.router, geoServices.NewGeoService(nil, d.router, nil, geoConf), valkey)
	return trackingServices.NewLiveService(trackingRepos.NewTrackingRepository(d.db), eta)
}

// trackingSocket is GET /tracking/ws's handler list, one middleware per handler: the browser's
// subprotocol token and ?tenant_slug= become headers, then auth, the chain that lets a guardian and a
// driver through, the module check, and the upgrade. It also gives the hub its ETA source.
func trackingSocket(d routeDeps, chains tenantChains, live *trackingServices.LiveService) []gin.HandlerFunc {
	withTenant := func(ctx context.Context, session realtime.Session) context.Context {
		if session.DatabaseURL == "" {
			return ctx
		}
		return context.WithValue(ctx, coreServices.TenantDatabaseURLKey, session.DatabaseURL)
	}
	d.hub.SetEta(func(ctx context.Context, session realtime.Session, tripID uuid.UUID, riderID *uuid.UUID, sentAt time.Time) (json.RawMessage, time.Time, bool) {
		if riderID != nil {
			return riderEta(withTenant(ctx, session), live, session, tripID, *riderID, sentAt)
		}
		eta, ok := live.CachedEta(ctx, tripID)
		if !ok {
			var err error
			if _, eta, err = live.Trip(withTenant(ctx, session), session.TenantID, tripID); err != nil {
				log.Printf("⚠️  realtime: eta %s: %v", tripID, err)
				return nil, time.Time{}, false
			}
		}
		if len(eta.Eta) == 0 {
			return nil, time.Time{}, false
		}
		data, err := json.Marshal(map[string]any{"trip_id": tripID, "eta": eta.Eta})
		return data, eta.ComputedAt, err == nil
	})
	check := func(ctx context.Context, session realtime.Session, channel string) (bool, error) {
		return live.CanSubscribe(withTenant(ctx, session), session.TenantID, session.UserID, session.Guardian, session.Organization, channel)
	}
	trackingConf := config.ModularAppConfig.Tracking
	handlers := []gin.HandlerFunc{realtime.BrowserAuth(), d.authMiddleware, chains.socket}
	if chains.modules != nil {
		handlers = append(handlers, tenancyMW.LoadTenantModules(chains.modules), tenancyMW.RequireModule("tracking"))
	}
	return append(handlers, d.hub.Serve(check,
		realtime.OriginPatterns(config.ModularAppConfig.Core.AllowedOrigins),
		time.Duration(trackingConf.WSPingSeconds)*time.Second))
}

// riderEta is a rider channel's ETA frame: the trip's estimate cut to the rider's pending stops. The
// subscribe check already scoped the channel, so the snapshot is read unscoped; it is read only when
// the cached estimate is newer than the one sent — one query per 30 s per watched rider.
func riderEta(ctx context.Context, live *trackingServices.LiveService, session realtime.Session, tripID, riderID uuid.UUID, sentAt time.Time) (json.RawMessage, time.Time, bool) {
	if cached, ok := live.CachedEta(ctx, tripID); ok && cached.ComputedAt.Equal(sentAt) {
		return nil, time.Time{}, false
	}
	rider, computedAt, err := live.Rider(ctx, session.TenantID, riderID, nil, nil)
	if err != nil {
		log.Printf("⚠️  realtime: rider eta %s: %v", riderID, err)
		return nil, time.Time{}, false
	}
	if rider.TripID == nil || *rider.TripID != tripID || len(rider.Eta) == 0 {
		return nil, time.Time{}, false
	}
	data, err := json.Marshal(map[string]any{"trip_id": tripID, "eta": rider.Eta})
	return data, computedAt, err == nil
}

// registerTracking mounts tracking behind the tenant chain plus its module check. valkey is where the
// GPS ingest queues points; nil stores them inline (TRACK-010 D2).
func registerTracking(d routeDeps, chains tenantChains, valkey coreServices.ValkeyService) {
	if !config.ModularAppConfig.Core.IsModuleEnabled("tracking") {
		return
	}
	trackingService := trackingServices.NewTrackingService(trackingRepos.NewTrackingRepository(d.db))
	// Scans live in the documents bucket (INFRA-007); the upload ticket learns their types and cap here.
	trackingDocumentFiles := trackingServices.NewDocumentFiles(d.documents, config.ModularAppConfig.Tracking)
	d.purposes.Register(trackingDocumentFiles.Purpose())
	trackingController := trackingControllers.NewTrackingController(trackingService, trackingDocumentFiles)

	trackingChain := func(tenant gin.HandlerFunc) gin.HandlerFunc { return chains.withModule(tenant, "tracking") }

	// The GPS ingest: a device token runs the device chain, anything else is a driver's session.
	trackingConf := config.ModularAppConfig.Tracking
	ingestController := trackingControllers.NewIngestController(trackingServices.NewPositionIngest(
		trackingRepos.NewTrackingRepository(d.db), valkey, trackingConf.GPSStreamMaxLen, trackingConf.IngestRadii()))
	// Each handler below is one middleware call, so each one's c.Next() moves to the next handler as
	// gin intends; wrapping two of them in one closure would run the controller from inside the first.
	var ingestAuth []gin.HandlerFunc
	if chains.device != nil {
		isDevice := func(c *gin.Context) bool { return c.GetHeader(tenancyMW.DeviceTokenHeader) != "" }
		ingestAuth = []gin.HandlerFunc{
			func(c *gin.Context) {
				if isDevice(c) {
					chains.device(c)
				} else {
					d.authMiddleware(c)
				}
			},
			func(c *gin.Context) {
				if isDevice(c) {
					c.Next()
				} else {
					chains.driver(c)
				}
			},
		}
		if chains.modules != nil {
			ingestAuth = append(ingestAuth, tenancyMW.LoadTenantModules(chains.modules), tenancyMW.RequireModule("tracking"))
		}
	}

	// The driver's phone (TRACK-011): the ingest's driver chain, one middleware per handler.
	var driverAuth []gin.HandlerFunc
	if chains.driver != nil {
		driverAuth = []gin.HandlerFunc{d.authMiddleware, chains.driver}
		if chains.modules != nil {
			driverAuth = append(driverAuth, tenancyMW.LoadTenantModules(chains.modules), tenancyMW.RequireModule("tracking"))
		}
	}
	driverRoutes := trackingRoutes.Driver{
		Controller: trackingControllers.NewDriverController(trackingServices.NewDriverService(trackingRepos.NewTrackingRepository(d.db))),
		Auth:       driverAuth,
	}

	// The guardian's phone (TRACK-012): the portal chain, one middleware per handler.
	var portalAuth []gin.HandlerFunc
	if chains.modules != nil {
		portalAuth = []gin.HandlerFunc{d.authMiddleware, chains.portal,
			tenancyMW.LoadTenantModules(chains.modules), tenancyMW.RequireModule("tracking")}
	}
	portalRoutes := trackingRoutes.Portal{
		Controller: trackingControllers.NewPortalController(trackingServices.NewNotificationService(trackingRepos.NewTrackingRepository(d.db))),
		Auth:       portalAuth,
	}

	// The live snapshots, and the socket when this process runs the gateway (TRACK-025).
	live := liveService(d, valkey)
	liveRoutes := trackingRoutes.Live{Controller: trackingControllers.NewLiveController(live)}
	if d.hub != nil {
		liveRoutes.Socket = trackingSocket(d, chains, live)
	}

	// Register tracking routes with tenant middleware and JWT service for auth. The guardian's
	// two reads run the same chain with the portal refusal lifted (TRACK-017 D1).
	trackingRoutes.RegisterTrackingRoutes(d.engine, trackingController, trackingChain(chains.tenant), trackingChain(chains.portal), d.jwt,
		trackingRoutes.Ingest{Controller: ingestController, Auth: ingestAuth}, liveRoutes, driverRoutes, portalRoutes,
		trackingControllers.NewAccessController(trackingServices.NewAccessService(trackingRepos.NewTrackingRepository(d.db), d.email)))
	log.Println("✅ Tracking module enabled and routes registered")
}

// registerBusinessModules mounts inventory, sales and purchasing when enabled.
// The inventory services it returns are zero when inventory is off.
func registerBusinessModules(d routeDeps, chains tenantChains) inventoryRoutes.InventoryServices {
	coreConf := config.ModularAppConfig.Core

	var inventorySvcs inventoryRoutes.InventoryServices
	if coreConf.IsModuleEnabled("inventory") {
		inventorySvcs = inventoryRoutes.NewInventoryServices(d.db, d.media)
		inventoryRoutes.RegisterInventoryRoutes(d.apiV1, inventorySvcs, d.authMiddleware, chains.withModule(chains.tenant, "inventory"))
		log.Println("✅ Inventory module (with Batches) enabled and routes registered")
	}

	if coreConf.IsModuleEnabled("sales") {
		customerController := salesControllers.NewCustomerController(
			salesServices.NewCustomerService(salesRepos.NewCustomerRepository(d.db)))
		salesOrderController := salesControllers.NewSalesOrderController(
			salesServices.NewSalesOrderService(salesRepos.NewSalesOrderRepository(d.db)))

		// Register sales routes (with JWT auth + tenant middleware)
		salesRoutes.SetupSalesRoutes(d.apiV1, customerController, salesOrderController, d.authMiddleware, chains.withModule(chains.tenant, "sales"))
		log.Println("✅ Sales module enabled and routes registered")
	}

	if coreConf.IsModuleEnabled("purchasing") {
		purchasingService := purchasingServices.NewPurchasingService(purchasingRepos.NewPurchasingRepository(d.db))
		purchasingRoutes.RegisterPurchasingRoutes(d.apiV1, purchasingService, d.authMiddleware, chains.withModule(chains.tenant, "purchasing"))
		log.Println("✅ Purchasing module enabled and routes registered")
	}

	return inventorySvcs
}

// registerImport mounts the generic CSV import, tenant-scoped. The registry sits
// after every module block and outside all of them (INV-015 D1): its
// descriptors come from more than one module, and the tenant server —
// the only one that has inventory — does not enable tenancy, so a
// registry built inside that block left /import unreachable exactly
// where it is needed.
func registerImport(d routeDeps, chains tenantChains, inventorySvcs inventoryRoutes.InventoryServices) {
	coreConf := config.ModularAppConfig.Core
	importRegistry := importServices.NewRegistry()
	if coreConf.IsModuleEnabled("users") {
		importRegistry.Register("users", userServices.NewUsersImportDescriptor(d.users, d.userAudit, d.email, d.db, d.media))
	}
	if coreConf.IsModuleEnabled("inventory") {
		// Registration order is import order (IMPORT-001 A1-D1): the tree the
		// catalogue files into, the catalogue, then what points at its SKUs.
		importRegistry.Register("inventory", inventoryServices.NewCategoriesImportDescriptor(
			inventorySvcs.Category, inventorySvcs.ImportLookup))
		importRegistry.Register("inventory", inventoryServices.NewProductsImportDescriptor(
			inventorySvcs.Product, inventorySvcs.Category, inventorySvcs.Movement, inventorySvcs.Stock,
			inventorySvcs.Sku, inventorySvcs.ImportLookup, d.media, config.GetConfig().Inventory))
		importRegistry.Register("inventory", inventoryServices.NewProductStockImportDescriptor(
			inventorySvcs.Movement, inventorySvcs.Stock, inventorySvcs.ImportLookup))
		importRegistry.Register("inventory", inventoryServices.NewProductBatchesImportDescriptor(
			inventorySvcs.Batch, inventorySvcs.ImportLookup))
	}
	if coreConf.IsModuleEnabled("tracking") {
		// What a transport tenant loads by the dozen when it starts (APP-009): the policy and the
		// places first, then the fleet and the people.
		tracking := trackingServices.NewTrackingService(trackingRepos.NewTrackingRepository(d.db))
		importRegistry.Register("tracking", trackingServices.NewDocumentTypesImportDescriptor(tracking))
		importRegistry.Register("tracking", trackingServices.NewStopPlacesImportDescriptor(tracking))
		importRegistry.Register("tracking", trackingServices.NewVehiclesImportDescriptor(tracking))
		importRegistry.Register("tracking", trackingServices.NewRidersImportDescriptor(tracking))
	}
	importController := importControllers.NewImportController(importServices.NewImportService(importRegistry))
	importRoutes.RegisterImportRoutes(d.apiV1, importController, d.jwt, chains.tenant)
	log.Println("✅ Import module routes registered")
}

// registerGeo mounts address search and routing (INFRA-003) — the tenant server. Observed durations
// (the ETA's second source) are nil until TRACK-010.
func registerGeo(d routeDeps, chains tenantChains) {
	if !config.ModularAppConfig.Core.IsModuleEnabled("geo") {
		return
	}
	geoConf := config.ModularAppConfig.Geo
	geoService := geoServices.NewGeoService(geoRepos.NewPlacesRepository(d.db), d.router, nil, geoConf)
	geoRoutes.RegisterGeoRoutes(d.apiV1, geoControllers.NewGeoController(geoService), d.authMiddleware, chains.tenant)
	log.Println("✅ Geo module routes registered")
}
