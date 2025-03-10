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

	authController := controllers.NewAuthController(userService, jwtService, parser)
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
			authRoutes.POST("/login", authController.Login)
			authRoutes.POST("/login_facebook", authController.LoginFacebook)
			authRoutes.POST("/register", authController.Register)
			authRoutes.POST("/refresh_token", authController.RefreshToken)
			authRoutes.POST("/confirm_email", authController.ConfirmEmailAddress)
			authRoutes.POST("/request_password_reset", authController.GeneratePasswordResetToken)
			authRoutes.POST("/password_reset", authController.ResetPasswordWithToken)
			protectedRoutes := authRoutes.Use(middleware.AuthMiddleware(jwtService))
			{
				protectedRoutes.POST("/get_profile", authController.GetProfile)
				protectedRoutes.POST("/update_profile", authController.UpdateProfile)
				protectedRoutes.POST("/change_password", authController.ChangePassword)
				protectedRoutes.POST("/request_verify_email", authController.GenerateEmailVerificationToken)
			}
		}

		mainRoutes.Use(middleware.AuthMiddleware(jwtService))
		{
			mainRoutes.POST("/logout", authController.Logout)

			userRoutes := mainRoutes.Group("/users")
			{
				//userRoutes.GET("", controllers.Index)
				userRoutes.POST("", userController.Create)
				userRoutes.PUT(":id", userController.Update)
				//userRoutes.GET(":id", controllers.Show)
				//userRoutes.DELETE(":id", controllers.Delete)
			}
		}
	}

	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
}
