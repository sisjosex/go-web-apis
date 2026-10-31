// Package permissions defines all permission codes for the sales module
// and registers them in the global tenancy permission registry.
package permissions

import (
	"josex/web/modules/tenancy/models"
	permRegistry "josex/web/modules/tenancy/permissions"
)

// Permission code constants — use these in RequirePermission() calls.
const (
	CustomersRead   = "sales:customers:read"
	CustomersWrite  = "sales:customers:write"
	CustomersDelete = "sales:customers:delete"

	OrdersRead   = "sales:orders:read"
	OrdersCreate = "sales:orders:create"
	OrdersManage = "sales:orders:manage"

	ReturnsRead   = "sales:returns:read"
	ReturnsManage = "sales:returns:manage"

	PaymentsRead   = "sales:payments:read"
	PaymentsCreate = "sales:payments:create"

	ReportsRead = "sales:reports:read"
)

func init() {
	permRegistry.RegisterSections("sales", "sales")
	permRegistry.Register([]models.AvailablePermission{
		{Code: CustomersRead, Module: "sales", Name: "permission.sales.customers-read", Description: "permission.sales.customers-read.description", Section: "sales", Access: models.AccessRead},
		{Code: CustomersWrite, Module: "sales", Name: "permission.sales.customers-write", Description: "permission.sales.customers-write.description", Section: "sales", Access: models.AccessWrite},
		{Code: CustomersDelete, Module: "sales", Name: "permission.sales.customers-delete", Description: "permission.sales.customers-delete.description", Section: "sales", Access: models.AccessWrite},
		{Code: OrdersRead, Module: "sales", Name: "permission.sales.orders-read", Description: "permission.sales.orders-read.description", Section: "sales", Access: models.AccessRead},
		{Code: OrdersCreate, Module: "sales", Name: "permission.sales.orders-create", Description: "permission.sales.orders-create.description", Section: "sales", Access: models.AccessWrite},
		{Code: OrdersManage, Module: "sales", Name: "permission.sales.orders-manage", Description: "permission.sales.orders-manage.description", Section: "sales", Access: models.AccessWrite},
		{Code: ReturnsRead, Module: "sales", Name: "permission.sales.returns-read", Description: "permission.sales.returns-read.description", Section: "sales", Access: models.AccessRead},
		{Code: ReturnsManage, Module: "sales", Name: "permission.sales.returns-manage", Description: "permission.sales.returns-manage.description", Section: "sales", Access: models.AccessWrite},
		{Code: PaymentsRead, Module: "sales", Name: "permission.sales.payments-read", Description: "permission.sales.payments-read.description", Section: "sales", Access: models.AccessRead},
		{Code: PaymentsCreate, Module: "sales", Name: "permission.sales.payments-create", Description: "permission.sales.payments-create.description", Section: "sales", Access: models.AccessWrite},
		{Code: ReportsRead, Module: "sales", Name: "permission.sales.reports-read", Description: "permission.sales.reports-read.description", Section: "sales", Access: models.AccessRead},
	})
}
