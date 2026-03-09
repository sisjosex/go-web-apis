package routes

import (
	"josex/web/modules/purchasing/controllers"
	"josex/web/modules/purchasing/interfaces"

	"github.com/gin-gonic/gin"
)

// RegisterPurchasingRoutes registers all purchasing routes
func RegisterPurchasingRoutes(router *gin.Engine, authMiddleware gin.HandlerFunc, service interfaces.PurchasingService) {
	ctrl := controllers.NewPurchasingController(service)

	// All purchasing routes require authentication
	routeGroup := router.Group("/api/v1/purchasing")
	routeGroup.Use(authMiddleware)

	// Supplier routes
	routeGroup.POST("/suppliers", ctrl.CreateSupplier)
	routeGroup.GET("/suppliers", ctrl.ListSuppliers)
	routeGroup.GET("/suppliers/:id", ctrl.GetSupplier)

	// Purchase Order routes
	routeGroup.POST("/purchase-orders", ctrl.CreatePurchaseOrder)
	routeGroup.GET("/purchase-orders", ctrl.ListPurchaseOrders)
	routeGroup.GET("/purchase-orders/:id", ctrl.GetPurchaseOrder)
	routeGroup.PATCH("/purchase-orders/:id/approve", ctrl.ApprovePurchaseOrder)
	routeGroup.PATCH("/purchase-orders/:id/receive", ctrl.ReceivePurchaseOrder)

	// Purchase Order Items
	routeGroup.POST("/purchase-orders/:id/items", ctrl.AddPurchaseOrderItem)

	// Purchase Order Invoices
	routeGroup.POST("/purchase-orders/:id/invoices", ctrl.AddInvoice)

	// Reports
	routeGroup.GET("/pending-payments", ctrl.GetPendingPayments)

	// Phase 2A: Price Comparison
	routeGroup.GET("/price-comparison", ctrl.GetPriceComparison)

	// Phase 2A: FIFO Batch Tracking
	routeGroup.POST("/batches", ctrl.CreateProductBatch)
	routeGroup.GET("/batches", ctrl.GetProductBatches)
	routeGroup.GET("/batches/oldest", ctrl.GetOldestBatchForSale)

	// Phase 2A: Request for Quote (RFQ)
	routeGroup.POST("/rfq", ctrl.CreateRFQ)
	routeGroup.GET("/rfq/:rfq_id/responses", ctrl.GetRFQResponses)
	routeGroup.GET("/rfq/:rfq_id/comparison", ctrl.GetRFQComparison)
	routeGroup.PATCH("/rfq/:rfq_id/responses/:response_id/select", ctrl.SelectRFQResponse)
}
