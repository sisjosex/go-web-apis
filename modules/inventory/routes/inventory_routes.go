package routes

import (
	coreServices "josex/web/modules/core/services"
	"josex/web/modules/inventory/controllers"
	"josex/web/modules/inventory/repositories"
	inventoryServices "josex/web/modules/inventory/services"

	"github.com/gin-gonic/gin"
)

func RegisterInventoryRoutes(
	router *gin.RouterGroup,
	dbService coreServices.DatabaseService,
) {
	// Create repositories
	productRepo := repositories.NewProductRepository(dbService, nil)
	movementRepo := repositories.NewMovementRepository(dbService, nil)
	stockRepo := repositories.NewStockRepository(dbService, nil)

	// Create services
	productService := inventoryServices.NewProductService(productRepo, nil)
	movementService := inventoryServices.NewMovementService(movementRepo, nil)
	stockService := inventoryServices.NewStockService(stockRepo)

	// Create controllers
	productController := controllers.NewProductController(productService)
	movementController := controllers.NewMovementController(movementService)
	stockController := controllers.NewStockController(stockService)

	// Public routes
	api := router.Group("/inventory")

	// Products
	api.POST("/products", productController.CreateProductWithVariants)
	api.GET("/products", productController.ListProducts)
	api.GET("/products/:id", productController.GetProduct)
	api.GET("/products/sku/:sku", productController.GetProductBySkU)

	// Movements
	api.POST("/movements", movementController.RecordMovement)
	api.GET("/movements/:id", movementController.GetMovement)
	api.GET("/stock/:product_id", movementController.GetProductStock)

	// Stock Management
	api.POST("/reserve", stockController.ReserveStock)
	api.POST("/release-reserved", stockController.ReleaseReservedStock)
	api.PATCH("/products/:product_id/reorder-level", stockController.UpdateReorderLevel)
}
