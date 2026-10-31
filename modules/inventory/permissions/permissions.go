// Package permissions defines all permission codes for the inventory module
// and registers them in the global tenancy permission registry.
package permissions

import (
	"josex/web/modules/tenancy/models"
	permRegistry "josex/web/modules/tenancy/permissions"
)

// Permission code constants — use these in RequirePermission() calls.
const (
	ProductsRead   = "inventory:products:read"
	ProductsWrite  = "inventory:products:write"
	ProductsDelete = "inventory:products:delete"

	CategoriesRead   = "inventory:categories:read"
	CategoriesManage = "inventory:categories:manage"

	StockRead    = "inventory:stock:read"
	StockAdjust  = "inventory:stock:adjust"
	StockReserve = "inventory:stock:reserve"

	BatchesRead   = "inventory:batches:read"
	BatchesManage = "inventory:batches:manage"

	WarehousesManage = "inventory:warehouses:manage"
	StockTransfer    = "inventory:stock:transfer"
	StockCount       = "inventory:stock:count"
)

func init() {
	permRegistry.RegisterSections("inventory", "products_stock")
	permRegistry.Register([]models.AvailablePermission{
		{Code: ProductsRead, Module: "inventory", Name: "permission.inventory.products-read", Description: "permission.inventory.products-read.description", Section: "products_stock", Access: models.AccessRead},
		{Code: ProductsWrite, Module: "inventory", Name: "permission.inventory.products-write", Description: "permission.inventory.products-write.description", Section: "products_stock", Access: models.AccessWrite},
		{Code: ProductsDelete, Module: "inventory", Name: "permission.inventory.products-delete", Description: "permission.inventory.products-delete.description", Section: "products_stock", Access: models.AccessWrite},
		{Code: CategoriesRead, Module: "inventory", Name: "permission.inventory.categories-read", Description: "permission.inventory.categories-read.description", Section: "products_stock", Access: models.AccessRead},
		{Code: CategoriesManage, Module: "inventory", Name: "permission.inventory.categories-manage", Description: "permission.inventory.categories-manage.description", Section: "products_stock", Access: models.AccessWrite},
		{Code: WarehousesManage, Module: "inventory", Name: "permission.inventory.warehouses-manage", Description: "permission.inventory.warehouses-manage.description", Section: "products_stock", Access: models.AccessWrite},
		{Code: StockTransfer, Module: "inventory", Name: "permission.inventory.stock-transfer", Description: "permission.inventory.stock-transfer.description", Section: "products_stock", Access: models.AccessWrite},
		{Code: StockCount, Module: "inventory", Name: "permission.inventory.stock-count", Description: "permission.inventory.stock-count.description", Section: "products_stock", Access: models.AccessWrite},
		{Code: StockRead, Module: "inventory", Name: "permission.inventory.stock-read", Description: "permission.inventory.stock-read.description", Section: "products_stock", Access: models.AccessRead},
		{Code: StockAdjust, Module: "inventory", Name: "permission.inventory.stock-adjust", Description: "permission.inventory.stock-adjust.description", Section: "products_stock", Access: models.AccessWrite},
		{Code: StockReserve, Module: "inventory", Name: "permission.inventory.stock-reserve", Description: "permission.inventory.stock-reserve.description", Section: "products_stock", Access: models.AccessWrite},
		{Code: BatchesRead, Module: "inventory", Name: "permission.inventory.batches-read", Description: "permission.inventory.batches-read.description", Section: "products_stock", Access: models.AccessRead},
		{Code: BatchesManage, Module: "inventory", Name: "permission.inventory.batches-manage", Description: "permission.inventory.batches-manage.description", Section: "products_stock", Access: models.AccessWrite},
	})
}
