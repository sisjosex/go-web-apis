package routes

import (
	"josex/web/config"
	"josex/web/controllers"
	"josex/web/interfaces"
	"josex/web/middleware"
	"josex/web/repositories"
	"josex/web/services"
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

func SetupRoutes(r *gin.Engine, dbService interfaces.DatabaseService) {

	var jwtService = services.NewJWTService(
		config.AppConfig.JwtSecretKey,
		config.AppConfig.JwtRefreshKey,
		time.Duration(config.AppConfig.JwtExpirationSeconds),
		time.Duration(config.AppConfig.JwtRefreshExpirationSeconds),
	)

	parser, err := uaparser.New("./config/regexes.yaml")
	if err != nil {
		log.Fatal(err)
	}

	userRepository := repositories.NewUserRepository(dbService)
	userService := services.NewUserService(userRepository)

	authController := controllers.NewAuthController(userService, jwtService, parser, dbService)
	sessionController := controllers.NewSessionController(userService)
	userController := controllers.NewUserController(userService)

	// Limitador de solicitudes
	limiter := tollbooth.NewLimiter(10, nil)         // 5 req/segundo
	limiter.SetTokenBucketExpirationTTL(time.Second) // Define la ventana de tiempo en 1 segundo
	limiter.SetIPLookups([]string{"RemoteAddr", "X-Forwarded-For", "X-Real-IP"})
	r.Use(tollbooth_gin.LimitHandler(limiter))

	mainRoutes := r.Group("/api/v1")
	{
		authRoutes := mainRoutes.Group("/auth")
		{
			// Authentication endpoints (public)
			authRoutes.POST("/login", authController.Login)
			authRoutes.POST("/login/facebook", authController.LoginFacebook)
			authRoutes.POST("/register", authController.Register)
			authRoutes.POST("/token/refresh", authController.RefreshToken)

			// Email verification (public - requires token in body)
			authRoutes.PUT("/email/verification", authController.ConfirmEmailAddress)

			// Password reset (public - requires token in body)
			authRoutes.POST("/password/reset", authController.GeneratePasswordResetToken)
			authRoutes.PUT("/password/reset", authController.ResetPasswordWithToken)

			// Protected routes (require authentication)
			protectedRoutes := authRoutes.Use(middleware.AuthMiddleware(jwtService))
			{
				// Profile management
				protectedRoutes.GET("/profile", authController.GetProfile)
				protectedRoutes.PATCH("/profile", authController.UpdateProfile)

				// Password management
				protectedRoutes.PUT("/password", authController.ChangePassword)

				// Email verification request
				protectedRoutes.POST("/email/verification", authController.GenerateEmailVerificationToken)

				// Session management
				protectedRoutes.GET("/sessions", sessionController.GetActiveSessions)
				protectedRoutes.DELETE("/sessions/:id", sessionController.LogoutSession)
				protectedRoutes.DELETE("/sessions", sessionController.LogoutAllSessions)
				protectedRoutes.POST("/logout", authController.Logout)
			}
		}

		mainRoutes.Use(middleware.AuthMiddleware(jwtService))
		{
			userRoutes := mainRoutes.Group("/users")
			{
				userRoutes.GET("", userController.ListUsers)
				userRoutes.POST("", userController.Create)
				userRoutes.GET("/:id", userController.GetUserById)
				userRoutes.PUT("/:id", userController.Update)
				userRoutes.DELETE("/:id", userController.SoftDeleteUser)
			}
		}
	}

	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
}
