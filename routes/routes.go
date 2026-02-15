package routes

import (
	"josex/web/config"
	authControllers "josex/web/modules/auth/controllers"
	authRepos "josex/web/modules/auth/repositories"
	authRoutes "josex/web/modules/auth/routes"
	authServices "josex/web/modules/auth/services"
	coreMiddleware "josex/web/modules/core/middleware"
	coreServices "josex/web/modules/core/services"
	tenancyControllers "josex/web/modules/tenancy/controllers"
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

	"github.com/didip/tollbooth/v7"
	"github.com/didip/tollbooth_gin"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"github.com/ua-parser/uap-go/uaparser"
)

func SetupRoutes(r *gin.Engine, dbService coreServices.DatabaseService) {

	authConf := config.ModularAppConfig.Auth
	var jwtService = authServices.NewJWTService(
		authConf.JWTSecretKey,
		authConf.JWTRefreshKey,
		authConf.JWTExpiration,
		authConf.JWTRefreshExpiration,
	)

	parser, err := uaparser.New("./config/regexes.yaml")
	if err != nil {
		log.Fatal(err)
	}

	// Core services
	emailService := coreServices.NewEmailService()

	// Auth module
	authRepository := authRepos.NewAuthRepository(dbService)
	authService := authServices.NewAuthService(authRepository)

	// Users module
	userRepository := userRepos.NewUserRepository(dbService)
	userService := userServices.NewUserService(userRepository)

	// Controllers
	authController := authControllers.NewAuthController(authService, jwtService, emailService, parser, dbService)
	sessionController := authControllers.NewSessionController(authService)
	userController := userControllers.NewUserController(userService)

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
		authRoutes.RegisterOtpRoutes(apiV1, dbService, authConf)
		userRoutes.RegisterUserRoutes(apiV1, userController, jwtService)

		// Get config
		coreConf := config.ModularAppConfig.Core
		tenancyConf := config.ModularAppConfig.Tenancy

		// Initialize tenancy services (needed for other modules)
		var tenantMiddleware gin.HandlerFunc
		if coreConf.IsModuleEnabled("tenancy") && tenancyConf != nil && tenancyConf.Enabled {
			tenantRepository := tenancyRepos.NewTenantRepository(dbService)
			tenantService := tenancyServices.NewTenantService(tenantRepository, dbService)
			tenantController := tenancyControllers.NewTenantController(tenantService)

			// Register tenant routes
			tenancyRoutes.RegisterTenantRoutes(apiV1, tenantController, tenantService, jwtService)

			// Create tenant middleware for use in other modules (optional header, allows main DB access for admins)
			tenantMiddleware = tenancyMW.TenantMiddlewareFromHeaderOptional(tenantService)

			log.Println("✅ Tenancy module enabled and routes registered")
		}

		// Tracking module (if enabled)
		if coreConf.IsModuleEnabled("tracking") {
			// Initialize tracking services
			trackingRepository := trackingRepos.NewTrackingRepository(dbService)
			trackingService := trackingServices.NewTrackingService(trackingRepository)
			trackingController := trackingControllers.NewTrackingController(trackingService)

			// Register tracking routes with tenant middleware and JWT service for auth
			trackingRoutes.RegisterTrackingRoutes(r, trackingController, tenantMiddleware, jwtService)
			log.Println("✅ Tracking module enabled and routes registered")
		}
	}

	// Swagger documentation
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
}
