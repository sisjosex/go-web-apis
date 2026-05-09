package routes

import (
	"josex/web/modules/purchasing/controllers"
	"josex/web/modules/purchasing/interfaces"
	purchasingPerms "josex/web/modules/purchasing/permissions"
	tenancyMW "josex/web/modules/tenancy/middleware"

	"github.com/gin-gonic/gin"
)

// RegisterPurchasingRoutes registers all purchasing routes with auth, tenant, and permission middleware.
func RegisterPurchasingRoutes(
	apiV1 *gin.RouterGroup,
	service interfaces.PurchasingService,
	authMiddleware gin.HandlerFunc,
	tenantMiddleware gin.HandlerFunc,
) {
	ctrl := controllers.NewPurchasingController(service)

	r := apiV1.Group("/purchasing", authMiddleware, tenantMiddleware)

	// Suppliers
	r.GET("/suppliers", ctrl.ListSuppliers)
	r.GET("/suppliers/:id", ctrl.GetSupplier)
	r.POST("/suppliers",
		tenancyMW.RequirePermission(purchasingPerms.SuppliersWrite),
		ctrl.CreateSupplier)

	// Purchase Orders
	r.GET("/purchase-orders", ctrl.ListPurchaseOrders)
	r.GET("/purchase-orders/:id", ctrl.GetPurchaseOrder)
	r.POST("/purchase-orders",
		tenancyMW.RequirePermission(purchasingPerms.OrdersCreate),
		ctrl.CreatePurchaseOrder)
	r.PATCH("/purchase-orders/:id/approve",
		tenancyMW.RequirePermission(purchasingPerms.OrdersApprove),
		ctrl.ApprovePurchaseOrder)
	r.PATCH("/purchase-orders/:id/receive",
		tenancyMW.RequirePermission(purchasingPerms.OrdersReceive),
		ctrl.ReceivePurchaseOrder)

	// Purchase Order Items
	r.POST("/purchase-orders/:id/items",
		tenancyMW.RequirePermission(purchasingPerms.OrdersCreate),
		ctrl.AddPurchaseOrderItem)

	// Purchase Order Invoices
	r.POST("/purchase-orders/:id/invoices",
		tenancyMW.RequirePermission(purchasingPerms.InvoicesManage),
		ctrl.AddInvoice)

	// Reports
	r.GET("/pending-payments",
		tenancyMW.RequirePermission(purchasingPerms.ReportsRead),
		ctrl.GetPendingPayments)
	r.GET("/price-comparison",
		tenancyMW.RequirePermission(purchasingPerms.ReportsRead),
		ctrl.GetPriceComparison)

	// FIFO Batches
	r.GET("/batches",
		tenancyMW.RequirePermission(purchasingPerms.BatchesRead),
		ctrl.GetProductBatches)
	r.GET("/batches/oldest",
		tenancyMW.RequirePermission(purchasingPerms.BatchesRead),
		ctrl.GetOldestBatchForSale)
	r.POST("/batches",
		tenancyMW.RequirePermission(purchasingPerms.BatchesCreate),
		ctrl.CreateProductBatch)

	// RFQ
	r.GET("/rfq/:rfq_id/responses",
		tenancyMW.RequirePermission(purchasingPerms.RFQRead),
		ctrl.GetRFQResponses)
	r.GET("/rfq/:rfq_id/comparison",
		tenancyMW.RequirePermission(purchasingPerms.RFQRead),
		ctrl.GetRFQComparison)
	r.POST("/rfq",
		tenancyMW.RequirePermission(purchasingPerms.RFQManage),
		ctrl.CreateRFQ)
	r.PATCH("/rfq/:rfq_id/responses/:response_id/select",
		tenancyMW.RequirePermission(purchasingPerms.RFQManage),
		ctrl.SelectRFQResponse)
}
