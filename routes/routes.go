package routes

import (
	"josex/web/controllers"
	"josex/web/interfaces"
	"josex/web/repositories"
	"josex/web/services"

	"github.com/gin-gonic/gin"
)

func SetupRoutes(r *gin.Engine, dbService interfaces.DatabaseService) {
	userRepository := repositories.NewUserRepository(dbService)
	userService := services.NewUserService(userRepository)
	userController := controllers.NewUserController(userService)

	mainRoutes := r.Group("/api/v1")
	{
		userRoutes := mainRoutes.Group("/users")
		{
			//userRoutes.GET("", controllers.Index)
			userRoutes.POST("", userController.Create)
			//userRoutes.POST(":id", controllers.Update)
			//userRoutes.GET(":id", controllers.Show)
			//userRoutes.DELETE(":id", controllers.Delete)
		}
	}
}
