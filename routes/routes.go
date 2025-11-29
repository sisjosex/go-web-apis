package routes

import (
	"josex/web/config"
	authControllers "josex/web/modules/auth/controllers"
	authRepos "josex/web/modules/auth/repositories"
	authRoutes "josex/web/modules/auth/routes"
	authServices "josex/web/modules/auth/services"
	coreServices "josex/web/modules/core/services"
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

	var jwtService = authServices.NewJWTService(
		config.AppConfig.JwtSecretKey,
		config.AppConfig.JwtRefreshKey,
		time.Duration(config.AppConfig.JwtExpirationSeconds),
		time.Duration(config.AppConfig.JwtRefreshExpirationSeconds),
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

	// API v1 routes
	apiV1 := r.Group("/api/v1")
	{
		// Register module routes
		authRoutes.RegisterAuthRoutes(apiV1, authController, sessionController, jwtService)
		userRoutes.RegisterUserRoutes(apiV1, userController, jwtService)
	}

	// Swagger documentation
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
}
