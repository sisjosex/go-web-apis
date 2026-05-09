package routes

import (
	"josex/web/modules/sales/controllers"
	salesPerms "josex/web/modules/sales/permissions"
	tenancyMW "josex/web/modules/tenancy/middleware"

	"github.com/gin-gonic/gin"
)

// SetupSalesRoutes registers all sales module routes
func SetupSalesRoutes(apiV1 *gin.RouterGroup, customerCtrl *controllers.CustomerController, orderCtrl *controllers.SalesOrderController, authMiddleware gin.HandlerFunc, tenantMiddleware gin.HandlerFunc) {
	// Customer routes
	customers := apiV1.Group("/sales/customers", authMiddleware, tenantMiddleware)
	{
		customers.GET("/:id", customerCtrl.GetCustomer)
		customers.POST("",
			tenancyMW.RequirePermission(salesPerms.CustomersWrite),
			customerCtrl.CreateCustomer)
		customers.PATCH("/:id",
			tenancyMW.RequirePermission(salesPerms.CustomersWrite),
			customerCtrl.UpdateCustomer)
		customers.DELETE("/:id",
			tenancyMW.RequirePermission(salesPerms.CustomersDelete),
			customerCtrl.DeleteCustomer)
	}

	// Sales Order routes
	orders := apiV1.Group("/sales/orders", authMiddleware, tenantMiddleware)
	{
		orders.GET("", orderCtrl.ListSalesOrders)
		orders.GET("/:id", orderCtrl.GetSalesOrder)
		orders.GET("/:id/with-batches", orderCtrl.GetOrderWithBatches)
		orders.POST("",
			tenancyMW.RequirePermission(salesPerms.OrdersCreate),
			orderCtrl.CreateSalesOrder)
		orders.PATCH("/:id",
			tenancyMW.RequirePermission(salesPerms.OrdersManage),
			orderCtrl.UpdateSalesOrder)
		orders.POST("/:id/items",
			tenancyMW.RequirePermission(salesPerms.OrdersManage),
			orderCtrl.AddOrderItem)
		orders.PATCH("/:id/complete",
			tenancyMW.RequirePermission(salesPerms.OrdersManage),
			orderCtrl.CompleteOrder)
		orders.PATCH("/:id/cancel",
			tenancyMW.RequirePermission(salesPerms.OrdersManage),
			orderCtrl.CancelOrder)
	}

	// Reports routes
	reports := apiV1.Group("/sales/reports", authMiddleware, tenantMiddleware)
	{
		reports.GET("/sales",
			tenancyMW.RequirePermission(salesPerms.ReportsRead),
			orderCtrl.GetSalesReport)
	}

	// Returns routes
	returns := apiV1.Group("/sales/returns", authMiddleware, tenantMiddleware)
	{
		returns.GET("",
			tenancyMW.RequirePermission(salesPerms.ReturnsRead),
			orderCtrl.GetReturns)
		returns.POST("",
			tenancyMW.RequirePermission(salesPerms.ReturnsManage),
			orderCtrl.CreateReturn)
		returns.PATCH("/:id/approve",
			tenancyMW.RequirePermission(salesPerms.ReturnsManage),
			orderCtrl.ApproveReturn)
	}

	// Payments routes
	payments := apiV1.Group("/sales/payments", authMiddleware, tenantMiddleware)
	{
		payments.GET("",
			tenancyMW.RequirePermission(salesPerms.PaymentsRead),
			orderCtrl.GetPayments)
		payments.POST("",
			tenancyMW.RequirePermission(salesPerms.PaymentsCreate),
			orderCtrl.CreatePayment)
	}
}
