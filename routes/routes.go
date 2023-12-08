package routes

import (
	"josex/web/controllers"

	"github.com/gin-gonic/gin"
)

func SetupRoutes(r *gin.Engine) {

	mainRoutes := r.Group("/api/v1")
	{
		userRoutes := mainRoutes.Group("/users")
		{
			userRoutes.GET("", controllers.Index)
			userRoutes.POST("", controllers.Create)
			userRoutes.POST(":id", controllers.Update)
			userRoutes.GET(":id", controllers.Show)
			userRoutes.DELETE(":id", controllers.Delete)
		}
	}
}
