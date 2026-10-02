package routes

import (
	appConfig "josex/web/config"
	coreServices "josex/web/modules/core/services"
	"josex/web/modules/inventory/controllers"
	inventoryInterfaces "josex/web/modules/inventory/interfaces"
	inventoryPerms "josex/web/modules/inventory/permissions"
	"josex/web/modules/inventory/repositories"
	inventoryServices "josex/web/modules/inventory/services"
	tenancyMW "josex/web/modules/tenancy/middleware"

	"github.com/gin-gonic/gin"
)

// InventoryServices is the module's service layer, built once and handed back so
// callers outside the module can reach it. The CSV import descriptors live in
// routes.go (INV-015 D1) and every one of them is a wrapper over a service that
// used to be created inside RegisterInventoryRoutes and unreachable from there.
type InventoryServices struct {
	Product  inventoryInterfaces.ProductService
	Category inventoryInterfaces.CategoryService
	Movement inventoryInterfaces.MovementService
	Stock    inventoryInterfaces.StockService
	Sku      inventoryInterfaces.SkuService
	Batch    inventoryInterfaces.BatchService
	// ImportLookup is the read-only repository the import descriptors resolve
	// and preview rows through (INV-017 D1); no route of this module uses it.
	ImportLookup inventoryInterfaces.ImportLookupRepository
}

// NewInventoryServices wires the module's repositories and services over one
// database service.
// media is where the product images live, so removing a media row removes its image (INFRA-007).
func NewInventoryServices(dbService coreServices.DatabaseService, media coreServices.MediaService) InventoryServices {
	productRepo := repositories.NewProductRepository(dbService, nil)
	movementRepo := repositories.NewMovementRepository(dbService, nil)
	stockRepo := repositories.NewStockRepository(dbService, nil)
	categoryRepo := repositories.NewCategoryRepository(dbService, nil)
	skuRepo := repositories.NewSkuRepository(dbService, nil)
	batchRepo := repositories.NewBatchRepository(dbService)

	return InventoryServices{
		Product:  inventoryServices.NewProductService(productRepo, appConfig.GetConfig().Inventory, nil, media),
		Category: inventoryServices.NewCategoryService(categoryRepo, nil),
		Movement: inventoryServices.NewMovementService(movementRepo, nil),
		Stock:    inventoryServices.NewStockService(stockRepo),
		Sku:      inventoryServices.NewSkuService(skuRepo, appConfig.GetConfig().Inventory, nil),
		Batch:    inventoryServices.NewBatchService(batchRepo),

		ImportLookup: repositories.NewImportLookupRepository(dbService),
	}
}

func RegisterInventoryRoutes(
	router *gin.RouterGroup,
	services InventoryServices,
	authMiddleware gin.HandlerFunc,
	tenantMiddleware gin.HandlerFunc,
) {
	// Create controllers
	productController := controllers.NewProductController(services.Product)
	movementController := controllers.NewMovementController(services.Movement)
	stockController := controllers.NewStockController(services.Stock)
	categoryController := controllers.NewCategoryController(services.Category)
	skuController := controllers.NewSkuController(services.Sku)
	batchController := controllers.NewBatchController(services.Batch)

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

	// Product SKUs (sellable combinations). Generation is an explicit call and
	// never a side-effect of saving the product (INV-008 D2).
	api.GET("/products/:id/skus", skuController.ListProductSkus)
	api.POST("/products/:id/skus/generate",
		tenancyMW.RequirePermission(inventoryPerms.ProductsWrite),
		skuController.GenerateSkus)
	api.POST("/products/:id/skus/redistribute",
		tenancyMW.RequirePermission(inventoryPerms.StockAdjust),
		skuController.RedistributeStock)

	// Product media
	api.POST("/products/:id/media",
		tenancyMW.RequirePermission(inventoryPerms.ProductsWrite),
		productController.AddProductMedia)
	api.DELETE("/products/:id/media/:media_id",
		tenancyMW.RequirePermission(inventoryPerms.ProductsDelete),
		productController.RemoveProductMedia)

	// Movements
	api.GET("/movements", movementController.ListMovements)
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
	api.PATCH("/batches/:id",
		tenancyMW.RequirePermission(inventoryPerms.BatchesManage),
		batchController.UpdateBatch)
	// DELETE voids the lot (status = 'void'); the row is never removed.
	api.DELETE("/batches/:id",
		tenancyMW.RequirePermission(inventoryPerms.BatchesManage),
		batchController.VoidBatch)
	// D2 — splits a lot stranded in the unassigned bucket onto real combinations.
	api.POST("/products/:id/batches/redistribute",
		tenancyMW.RequirePermission(inventoryPerms.BatchesManage),
		batchController.RedistributeBatches)
}
