package routes

import (
	"josex/web/modules/core/api"
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
		api.Get(customers, "/{id}", customerCtrl.GetCustomer, api.Doc{Summary: "Get a customer by ID", Errors: []int{404}})
		api.Post(customers, "", customerCtrl.CreateCustomer, api.Doc{Summary: "Create a new customer", Status: 201, Errors: []int{400}}, tenancyMW.RequirePermission(salesPerms.CustomersWrite))
		api.Patch(customers, "/{id}", customerCtrl.UpdateCustomer, api.Doc{Summary: "Update a customer", Errors: []int{400, 404}}, tenancyMW.RequirePermission(salesPerms.CustomersWrite))
		api.Delete(customers, "/{id}", customerCtrl.DeleteCustomer, api.Doc{Summary: "Delete a customer", Status: 204, Errors: []int{404}}, tenancyMW.RequirePermission(salesPerms.CustomersDelete))
	}

	// Sales Order routes
	orders := apiV1.Group("/sales/orders", authMiddleware, tenantMiddleware)
	{
		api.Get(orders, "", orderCtrl.ListSalesOrders, api.Doc{Summary: "List sales orders for tenant"})
		api.Get(orders, "/{id}", orderCtrl.GetSalesOrder, api.Doc{Summary: "Get a sales order by ID", Errors: []int{404}})
		api.Get(orders, "/{id}/with-batches", orderCtrl.GetOrderWithBatches, api.Doc{Summary: "Get order details with batch assignments", Errors: []int{404}})
		api.Post(orders, "", orderCtrl.CreateSalesOrder, api.Doc{Summary: "Create a new sales order", Status: 201, Errors: []int{400, 404}}, tenancyMW.RequirePermission(salesPerms.OrdersCreate))
		api.Patch(orders, "/{id}", orderCtrl.UpdateSalesOrder, api.Doc{Summary: "Update a sales order", Errors: []int{400, 404}}, tenancyMW.RequirePermission(salesPerms.OrdersManage))
		api.Post(orders, "/{id}/items", orderCtrl.AddOrderItem, api.Doc{Summary: "Add an item to an order with batch assignment", Status: 201, Errors: []int{400, 404, 409}}, tenancyMW.RequirePermission(salesPerms.OrdersManage))
		api.Patch(orders, "/{id}/complete", orderCtrl.CompleteOrder, api.Doc{Summary: "Complete a sales order and consume inventory", Errors: []int{404, 409}}, tenancyMW.RequirePermission(salesPerms.OrdersManage))
		api.Patch(orders, "/{id}/cancel", orderCtrl.CancelOrder, api.Doc{Summary: "Cancel a sales order and release batch assignments", Errors: []int{404, 409}}, tenancyMW.RequirePermission(salesPerms.OrdersManage))
	}

	// Reports routes
	reports := apiV1.Group("/sales/reports", authMiddleware, tenantMiddleware)
	{
		api.Get(reports, "/sales", orderCtrl.GetSalesReport, api.Doc{Summary: "Get sales report for date range", Errors: []int{400}}, tenancyMW.RequirePermission(salesPerms.ReportsRead))
	}

	// Returns routes
	returns := apiV1.Group("/sales/returns", authMiddleware, tenantMiddleware)
	{
		api.Get(returns, "", orderCtrl.GetReturns, api.Doc{Summary: "Get returns for an order", Errors: []int{400}}, tenancyMW.RequirePermission(salesPerms.ReturnsRead))
		api.Post(returns, "", orderCtrl.CreateReturn, api.Doc{Summary: "Create a return request for a completed order", Status: 201, Errors: []int{400, 404, 409}}, tenancyMW.RequirePermission(salesPerms.ReturnsManage))
		api.Patch(returns, "/{id}/approve", orderCtrl.ApproveReturn, api.Doc{Summary: "Approve a return request and restore inventory", Errors: []int{404, 409}}, tenancyMW.RequirePermission(salesPerms.ReturnsManage))
	}

	// Payments routes
	payments := apiV1.Group("/sales/payments", authMiddleware, tenantMiddleware)
	{
		api.Get(payments, "", orderCtrl.GetPayments, api.Doc{Summary: "Get payments for an order", Errors: []int{400}}, tenancyMW.RequirePermission(salesPerms.PaymentsRead))
		api.Post(payments, "", orderCtrl.CreatePayment, api.Doc{Summary: "Create a payment record for an order", Status: 201, Errors: []int{400, 404}}, tenancyMW.RequirePermission(salesPerms.PaymentsCreate))
	}
}
