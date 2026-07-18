package routes

import (
	"josex/web/config"
	authControllers "josex/web/modules/auth/controllers"
	authMW "josex/web/modules/auth/middleware"
	authRepos "josex/web/modules/auth/repositories"
	authRoutes "josex/web/modules/auth/routes"
	authServices "josex/web/modules/auth/services"
	billingControllers "josex/web/modules/billing/controllers"
	billingRepos "josex/web/modules/billing/repositories"
	billingRoutes "josex/web/modules/billing/routes"
	billingServices "josex/web/modules/billing/services"
	coreMiddleware "josex/web/modules/core/middleware"
	coreModels "josex/web/modules/core/models"
	coreServices "josex/web/modules/core/services"
	inventoryRoutes "josex/web/modules/inventory/routes"
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
	tenancyRepos "josex/web/modules/tenancy/repositories"
	tenancyRoutes "josex/web/modules/tenancy/routes"
	tenancyServices "josex/web/modules/tenancy/services"
	trackingControllers "josex/web/modules/tracking/controllers"
	trackingRepos "josex/web/modules/tracking/repositories"
	trackingRoutes "josex/web/modules/tracking/routes"
	trackingServices "josex/web/modules/tracking/services"
	userControllers "josex/web/modules/users/controllers"
	userRepos "josex/web/modules/users/repositories"
	userRoutes "josex/web/modules/users/routes"
	userServices "josex/web/modules/users/services"
	"log"
	"time"

	_ "josex/web/docs"
	_ "josex/web/modules/inventory"
	_ "josex/web/modules/purchasing"
	_ "josex/web/modules/sales"
	_ "josex/web/modules/tracking"
	_ "josex/web/modules/users"
	coreValidators "josex/web/modules/core/validators"

	"github.com/didip/tollbooth/v7"
	"github.com/didip/tollbooth_gin"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"github.com/ua-parser/uap-go/uaparser"
)

func SetupRoutes(r *gin.Engine, dbService coreServices.DatabaseService) {
	// Register custom validators (must be done before any validation runs)
	coreValidators.RegisterValidations()

	authConf := config.ModularAppConfig.Auth
	var jwtService = authServices.NewJWTService(
		authConf.JWTSecretKey,
		authConf.JWTRefreshKey,
		authConf.JWTExpiration,
		authConf.JWTRefreshExpiration,
	)

	// Try to load regexes from standard location
	// If not found (e.g., in tests), use a basic parser
	parser, err := uaparser.New("./config/regexes.yaml")
	if err != nil {
		log.Printf("⚠️  Warning: Could not load UA parser regexes: %v (using nil parser)", err)
		parser = nil
	}

	// Core services
	emailService := coreServices.NewEmailService()

	// Auth module
	authRepository := authRepos.NewAuthRepository(dbService)
	authService := authServices.NewAuthService(authRepository)

	// Users module
	userRepository := userRepos.NewUserRepository(dbService)
	userService := userServices.NewUserService(userRepository)

	// Users audit module
	userAuditRepository := userRepos.NewUserAuditRepository(dbService)
	userAuditService := userServices.NewUserAuditService(userAuditRepository)
	userAuditController := userControllers.NewUserAuditController(userAuditService)

	// Billing module (always initialized — plan info is needed cross-module)
	billingRepository := billingRepos.NewBillingRepository(dbService)
	billingService := billingServices.NewBillingService(billingRepository)
	billingController := billingControllers.NewBillingController(billingService)

	// Controllers
	authController := authControllers.NewAuthController(authService, jwtService, emailService, parser, dbService)
	sessionController := authControllers.NewSessionController(authService)
	userController := userControllers.NewUserController(userService, emailService, userAuditService)

	// CORS — must be registered before the rate limiter so preflight OPTIONS
	// requests are handled before they hit the limiter.
	r.Use(cors.New(cors.Config{
		AllowOrigins:     config.ModularAppConfig.Core.AllowedOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "Accept-Language", "X-Tenant-Slug"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// Limitador de solicitudes
	limiter := tollbooth.NewLimiter(10, nil)         // 10 req/segundo
	limiter.SetTokenBucketExpirationTTL(time.Second) // Define la ventana de tiempo en 1 segundo
	limiter.SetIPLookups([]string{"RemoteAddr", "X-Forwarded-For", "X-Real-IP"})
	r.Use(tollbooth_gin.LimitHandler(limiter))

	// Language middleware - detects lang from ?lang=es or Accept-Language header
	r.Use(coreMiddleware.LanguageMiddleware())

	// API v1 routes
	apiV1 := r.Group("/api/v1")
	{
		// Register module routes
		authRoutes.RegisterAuthRoutes(apiV1, authController, sessionController, jwtService)
		authRoutes.RegisterOtpRoutes(apiV1, dbService, authConf, jwtService)

		// Billing module — always enabled, user-scoped (no tenant required)
		billingRoutes.RegisterBillingRoutes(apiV1, billingController, authMW.AuthMiddleware(jwtService), jwtService)
		log.Println("✅ Billing module routes registered")

		// Get config
		coreConf := config.ModularAppConfig.Core
		tenancyConf := config.ModularAppConfig.Tenancy

		// Auth middleware shared across business modules
		authMiddleware := authMW.AuthMiddleware(jwtService)

		// Tenant middleware — no-op when tenancy is disabled so business routes still register
		tenantMiddleware := gin.HandlerFunc(func(c *gin.Context) { c.Next() })

		// Initialize tenancy services (needed for other modules)
		var moduleService tenancyInterfaces.ModuleService
		if coreConf.IsModuleEnabled("tenancy") && tenancyConf != nil && tenancyConf.Enabled {
			tenantRepository := tenancyRepos.NewTenantRepository(dbService)
			tenantService := tenancyServices.NewTenantService(tenantRepository, dbService)
			tenantController := tenancyControllers.NewTenantController(tenantService, billingService)

			permissionService := tenancyServices.NewPermissionService()
			permissionController := tenancyControllers.NewPermissionController(permissionService)

			roleRepository := tenancyRepos.NewRoleRepository(dbService)
			roleService := tenancyServices.NewRoleService(roleRepository)
			roleController := tenancyControllers.NewRoleController(roleService)

			moduleRepository := tenancyRepos.NewModuleRepository(dbService)
			moduleSvc := tenancyServices.NewModuleService(moduleRepository)
			moduleController := tenancyControllers.NewModuleController(moduleSvc)
			moduleService = moduleSvc

			// Register tenant, permission, role and module routes
			tenancyRoutes.RegisterTenantRoutes(apiV1, tenantController, tenantService, jwtService)
			tenancyRoutes.RegisterPermissionRoutes(apiV1, permissionController, jwtService)
			tenancyRoutes.RegisterRoleRoutes(apiV1, roleController, tenantService, jwtService)
			tenancyRoutes.RegisterModuleRoutes(apiV1, moduleController, tenantService, moduleSvc, jwtService)

			// Override tenant middleware with real implementation
			// TenantMiddlewareFromHeader always requires X-Tenant-Slug;
			// super_admin can switch to any tenant, regular users must be members.
			tenantMiddleware = tenancyMW.TenantMiddlewareFromHeader(tenantService)

			// Users module — tenant-scoped: requires X-Tenant-Slug + owner/admin role
			userRoutes.RegisterUserRoutes(apiV1, userController, userAuditController, jwtService, tenantMiddleware)

			// Platform routes — cross-tenant, super_admin only, no tenant scope
			platform := apiV1.Group("/platform")
			platform.Use(authMiddleware, authMW.RequireSystemRole(coreModels.SystemRoleSuperAdmin))
			// Register platform-admin routes here as modules are added:
			// platformRoutes.RegisterPlatformRoutes(platform, ...)

			log.Println("✅ Tenancy module enabled and routes registered")
		}

		// Tracking module (if enabled)
		if coreConf.IsModuleEnabled("tracking") {
			// Initialize tracking services
			trackingRepository := trackingRepos.NewTrackingRepository(dbService)
			trackingService := trackingServices.NewTrackingService(trackingRepository)
			trackingController := trackingControllers.NewTrackingController(trackingService)

			// Build a composite tenant+module middleware chain for tracking
			trackingTenantMiddleware := tenantMiddleware
			if moduleService != nil {
				trackingTenantMiddleware = gin.HandlerFunc(func(c *gin.Context) {
					tenantMiddleware(c)
					if c.IsAborted() {
						return
					}
					tenancyMW.LoadTenantModules(moduleService)(c)
					if c.IsAborted() {
						return
					}
					tenancyMW.RequireModule("tracking")(c)
				})
			}

			// Register tracking routes with tenant middleware and JWT service for auth
			trackingRoutes.RegisterTrackingRoutes(r, trackingController, trackingTenantMiddleware, jwtService)
			log.Println("✅ Tracking module enabled and routes registered")
		}

		// Inventory module (if enabled)
		if coreConf.IsModuleEnabled("inventory") {
			inventoryRoutes.RegisterInventoryRoutes(apiV1, dbService, authMiddleware, tenantMiddleware)
			log.Println("✅ Inventory module (with Batches) enabled and routes registered")
		}

		// Sales module (if enabled)
		if coreConf.IsModuleEnabled("sales") {
			// Initialize sales services
			customerRepository := salesRepos.NewCustomerRepository(dbService)
			salesOrderRepository := salesRepos.NewSalesOrderRepository(dbService)

			customerService := salesServices.NewCustomerService(customerRepository)
			salesOrderService := salesServices.NewSalesOrderService(salesOrderRepository)

			customerController := salesControllers.NewCustomerController(customerService)
			salesOrderController := salesControllers.NewSalesOrderController(salesOrderService)

			// Register sales routes (with JWT auth + tenant middleware)
			salesRoutes.SetupSalesRoutes(apiV1, customerController, salesOrderController, authMiddleware, tenantMiddleware)
			log.Println("✅ Sales module enabled and routes registered")
		}

		// Purchasing module (if enabled)
		if coreConf.IsModuleEnabled("purchasing") {
			purchasingRepository := purchasingRepos.NewPurchasingRepository(dbService)
			purchasingService := purchasingServices.NewPurchasingService(purchasingRepository)
			purchasingRoutes.RegisterPurchasingRoutes(apiV1, purchasingService, authMiddleware, tenantMiddleware)
			log.Println("✅ Purchasing module enabled and routes registered")
		}
	}

	// Swagger documentation
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
}
