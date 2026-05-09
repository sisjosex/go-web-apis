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
	permRegistry.Register([]models.AvailablePermission{
		{Code: CustomersRead, Module: "sales", Name: "permission.sales.customers-read", Description: "permission.sales.customers-read.description"},
		{Code: CustomersWrite, Module: "sales", Name: "permission.sales.customers-write", Description: "permission.sales.customers-write.description"},
		{Code: CustomersDelete, Module: "sales", Name: "permission.sales.customers-delete", Description: "permission.sales.customers-delete.description"},
		{Code: OrdersRead, Module: "sales", Name: "permission.sales.orders-read", Description: "permission.sales.orders-read.description"},
		{Code: OrdersCreate, Module: "sales", Name: "permission.sales.orders-create", Description: "permission.sales.orders-create.description"},
		{Code: OrdersManage, Module: "sales", Name: "permission.sales.orders-manage", Description: "permission.sales.orders-manage.description"},
		{Code: ReturnsRead, Module: "sales", Name: "permission.sales.returns-read", Description: "permission.sales.returns-read.description"},
		{Code: ReturnsManage, Module: "sales", Name: "permission.sales.returns-manage", Description: "permission.sales.returns-manage.description"},
		{Code: PaymentsRead, Module: "sales", Name: "permission.sales.payments-read", Description: "permission.sales.payments-read.description"},
		{Code: PaymentsCreate, Module: "sales", Name: "permission.sales.payments-create", Description: "permission.sales.payments-create.description"},
		{Code: ReportsRead, Module: "sales", Name: "permission.sales.reports-read", Description: "permission.sales.reports-read.description"},
	})
}
