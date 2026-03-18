package routes

import (
	"josex/web/modules/sales/controllers"

	"github.com/gin-gonic/gin"
)

// SetupSalesRoutes registers all sales module routes
func SetupSalesRoutes(apiV1 *gin.RouterGroup, customerCtrl *controllers.CustomerController, orderCtrl *controllers.SalesOrderController, authMiddleware gin.HandlerFunc) {
	// Customer routes
	customerGroup := apiV1.Group("/sales/customers", authMiddleware)
	{
		customerGroup.POST("", customerCtrl.CreateCustomer)
		customerGroup.GET("/:id", customerCtrl.GetCustomer)
		customerGroup.PATCH("/:id", customerCtrl.UpdateCustomer)
		customerGroup.DELETE("/:id", customerCtrl.DeleteCustomer)
	}

	// Sales Order routes
	orderGroup := apiV1.Group("/sales/orders", authMiddleware)
	{
		orderGroup.POST("", orderCtrl.CreateSalesOrder)
		orderGroup.GET("", orderCtrl.ListSalesOrders)
		orderGroup.GET("/:id", orderCtrl.GetSalesOrder)
		orderGroup.PATCH("/:id", orderCtrl.UpdateSalesOrder)

		// Phase 1: Batch Assignment & Order Completion
		orderGroup.POST("/:id/items", orderCtrl.AddOrderItem)
		orderGroup.PATCH("/:id/complete", orderCtrl.CompleteOrder)

		// Phase 2: Reporting & Cancellation
		orderGroup.PATCH("/:id/cancel", orderCtrl.CancelOrder)
		orderGroup.GET("/:id/with-batches", orderCtrl.GetOrderWithBatches)
	}

	// Sales Reports routes
	reportsGroup := apiV1.Group("/sales/reports", authMiddleware)
	{
		// Phase 2: Sales Report
		reportsGroup.GET("/sales", orderCtrl.GetSalesReport)
	}

	// Returns routes
	returnsGroup := apiV1.Group("/sales/returns", authMiddleware)
	{
		// Phase 3: Returns Management
		returnsGroup.POST("", orderCtrl.CreateReturn)
		returnsGroup.GET("", orderCtrl.GetReturns)
		returnsGroup.PATCH("/:id/approve", orderCtrl.ApproveReturn)
	}

	// Payments routes
	paymentsGroup := apiV1.Group("/sales/payments", authMiddleware)
	{
		// Phase 3: Payments Management
		paymentsGroup.POST("", orderCtrl.CreatePayment)
		paymentsGroup.GET("", orderCtrl.GetPayments)
	}
}
