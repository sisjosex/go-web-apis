package routes

import (
	"josex/web/config"
	"josex/web/controllers"
	"josex/web/interfaces"
	"josex/web/middleware"
	"josex/web/repositories"
	"josex/web/services"
	"time"

	"github.com/gin-gonic/gin"
)

func SetupRoutes(r *gin.Engine, dbService interfaces.DatabaseService) {

	var jwtService = services.NewJWTService(
		config.AppConfig.JwtSecretKey,
		config.AppConfig.JwtRefreshKey,
		time.Duration(config.AppConfig.JwtExpirationSeconds),
		time.Duration(config.AppConfig.JwtRefreshExpirationSeconds),
	)

	userRepository := repositories.NewUserRepository(dbService)
	userService := services.NewUserService(userRepository)

	authController := controllers.NewAuthController(userService, jwtService)
	userController := controllers.NewUserController(userService)

	mainRoutes := r.Group("/api/v1")
	{
		authRoutes := mainRoutes.Group("/auth")
		{
			authRoutes.POST("/login", authController.Login)
			authRoutes.POST("/login_facebook", authController.LoginFacebook)
			authRoutes.POST("/register", authController.Register)
			authRoutes.POST("/refresh_token", authController.RefreshToken)
			protectedRoutes := authRoutes.Use(middleware.AuthMiddleware(jwtService))
			{
				protectedRoutes.POST("/update_profile", authController.UpdateProfile)
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
}
