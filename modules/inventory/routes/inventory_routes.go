package routes

import (
	appConfig "josex/web/config"
	"josex/web/modules/core/api"
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
	Unit     inventoryInterfaces.UnitRepository
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
		Unit:     repositories.NewUnitRepository(dbService),

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

	inventory := router.Group("/inventory", authMiddleware, tenantMiddleware)

	// Units, presentations and codes (INV-023).
	unitController := controllers.NewUnitController(services.Unit, services.Product)
	api.Get(inventory, "/units", unitController.ListUnits, api.Doc{Summary: "List the units", Description: "The standard list and the business's own"}, tenancyMW.RequirePermission(inventoryPerms.ProductsRead))
	api.Put(inventory, "/products/{id}/units", unitController.ReplaceProductUnits, api.Doc{Summary: "Replace a product's presentations", Description: "The whole list diffed by id, exactly one base at factor 1; a presentation a line names is deactivated, not deleted. Answers the product.", Errors: []int{400, 404, 409}}, tenancyMW.RequirePermission(inventoryPerms.ProductsWrite))
	api.Get(inventory, "/codes/{code}", unitController.FindByCode, api.Doc{Summary: "What a barcode names", Description: "Product, combination, presentation, factor and price", Errors: []int{404}}, tenancyMW.RequirePermission(inventoryPerms.ProductsRead))

	// Downloads, with the permission of the list each one reads (INV-021).
	exportController := controllers.NewExportController(services.Product, services.Movement, services.Batch)
	api.Get(inventory, "/export/products", exportController.ExportProducts, api.Doc{Summary: "Download products as CSV", Description: "UTF-8 with BOM, the list's filters, at most 50 000 rows", Errors: []int{400, 422}}, tenancyMW.RequirePermission(inventoryPerms.ProductsRead))
	api.Get(inventory, "/export/movements", exportController.ExportMovements, api.Doc{Summary: "Download movements as CSV", Description: "One line per movement, the ledger's filters, at most 50 000 rows", Errors: []int{400, 422}}, tenancyMW.RequirePermission(inventoryPerms.StockRead))
	api.Get(inventory, "/export/batches", exportController.ExportBatches, api.Doc{Summary: "Download lots as CSV", Description: "One product's lots, or those about to expire", Errors: []int{400, 422}}, tenancyMW.RequirePermission(inventoryPerms.BatchesRead))

	// Products
	api.Get(inventory, "/products", productController.ListProducts, api.Doc{Summary: "List all products", Description: "One page of products, newest first, each with its categories, stock, axes and price range; counts carry the chips. Narrowed by status (active by default), chip, category and a name/SKU search.", Errors: []int{400}}, tenancyMW.RequirePermission(inventoryPerms.ProductsRead))
	api.Get(inventory, "/products/{id}", productController.GetProduct, api.Doc{Summary: "Get product by ID", Description: "Retrieve a product with all its details", Errors: []int{404}}, tenancyMW.RequirePermission(inventoryPerms.ProductsRead))
	api.Get(inventory, "/products/sku/{sku}", productController.GetProductBySkU, api.Doc{Summary: "Get product by SKU", Description: "Retrieve a product by its SKU code", Errors: []int{404}}, tenancyMW.RequirePermission(inventoryPerms.ProductsRead))
	api.Post(inventory, "/products", productController.CreateProductWithVariants, api.Doc{Summary: "Create a product with variants", Description: "Create a new product with optional variants (groups and options)", Status: 201, Errors: []int{400, 409}}, tenancyMW.RequirePermission(inventoryPerms.ProductsWrite))
	api.Put(inventory, "/products/{id}", productController.UpdateProductWithVariants, api.Doc{Summary: "Update a product and its variants", Description: "Update name, description and base price, and diff the variant tree by id. Omitting \"variants\" leaves the tree untouched; {\"groups\": []} removes every group. The SKU is immutable.", Errors: []int{400, 404, 409}}, tenancyMW.RequirePermission(inventoryPerms.ProductsWrite))

	// /bulk first: it has no id to read as one.
	api.Patch(inventory, "/products/bulk", productController.BulkUpdateProducts, api.Doc{Summary: "One change to several products", Description: "Status, price or percent change, the one category replacing theirs, or the POS flag, on at most 100 products in one statement. Changing the status also needs inventory:products:delete.", Errors: []int{400, 403, 404}}, tenancyMW.RequirePermission(inventoryPerms.ProductsWrite))
	api.Patch(inventory, "/products/{id}/status", productController.SetProductStatus, api.Doc{Summary: "Discontinue or reactivate a product", Description: "A discontinued product keeps its movements, lots and sales and leaves the default list.", Errors: []int{400, 404}}, tenancyMW.RequirePermission(inventoryPerms.ProductsDelete))

	// Product SKUs (sellable combinations). Generation is an explicit call and
	// never a side-effect of saving the product (INV-008 D2).
	api.Get(inventory, "/products/{id}/skus", skuController.ListProductSkus, api.Doc{Summary: "List the SKUs of a product", Description: "One row per sellable combination — the default SKU first, then the generated ones — with its option labels, quantities and stock status", Errors: []int{400, 404}}, tenancyMW.RequirePermission(inventoryPerms.ProductsRead))
	api.Post(inventory, "/products/{id}/skus/generate", skuController.GenerateSkus, api.Doc{Summary: "Generate the combinations of a product", Description: "Creates one SKU per missing combination of the product's inventory axes. Re-runnable: existing combinations are left alone and the default SKU keeps its quantity.", Status: 201, Errors: []int{404, 409}}, tenancyMW.RequirePermission(inventoryPerms.ProductsWrite))
	api.Post(inventory, "/products/{id}/skus/redistribute", skuController.RedistributeStock, api.Doc{Summary: "Move the default SKU's stock onto the combinations", Description: "Atomic move of the whole unassigned bucket. The amounts must sum to exactly the default SKU's current quantity; anything else changes nothing and returns 400.", Errors: []int{400, 404, 409}}, tenancyMW.RequirePermission(inventoryPerms.StockAdjust))

	// Product media
	api.Post(inventory, "/products/{id}/media", productController.AddProductMedia, api.Doc{Summary: "Add media to a product or variant option", Status: 201}, tenancyMW.RequirePermission(inventoryPerms.ProductsWrite))
	api.Delete(inventory, "/products/{id}/media/{media_id}", productController.RemoveProductMedia, api.Doc{Summary: "Remove a media item from a product"}, tenancyMW.RequirePermission(inventoryPerms.ProductsDelete))

	// Movements
	api.Get(inventory, "/movements", movementController.ListMovements, api.Doc{Summary: "List inventory movements", Description: "Paginated movement history, optionally filtered by product, type and date range", Errors: []int{400}}, tenancyMW.RequirePermission(inventoryPerms.StockRead))
	api.Get(inventory, "/movements/{id}", movementController.GetMovement, api.Doc{Summary: "Get movement by ID", Description: "Retrieve a specific inventory movement", Errors: []int{404}}, tenancyMW.RequirePermission(inventoryPerms.StockRead))
	api.Get(inventory, "/stock/{product_id}", movementController.GetProductStock, api.Doc{Summary: "Get current product stock", Description: "Retrieve current stock level for a product", Errors: []int{404}}, tenancyMW.RequirePermission(inventoryPerms.StockRead))
	api.Post(inventory, "/movements", movementController.RecordMovement, api.Doc{Summary: "Record an inventory movement", Description: "Record a stock movement (purchase, sale, adjustment, etc.)", Status: 201, Errors: []int{400, 404, 409}}, tenancyMW.RequirePermission(inventoryPerms.StockAdjust))

	// Stock Management
	api.Post(inventory, "/reserve", stockController.ReserveStock, api.Doc{Summary: "Reserve stock for a sales order", Description: "Reserve inventory when a sales order is created", Errors: []int{400, 404, 409}}, tenancyMW.RequirePermission(inventoryPerms.StockReserve))
	api.Post(inventory, "/release-reserved", stockController.ReleaseReservedStock, api.Doc{Summary: "Release reserved stock", Description: "Release inventory when a sales order is cancelled", Errors: []int{400, 404}}, tenancyMW.RequirePermission(inventoryPerms.StockReserve))
	api.Patch(inventory, "/products/{id}/reorder-level", stockController.UpdateReorderLevel, api.Doc{Summary: "Update product reorder level", Description: "Update the minimum stock level for a product", Errors: []int{400, 404}}, tenancyMW.RequirePermission(inventoryPerms.StockAdjust))

	// Categories
	api.Get(inventory, "/categories/search", categoryController.SearchCategories, api.Doc{Summary: "Search product categories", Description: "Search categories by name, slug, or description"}, tenancyMW.RequirePermission(inventoryPerms.CategoriesRead))
	api.Get(inventory, "/categories/{id}/products", categoryController.GetProductsByCategory, api.Doc{Summary: "Get products in category", Description: "Retrieve all products assigned to a specific category", Errors: []int{404}}, tenancyMW.RequirePermission(inventoryPerms.CategoriesRead))
	api.Get(inventory, "/categories", categoryController.ListCategories, api.Doc{Summary: "List product categories", Description: "Retrieve a paginated list of product categories (optionally filtered by parent)"}, tenancyMW.RequirePermission(inventoryPerms.CategoriesRead))
	api.Get(inventory, "/categories/{id}", categoryController.GetCategory, api.Doc{Summary: "Get a product category", Description: "Retrieve a single product category by ID with product count", Errors: []int{404}}, tenancyMW.RequirePermission(inventoryPerms.CategoriesRead))
	api.Post(inventory, "/categories", categoryController.CreateCategory, api.Doc{Summary: "Create a new product category", Description: "Create a new product category with hierarchical support", Status: 201, Errors: []int{400, 409}}, tenancyMW.RequirePermission(inventoryPerms.CategoriesManage))
	api.Put(inventory, "/categories/{id}", categoryController.UpdateCategory, api.Doc{Summary: "Update a product category", Description: "Update an existing product category", Errors: []int{400, 404}}, tenancyMW.RequirePermission(inventoryPerms.CategoriesManage))
	api.Delete(inventory, "/categories/{id}", categoryController.DeleteCategory, api.Doc{Summary: "Delete a product category", Description: "Soft delete a product category (cannot have products assigned)", Errors: []int{400, 404}}, tenancyMW.RequirePermission(inventoryPerms.CategoriesManage))

	// Product-Category mapping
	api.Get(inventory, "/products/{id}/categories", categoryController.GetCategoriesByProduct, api.Doc{Summary: "Get categories of a product", Description: "Retrieve all categories a specific product is assigned to", Errors: []int{404}}, tenancyMW.RequirePermission(inventoryPerms.ProductsRead))
	api.Post(inventory, "/products/{id}/categories/{category_id}", categoryController.AssignProductToCategory, api.Doc{Summary: "Assign product to category", Description: "Assign a product to a category", Status: 201, Errors: []int{400, 404, 409}}, tenancyMW.RequirePermission(inventoryPerms.CategoriesManage))
	api.Delete(inventory, "/products/{id}/categories/{category_id}", categoryController.RemoveProductFromCategory, api.Doc{Summary: "Remove product from category", Description: "Remove a product from a category", Errors: []int{400, 404}}, tenancyMW.RequirePermission(inventoryPerms.CategoriesManage))

	// Batches
	api.Get(inventory, "/batches/{id}", batchController.GetBatch, api.Doc{Summary: "Get batch details", Description: "Get detailed information about a specific batch", Errors: []int{404}}, tenancyMW.RequirePermission(inventoryPerms.BatchesRead))
	api.Get(inventory, "/batches/product/{productId}", batchController.ListBatchesByProduct, api.Doc{Summary: "List all batches for a product", Description: "One page of a product's batches, ordered by expiry date (FIFO)", Errors: []int{400}}, tenancyMW.RequirePermission(inventoryPerms.BatchesRead))
	api.Get(inventory, "/batches/product/{productId}/oldest", batchController.GetOldestBatchForSale, api.Doc{Summary: "Get oldest batch for sale (FIFO)", Description: "Get the oldest active batch for a product, used for FIFO sales", Errors: []int{404}}, tenancyMW.RequirePermission(inventoryPerms.BatchesRead))
	api.Get(inventory, "/batches/expiring", batchController.GetExpiringBatches, api.Doc{Summary: "Get all batches expiring soon", Description: "One page of the batches that will expire within the warning window, soonest first", Errors: []int{400}}, tenancyMW.RequirePermission(inventoryPerms.BatchesRead))
	api.Post(inventory, "/batches", batchController.CreateBatch, api.Doc{Summary: "Create a new batch for a product", Description: "Create a new inventory batch with lot number and expiry date", Status: 201, Errors: []int{400, 404}}, tenancyMW.RequirePermission(inventoryPerms.BatchesManage))
	api.Patch(inventory, "/batches/{id}", batchController.UpdateBatch, api.Doc{Summary: "Correct a lot", Description: "Correct a lot's lot number and expiry date, while it has no movements", Errors: []int{400, 404, 409}}, tenancyMW.RequirePermission(inventoryPerms.BatchesManage))
	// DELETE voids the lot (status = 'void'); the row is never removed.
	api.Delete(inventory, "/batches/{id}", batchController.VoidBatch, api.Doc{Summary: "Void a lot", Description: "Write a lot off by setting its status to void. Never deletes the row.", Errors: []int{404, 409}}, tenancyMW.RequirePermission(inventoryPerms.BatchesManage))
	// D2 — splits a lot stranded in the unassigned bucket onto real combinations.
	api.Post(inventory, "/products/{id}/batches/redistribute", batchController.RedistributeBatches, api.Doc{Summary: "Split a stranded lot onto real combinations", Description: "Split a default-SKU lot into per-combination lots keeping its lot number, dates and unit cost", Errors: []int{400, 404, 409}}, tenancyMW.RequirePermission(inventoryPerms.BatchesManage))
}
