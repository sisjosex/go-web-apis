package routes

import (
	"josex/web/controllers"
	"josex/web/interfaces"
	"josex/web/middleware"
	"josex/web/repositories"
	"josex/web/services"

	"github.com/gin-gonic/gin"
)

func SetupRoutes(r *gin.Engine, dbService interfaces.DatabaseService) {

	userRepository := repositories.NewUserRepository(dbService)
	userService := services.NewUserService(userRepository)

	authController := controllers.NewAuthController(userService)
	userController := controllers.NewUserController(userService)

	mainRoutes := r.Group("/api/v1")
	{
		mainRoutes.POST("/login", authController.Login)
		mainRoutes.POST("/login_facebook", authController.LoginFacebook)
		mainRoutes.POST("/register", authController.Register)

		mainRoutes.Use(middleware.AuthMiddleware())
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
