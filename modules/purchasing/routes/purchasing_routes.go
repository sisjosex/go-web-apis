package routes

import (
	"josex/web/modules/core/api"
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
	api.Get(r, "/suppliers", ctrl.ListSuppliers, api.Doc{}, tenancyMW.RequirePermission(purchasingPerms.SuppliersRead))
	api.Get(r, "/suppliers/{id}", ctrl.GetSupplier, api.Doc{}, tenancyMW.RequirePermission(purchasingPerms.SuppliersRead))
	api.Patch(r, "/suppliers/{id}", ctrl.UpdateSupplier, api.Doc{Summary: "Change a supplier", Description: "The fields sent; is_active reactivates", Errors: []int{400, 404}}, tenancyMW.RequirePermission(purchasingPerms.SuppliersWrite))
	api.Delete(r, "/suppliers/{id}", ctrl.DeactivateSupplier, api.Doc{Summary: "Deactivate a supplier", Description: "It leaves the default list; its orders keep reading it", Status: 204, Errors: []int{404}}, tenancyMW.RequirePermission(purchasingPerms.SuppliersWrite))
	api.Post(r, "/suppliers", ctrl.CreateSupplier, api.Doc{Status: 201}, tenancyMW.RequirePermission(purchasingPerms.SuppliersWrite))

	// Purchase Orders
	api.Get(r, "/purchase-orders", ctrl.ListPurchaseOrders, api.Doc{})
	api.Get(r, "/purchase-orders/{id}", ctrl.GetPurchaseOrder, api.Doc{})
	api.Post(r, "/purchase-orders", ctrl.CreatePurchaseOrder, api.Doc{Status: 201}, tenancyMW.RequirePermission(purchasingPerms.OrdersCreate))
	api.Patch(r, "/purchase-orders/{id}/approve", ctrl.ApprovePurchaseOrder, api.Doc{}, tenancyMW.RequirePermission(purchasingPerms.OrdersApprove))
	api.Patch(r, "/purchase-orders/{id}/receive", ctrl.ReceivePurchaseOrder, api.Doc{}, tenancyMW.RequirePermission(purchasingPerms.OrdersReceive))

	// Purchase Order Items
	api.Post(r, "/purchase-orders/{id}/items", ctrl.AddPurchaseOrderItem, api.Doc{Status: 201}, tenancyMW.RequirePermission(purchasingPerms.OrdersCreate))

	// Purchase Order Invoices
	api.Post(r, "/purchase-orders/{id}/invoices", ctrl.AddInvoice, api.Doc{Status: 201}, tenancyMW.RequirePermission(purchasingPerms.InvoicesManage))

	// Reports
	api.Get(r, "/pending-payments", ctrl.GetPendingPayments, api.Doc{}, tenancyMW.RequirePermission(purchasingPerms.ReportsRead))
	api.Get(r, "/price-comparison", ctrl.GetPriceComparison, api.Doc{}, tenancyMW.RequirePermission(purchasingPerms.ReportsRead))

	// FIFO Batches
	api.Get(r, "/batches", ctrl.GetProductBatches, api.Doc{}, tenancyMW.RequirePermission(purchasingPerms.BatchesRead))
	api.Get(r, "/batches/oldest", ctrl.GetOldestBatchForSale, api.Doc{}, tenancyMW.RequirePermission(purchasingPerms.BatchesRead))
	api.Post(r, "/batches", ctrl.CreateProductBatch, api.Doc{Status: 201}, tenancyMW.RequirePermission(purchasingPerms.BatchesCreate))

	// RFQ
	api.Get(r, "/rfq/{rfq_id}/responses", ctrl.GetRFQResponses, api.Doc{}, tenancyMW.RequirePermission(purchasingPerms.RFQRead))
	api.Get(r, "/rfq/{rfq_id}/comparison", ctrl.GetRFQComparison, api.Doc{}, tenancyMW.RequirePermission(purchasingPerms.RFQRead))
	api.Post(r, "/rfq", ctrl.CreateRFQ, api.Doc{Status: 201}, tenancyMW.RequirePermission(purchasingPerms.RFQManage))
	api.Patch(r, "/rfq/{rfq_id}/responses/{response_id}/select", ctrl.SelectRFQResponse, api.Doc{}, tenancyMW.RequirePermission(purchasingPerms.RFQManage))
}
