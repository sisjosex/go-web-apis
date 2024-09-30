package routes

import (
	"josex/web/controllers"
	"josex/web/interfaces"
	"josex/web/repositories"
	"josex/web/services"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/ua-parser/uap-go/uaparser"
)

func SetupRoutes(r *gin.Engine, dbService interfaces.DatabaseService) {
	parser, err := uaparser.New("./config/regexes.yaml")
	if err != nil {
		log.Fatal(err)
	}

	userRepository := repositories.NewUserRepository(dbService)
	userService := services.NewUserService(userRepository)
	userController := controllers.NewUserController(userService, parser)

	mainRoutes := r.Group("/api/v1")
	{
		mainRoutes.POST("/login", userController.Login)
		mainRoutes.POST("/register", userController.Register)
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
