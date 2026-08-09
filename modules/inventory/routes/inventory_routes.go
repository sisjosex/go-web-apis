package routes

import (
	coreServices "josex/web/modules/core/services"
	"josex/web/modules/inventory/controllers"
	inventoryPerms "josex/web/modules/inventory/permissions"
	"josex/web/modules/inventory/repositories"
	inventoryServices "josex/web/modules/inventory/services"
	tenancyMW "josex/web/modules/tenancy/middleware"

	"github.com/gin-gonic/gin"
)

func RegisterInventoryRoutes(
	router *gin.RouterGroup,
	dbService coreServices.DatabaseService,
	authMiddleware gin.HandlerFunc,
	tenantMiddleware gin.HandlerFunc,
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

	api := router.Group("/inventory", authMiddleware, tenantMiddleware)

	// Products
	api.GET("/products", productController.ListProducts)
	api.GET("/products/:id", productController.GetProduct)
	api.GET("/products/sku/:sku", productController.GetProductBySkU)
	api.POST("/products",
		tenancyMW.RequirePermission(inventoryPerms.ProductsWrite),
		productController.CreateProductWithVariants)
	api.PUT("/products/:id",
		tenancyMW.RequirePermission(inventoryPerms.ProductsWrite),
		productController.UpdateProductWithVariants)

	// Product media
	api.POST("/products/:id/media",
		tenancyMW.RequirePermission(inventoryPerms.ProductsWrite),
		productController.AddProductMedia)
	api.DELETE("/products/:id/media/:media_id",
		tenancyMW.RequirePermission(inventoryPerms.ProductsDelete),
		productController.RemoveProductMedia)

	// Movements
	api.GET("/movements/:id", movementController.GetMovement)
	api.GET("/stock/:product_id", movementController.GetProductStock)
	api.POST("/movements",
		tenancyMW.RequirePermission(inventoryPerms.StockAdjust),
		movementController.RecordMovement)

	// Stock Management
	api.POST("/reserve",
		tenancyMW.RequirePermission(inventoryPerms.StockReserve),
		stockController.ReserveStock)
	api.POST("/release-reserved",
		tenancyMW.RequirePermission(inventoryPerms.StockReserve),
		stockController.ReleaseReservedStock)
	api.PATCH("/products/:id/reorder-level",
		tenancyMW.RequirePermission(inventoryPerms.StockAdjust),
		stockController.UpdateReorderLevel)

	// Categories
	api.GET("/categories/search", categoryController.SearchCategories)
	api.GET("/categories/:id/products", categoryController.GetProductsByCategory)
	api.GET("/categories", categoryController.ListCategories)
	api.GET("/categories/:id", categoryController.GetCategory)
	api.POST("/categories",
		tenancyMW.RequirePermission(inventoryPerms.CategoriesManage),
		categoryController.CreateCategory)
	api.PUT("/categories/:id",
		tenancyMW.RequirePermission(inventoryPerms.CategoriesManage),
		categoryController.UpdateCategory)
	api.DELETE("/categories/:id",
		tenancyMW.RequirePermission(inventoryPerms.CategoriesManage),
		categoryController.DeleteCategory)

	// Product-Category mapping
	api.GET("/products/:id/categories", categoryController.GetCategoriesByProduct)
	api.POST("/products/:id/categories/:category_id",
		tenancyMW.RequirePermission(inventoryPerms.CategoriesManage),
		categoryController.AssignProductToCategory)
	api.DELETE("/products/:id/categories/:category_id",
		tenancyMW.RequirePermission(inventoryPerms.CategoriesManage),
		categoryController.RemoveProductFromCategory)

	// Batches
	api.GET("/batches/:id", batchController.GetBatch)
	api.GET("/batches/product/:productId", batchController.ListBatchesByProduct)
	api.GET("/batches/product/:productId/oldest", batchController.GetOldestBatchForSale)
	api.GET("/batches/expiring", batchController.GetExpiringBatches)
	api.POST("/batches",
		tenancyMW.RequirePermission(inventoryPerms.BatchesManage),
		batchController.CreateBatch)
}
