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
)

func init() {
	permRegistry.Register([]models.AvailablePermission{
		{Code: ProductsRead, Module: "inventory", Name: "permission.inventory.products-read", Description: "permission.inventory.products-read.description"},
		{Code: ProductsWrite, Module: "inventory", Name: "permission.inventory.products-write", Description: "permission.inventory.products-write.description"},
		{Code: ProductsDelete, Module: "inventory", Name: "permission.inventory.products-delete", Description: "permission.inventory.products-delete.description"},
		{Code: CategoriesRead, Module: "inventory", Name: "permission.inventory.categories-read", Description: "permission.inventory.categories-read.description"},
		{Code: CategoriesManage, Module: "inventory", Name: "permission.inventory.categories-manage", Description: "permission.inventory.categories-manage.description"},
		{Code: StockRead, Module: "inventory", Name: "permission.inventory.stock-read", Description: "permission.inventory.stock-read.description"},
		{Code: StockAdjust, Module: "inventory", Name: "permission.inventory.stock-adjust", Description: "permission.inventory.stock-adjust.description"},
		{Code: StockReserve, Module: "inventory", Name: "permission.inventory.stock-reserve", Description: "permission.inventory.stock-reserve.description"},
		{Code: BatchesRead, Module: "inventory", Name: "permission.inventory.batches-read", Description: "permission.inventory.batches-read.description"},
		{Code: BatchesManage, Module: "inventory", Name: "permission.inventory.batches-manage", Description: "permission.inventory.batches-manage.description"},
	})
}
