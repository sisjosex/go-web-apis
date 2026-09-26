package routes

import (
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
	coreConfig "josex/web/modules/core/config"
	coreJobs "josex/web/modules/core/jobs"
	coreMiddleware "josex/web/modules/core/middleware"
	coreModels "josex/web/modules/core/models"
	coreRoutes "josex/web/modules/core/routes"
	coreServices "josex/web/modules/core/services"
	geoControllers "josex/web/modules/geo/controllers"
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

	_ "josex/web/docs"
	coreValidators "josex/web/modules/core/validators"
	_ "josex/web/modules/inventory"
	_ "josex/web/modules/purchasing"
	_ "josex/web/modules/sales"
	_ "josex/web/modules/tracking"
	_ "josex/web/modules/users"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
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
}

// tenantChains are what business routes mount behind. Without tenancy both are
// no-ops, so business routes still register, and modules is nil.
type tenantChains struct {
	tenant gin.HandlerFunc
	// Same chain, but a portal guardian passes the mobile-only refusal (TRACK-017 D1). Only the
	// routes a guardian may read are registered behind it.
	portal  gin.HandlerFunc
	modules tenancyInterfaces.ModuleService
}

// SetupRoutes mounts the HTTP API. valkey is nil when REDIS_URL is unset.
func SetupRoutes(r *gin.Engine, dbService coreServices.DatabaseService, valkey coreServices.ValkeyService) {
	// Register custom validators (must be done before any validation runs)
	coreValidators.RegisterValidations()

	// Health probes first: gin applies only the middleware registered before a
	// route, so mounting /livez and /readyz here keeps the container's
	// healthcheck out of the rate limiter (INFRA-002).
	coreRoutes.RegisterHealthRoutes(r, dbService, valkey, false)

	authConf := config.ModularAppConfig.Auth
	jwtService := authServices.NewJWTService(
		authConf.JWTSecretKey,
		authConf.JWTRefreshKey,
		authConf.JWTExpiration,
		authConf.JWTRefreshExpiration,
	)

	// Stored files (D5): served straight off MEDIA_ROOT, which is why the media
	// service writes opaque UUID names and never the uploader's own filename.
	r.Static("/media", coreConfig.MediaRoot())

	useGlobalMiddleware(r, valkey)

	d := routeDeps{
		engine:         r,
		apiV1:          r.Group("/api/v1"),
		db:             dbService,
		jwt:            jwtService,
		authMiddleware: authMW.AuthMiddleware(jwtService),
		email:          coreServices.NewEmailService(),
		media:          coreServices.NewMediaService(),
		users:          userServices.NewUserService(userRepos.NewUserRepository(dbService)),
		userAudit:      userServices.NewUserAuditService(userRepos.NewUserAuditRepository(dbService)),
	}

	billingService := registerAuthAndBilling(d)
	registerJobsMonitor(d, valkey)
	chains := registerTenancy(d, billingService)
	registerTracking(d, chains)
	inventorySvcs := registerBusinessModules(d, chains)
	registerImport(d, chains, inventorySvcs)
	registerGeo(d, chains)

	// Swagger documentation
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
}

// useGlobalMiddleware applies to every route registered after it.
func useGlobalMiddleware(r *gin.Engine, valkey coreServices.ValkeyService) {
	// CORS — must be registered before the rate limiter so preflight OPTIONS
	// requests are handled before they hit the limiter.
	r.Use(cors.New(cors.Config{
		AllowOrigins:     config.ModularAppConfig.Core.AllowedOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "Accept-Language", "X-Tenant-Slug"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// Limitador de solicitudes — RATE_LIMIT_PER_SECOND por IP, compartido entre
	// réplicas cuando hay Valkey. La suite de tests comparte un solo router entre
	// todos los tests, así que necesita un límite alto para no rechazarse a sí misma.
	r.Use(coreMiddleware.RateLimit(valkey, config.ModularAppConfig.Core.RateLimitPerSecond))

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
	noop := gin.HandlerFunc(func(c *gin.Context) { c.Next() })
	chains := tenantChains{tenant: noop, portal: noop}

	tenancyConf := config.ModularAppConfig.Tenancy
	if !config.ModularAppConfig.Core.IsModuleEnabled("tenancy") || tenancyConf == nil || !tenancyConf.Enabled {
		return chains
	}

	tenantService := tenancyServices.NewTenantService(tenancyRepos.NewTenantRepository(d.db), d.db)
	tenantController := tenancyControllers.NewTenantController(tenantService, billingService)
	permissionController := tenancyControllers.NewPermissionController(tenancyServices.NewPermissionService())
	roleController := tenancyControllers.NewRoleController(tenancyServices.NewRoleService(tenancyRepos.NewRoleRepository(d.db)))
	moduleSvc := tenancyServices.NewModuleService(tenancyRepos.NewModuleRepository(d.db))
	moduleController := tenancyControllers.NewModuleController(moduleSvc)
	chains.modules = moduleSvc

	// Register tenant, permission, role and module routes
	tenancyRoutes.RegisterTenantRoutes(d.apiV1, tenantController, tenantService, d.jwt)
	tenancyRoutes.RegisterPermissionRoutes(d.apiV1, permissionController, d.jwt)
	tenancyRoutes.RegisterRoleRoutes(d.apiV1, roleController, tenantService, d.jwt)
	tenancyRoutes.RegisterModuleRoutes(d.apiV1, moduleController, tenantService, moduleSvc, d.jwt)

	// Override tenant middleware with real implementation
	// TenantMiddlewareFromHeader always requires X-Tenant-Slug;
	// super_admin can switch to any tenant, regular users must be members.
	chains.tenant = tenancyMW.TenantMiddlewareFromHeader(tenantService)
	chains.portal = tenancyMW.TenantMiddlewareFromHeader(tenantService, tenancyModels.RolePortal)

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

// registerTracking mounts tracking behind the tenant chain plus its module check.
func registerTracking(d routeDeps, chains tenantChains) {
	if !config.ModularAppConfig.Core.IsModuleEnabled("tracking") {
		return
	}
	trackingService := trackingServices.NewTrackingService(trackingRepos.NewTrackingRepository(d.db))
	// The document file store is config, not state: where scans live and how big one may be
	// (TRACK-016 D3).
	trackingDocumentFiles := trackingServices.NewDocumentFileStore(config.ModularAppConfig.Tracking)
	trackingController := trackingControllers.NewTrackingController(trackingService, trackingDocumentFiles)

	// Build a composite tenant+module middleware chain for tracking
	trackingChain := func(tenant gin.HandlerFunc) gin.HandlerFunc {
		if chains.modules == nil {
			return tenant
		}
		return gin.HandlerFunc(func(c *gin.Context) {
			tenant(c)
			if c.IsAborted() {
				return
			}
			tenancyMW.LoadTenantModules(chains.modules)(c)
			if c.IsAborted() {
				return
			}
			tenancyMW.RequireModule("tracking")(c)
		})
	}

	// Register tracking routes with tenant middleware and JWT service for auth. The guardian's
	// two reads run the same chain with the portal refusal lifted (TRACK-017 D1).
	trackingRoutes.RegisterTrackingRoutes(d.engine, trackingController, trackingChain(chains.tenant), trackingChain(chains.portal), d.jwt)
	log.Println("✅ Tracking module enabled and routes registered")
}

// registerBusinessModules mounts inventory, sales and purchasing when enabled.
// The inventory services it returns are zero when inventory is off.
func registerBusinessModules(d routeDeps, chains tenantChains) inventoryRoutes.InventoryServices {
	coreConf := config.ModularAppConfig.Core

	var inventorySvcs inventoryRoutes.InventoryServices
	if coreConf.IsModuleEnabled("inventory") {
		inventorySvcs = inventoryRoutes.NewInventoryServices(d.db)
		inventoryRoutes.RegisterInventoryRoutes(d.apiV1, inventorySvcs, d.authMiddleware, chains.tenant)
		log.Println("✅ Inventory module (with Batches) enabled and routes registered")
	}

	if coreConf.IsModuleEnabled("sales") {
		customerController := salesControllers.NewCustomerController(
			salesServices.NewCustomerService(salesRepos.NewCustomerRepository(d.db)))
		salesOrderController := salesControllers.NewSalesOrderController(
			salesServices.NewSalesOrderService(salesRepos.NewSalesOrderRepository(d.db)))

		// Register sales routes (with JWT auth + tenant middleware)
		salesRoutes.SetupSalesRoutes(d.apiV1, customerController, salesOrderController, d.authMiddleware, chains.tenant)
		log.Println("✅ Sales module enabled and routes registered")
	}

	if coreConf.IsModuleEnabled("purchasing") {
		purchasingService := purchasingServices.NewPurchasingService(purchasingRepos.NewPurchasingRepository(d.db))
		purchasingRoutes.RegisterPurchasingRoutes(d.apiV1, purchasingService, d.authMiddleware, chains.tenant)
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
	geoService := geoServices.NewGeoService(
		geoRepos.NewPlacesRepository(d.db), geoRouting.NewRouter(geoConf), nil, geoConf)
	geoRoutes.RegisterGeoRoutes(d.apiV1, geoControllers.NewGeoController(geoService), d.authMiddleware, chains.tenant)
	log.Println("✅ Geo module routes registered")
}
