package routes

import (
	"josex/web/modules/sales/controllers"

	"github.com/gin-gonic/gin"
)

// SetupSalesRoutes registers all sales module routes
func SetupSalesRoutes(apiV1 *gin.RouterGroup, customerCtrl *controllers.CustomerController, orderCtrl *controllers.SalesOrderController) {
	// Customer routes
	customerGroup := apiV1.Group("/sales/customers")
	{
		customerGroup.POST("", customerCtrl.CreateCustomer)
		customerGroup.GET("/:id", customerCtrl.GetCustomer)
		customerGroup.PATCH("/:id", customerCtrl.UpdateCustomer)
		customerGroup.DELETE("/:id", customerCtrl.DeleteCustomer)
	}

	// Sales Order routes
	orderGroup := apiV1.Group("/sales/orders")
	{
		orderGroup.POST("", orderCtrl.CreateSalesOrder)
		orderGroup.GET("", orderCtrl.ListSalesOrders)
		orderGroup.GET("/:id", orderCtrl.GetSalesOrder)
		orderGroup.PATCH("/:id", orderCtrl.UpdateSalesOrder)
	}
}
