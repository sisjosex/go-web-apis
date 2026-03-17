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
	categoryRepo := repositories.NewCategoryRepository(dbService, nil)

	// Create services
	productService := inventoryServices.NewProductService(productRepo, nil)
	movementService := inventoryServices.NewMovementService(movementRepo, nil)
	stockService := inventoryServices.NewStockService(stockRepo)
	categoryService := inventoryServices.NewCategoryService(categoryRepo, nil)

	// Create controllers
	productController := controllers.NewProductController(productService)
	movementController := controllers.NewMovementController(movementService)
	stockController := controllers.NewStockController(stockService)
	categoryController := controllers.NewCategoryController(categoryService)
	batchRepository := repositories.NewBatchRepository(dbService)
	batchService := inventoryServices.NewBatchService(batchRepository)
	batchController := controllers.NewBatchController(batchService)

	// Public routes (no auth required for reads)
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

	// Categories
	api.GET("/categories/search", categoryController.SearchCategories)
	api.GET("/categories/:id/products", categoryController.GetProductsByCategory)
	api.POST("/categories", categoryController.CreateCategory)
	api.GET("/categories", categoryController.ListCategories)
	api.GET("/categories/:id", categoryController.GetCategory)
	api.PUT("/categories/:id", categoryController.UpdateCategory)
	api.DELETE("/categories/:id", categoryController.DeleteCategory)

	// Product-Category mapping
	api.POST("/products/:productId/categories/:id", categoryController.AssignProductToCategory)
	api.DELETE("/products/:productId/categories/:id", categoryController.RemoveProductFromCategory)

	// Batches routes (integrated into inventory)
	// Public batch routes
	api.POST("/batches", batchController.CreateBatch)
	api.GET("/batches/:id", batchController.GetBatch)
	api.GET("/batches/product/:productId", batchController.ListBatchesByProduct)
	api.GET("/batches/product/:productId/oldest", batchController.GetOldestBatchForSale)
	api.GET("/batches/expiring", batchController.GetExpiringBatches)
}
