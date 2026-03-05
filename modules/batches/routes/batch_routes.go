package routes

import (
	authMiddleware "josex/web/modules/auth/middleware"
	jwtService "josex/web/modules/auth/services"
	batchControllers "josex/web/modules/batches/controllers"
	batchRepositories "josex/web/modules/batches/repositories"
	batchServices "josex/web/modules/batches/services"
	coreServices "josex/web/modules/core/services"

	"github.com/gin-gonic/gin"
)

func RegisterBatchesRoutes(apiV1 *gin.RouterGroup, dbService coreServices.DatabaseService, jwt jwtService.JWTService) {
	// Initialize DI
	repository := batchRepositories.NewBatchRepository(dbService)
	service := batchServices.NewBatchService(repository)
	controller := batchControllers.NewBatchController(service)

	// Protected routes
	protectedRoutes := apiV1.Group("/batches")
	protectedRoutes.Use(authMiddleware.AuthMiddleware(jwt))
	{
		protectedRoutes.POST("", controller.CreateBatch)
		protectedRoutes.GET("/:id", controller.GetBatch)
	}

	// Product batches routes
	productBatchesRoutes := apiV1.Group("/products/:productId/batches")
	productBatchesRoutes.Use(authMiddleware.AuthMiddleware(jwt))
	{
		productBatchesRoutes.GET("", controller.ListBatchesByProduct)
		productBatchesRoutes.GET("/oldest", controller.GetOldestBatchForSale)
	}
}
