// Package permissions defines all permission codes for the purchasing module
// and registers them in the global tenancy permission registry.
package permissions

import (
	"josex/web/modules/tenancy/models"
	permRegistry "josex/web/modules/tenancy/permissions"
)

// Permission code constants — use these in RequirePermission() calls.
const (
	SuppliersRead  = "purchasing:suppliers:read"
	SuppliersWrite = "purchasing:suppliers:write"

	OrdersRead    = "purchasing:orders:read"
	OrdersCreate  = "purchasing:orders:create"
	OrdersApprove = "purchasing:orders:approve"
	OrdersReceive = "purchasing:orders:receive"

	InvoicesManage = "purchasing:invoices:manage"

	RFQRead   = "purchasing:rfq:read"
	RFQManage = "purchasing:rfq:manage"

	BatchesRead   = "purchasing:batches:read"
	BatchesCreate = "purchasing:batches:create"

	ReportsRead = "purchasing:reports:read"
)

func init() {
	permRegistry.RegisterSections("purchasing", "purchases")
	permRegistry.Register([]models.AvailablePermission{
		{Code: SuppliersRead, Module: "purchasing", Name: "permission.purchasing.suppliers-read", Description: "permission.purchasing.suppliers-read.description", Section: "purchases", Access: models.AccessRead},
		{Code: SuppliersWrite, Module: "purchasing", Name: "permission.purchasing.suppliers-write", Description: "permission.purchasing.suppliers-write.description", Section: "purchases", Access: models.AccessWrite},
		{Code: OrdersRead, Module: "purchasing", Name: "permission.purchasing.orders-read", Description: "permission.purchasing.orders-read.description", Section: "purchases", Access: models.AccessRead},
		{Code: OrdersCreate, Module: "purchasing", Name: "permission.purchasing.orders-create", Description: "permission.purchasing.orders-create.description", Section: "purchases", Access: models.AccessWrite},
		{Code: OrdersApprove, Module: "purchasing", Name: "permission.purchasing.orders-approve", Description: "permission.purchasing.orders-approve.description", Section: "purchases", Access: models.AccessWrite},
		{Code: OrdersReceive, Module: "purchasing", Name: "permission.purchasing.orders-receive", Description: "permission.purchasing.orders-receive.description", Section: "purchases", Access: models.AccessWrite},
		{Code: InvoicesManage, Module: "purchasing", Name: "permission.purchasing.invoices-manage", Description: "permission.purchasing.invoices-manage.description", Section: "purchases", Access: models.AccessWrite},
		{Code: RFQRead, Module: "purchasing", Name: "permission.purchasing.rfq-read", Description: "permission.purchasing.rfq-read.description", Section: "purchases", Access: models.AccessRead},
		{Code: RFQManage, Module: "purchasing", Name: "permission.purchasing.rfq-manage", Description: "permission.purchasing.rfq-manage.description", Section: "purchases", Access: models.AccessWrite},
		{Code: BatchesRead, Module: "purchasing", Name: "permission.purchasing.batches-read", Description: "permission.purchasing.batches-read.description", Section: "purchases", Access: models.AccessRead},
		{Code: BatchesCreate, Module: "purchasing", Name: "permission.purchasing.batches-create", Description: "permission.purchasing.batches-create.description", Section: "purchases", Access: models.AccessWrite},
		{Code: ReportsRead, Module: "purchasing", Name: "permission.purchasing.reports-read", Description: "permission.purchasing.reports-read.description", Section: "purchases", Access: models.AccessRead},
	})
}
