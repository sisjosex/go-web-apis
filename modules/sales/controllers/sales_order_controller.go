package controllers

import (
	"net/http"
	"strconv"

	coreErrors "josex/web/modules/core/errors"
	coreModels "josex/web/modules/core/models"
	"josex/web/modules/sales/models"
	"josex/web/modules/sales/services"
	"josex/web/modules/sales/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// SalesOrderController handles sales order HTTP requests
type SalesOrderController struct {
	service *services.SalesOrderService
}

// NewSalesOrderController creates a new sales order controller
func NewSalesOrderController(service *services.SalesOrderService) *SalesOrderController {
	return &SalesOrderController{
		service: service,
	}
}

// CreateSalesOrder godoc
// @Summary Create a new sales order
// @Tags Sales - Orders
// @Accept json
// @Produce json
// @Param request body models.CreateSalesOrderRequestDto true "Order data"
// @Success 201 {object} models.SalesOrder
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Router /sales/orders [post]
func (ctrl *SalesOrderController) CreateSalesOrder(c *gin.Context) {
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists || tenantIDRaw == nil {
		c.JSON(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
		return
	}
	tenantID, err := uuid.Parse(tenantIDRaw.(string))
	if err != nil {
		c.JSON(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
		return
	}

	var dto models.CreateSalesOrderRequestDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.failed", err.Error()))
		return
	}

	order, err := ctrl.service.CreateSalesOrder(c.Request.Context(), tenantID, &dto)
	if err != nil {
		status := utils.GetHTTPStatusFromError(err)
		c.JSON(status, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": order})
}

// GetSalesOrder godoc
// @Summary Get a sales order by ID
// @Tags Sales - Orders
// @Produce json
// @Param id path string true "Order ID"
// @Success 200 {object} models.SalesOrder
// @Failure 404 {object} coreErrors.ErrorResponse
// @Router /sales/orders/{id} [get]
func (ctrl *SalesOrderController) GetSalesOrder(c *gin.Context) {
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists || tenantIDRaw == nil {
		c.JSON(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
		return
	}
	tenantID, err := uuid.Parse(tenantIDRaw.(string))
	if err != nil {
		c.JSON(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, "validation.invalid-uuid"))
		return
	}

	order, err := ctrl.service.GetSalesOrderByID(c.Request.Context(), tenantID, id)
	if err != nil {
		status := utils.GetHTTPStatusFromError(err)
		c.JSON(status, coreErrors.BuildError(c, err))
		return
	}

	// Get order items
	items, _ := ctrl.service.GetOrderItems(c.Request.Context(), tenantID, id)
	order.Items = items

	c.JSON(http.StatusOK, gin.H{"data": order})
}

// ListSalesOrders godoc
// @Summary List sales orders for tenant
// @Tags Sales - Orders
// @Produce json
// @Param limit query int false "Limit (default 20)"
// @Param offset query int false "Offset (default 0)"
// @Success 200 {array} models.SalesOrder
// @Router /sales/orders [get]
func (ctrl *SalesOrderController) ListSalesOrders(c *gin.Context) {
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists || tenantIDRaw == nil {
		c.JSON(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
		return
	}
	tenantID, err := uuid.Parse(tenantIDRaw.(string))
	if err != nil {
		c.JSON(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
		return
	}

	limit := 20
	offset := 0
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil {
			limit = parsed
		}
	}
	if o := c.Query("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil {
			offset = parsed
		}
	}

	orders, err := ctrl.service.GetAllSalesOrders(c.Request.Context(), tenantID, limit, offset)
	if err != nil {
		status := utils.GetHTTPStatusFromError(err)
		c.JSON(status, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": orders})
}

// UpdateSalesOrder godoc
// @Summary Update a sales order
// @Tags Sales - Orders
// @Accept json
// @Produce json
// @Param id path string true "Order ID"
// @Param request body models.UpdateSalesOrderRequestDto true "Updated order data"
// @Success 200 {object} models.SalesOrder
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Router /sales/orders/{id} [patch]
func (ctrl *SalesOrderController) UpdateSalesOrder(c *gin.Context) {
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists || tenantIDRaw == nil {
		c.JSON(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
		return
	}
	tenantID, err := uuid.Parse(tenantIDRaw.(string))
	if err != nil {
		c.JSON(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, "validation.invalid-uuid"))
		return
	}

	var dto models.UpdateSalesOrderRequestDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.failed", err.Error()))
		return
	}

	order, err := ctrl.service.UpdateSalesOrder(c.Request.Context(), tenantID, id, &dto)
	if err != nil {
		status := utils.GetHTTPStatusFromError(err)
		c.JSON(status, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": order})
}

// ========== PHASE 1: Batch Assignment & Order Completion ==========

// AddOrderItem godoc
// @Summary Add an item to an order with batch assignment
// @Tags Sales - Orders
// @Accept json
// @Produce json
// @Param id path string true "Order ID"
// @Param request body models.AddOrderItemRequestDto true "Item data"
// @Success 201 {object} models.OrderItem
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 409 {object} coreErrors.ErrorResponse
// @Router /sales/orders/{id}/items [post]
func (ctrl *SalesOrderController) AddOrderItem(c *gin.Context) {
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists || tenantIDRaw == nil {
		c.JSON(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
		return
	}
	tenantID, err := uuid.Parse(tenantIDRaw.(string))
	if err != nil {
		c.JSON(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
		return
	}

	orderId, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, "validation.invalid-uuid"))
		return
	}

	var dto models.AddOrderItemRequestDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.failed", err.Error()))
		return
	}

	item, err := ctrl.service.AddOrderItemWithBatch(c.Request.Context(), tenantID, orderId, dto.ProductID, dto.Quantity, dto.UnitPrice)
	if err != nil {
		status := utils.GetHTTPStatusFromError(err)
		c.JSON(status, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": item})
}

// CompleteOrder godoc
// @Summary Complete a sales order and consume inventory
// @Tags Sales - Orders
// @Accept json
// @Produce json
// @Param id path string true "Order ID"
// @Success 200 {object} models.SalesOrder
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 409 {object} coreErrors.ErrorResponse
// @Router /sales/orders/{id}/complete [patch]
func (ctrl *SalesOrderController) CompleteOrder(c *gin.Context) {
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists || tenantIDRaw == nil {
		c.JSON(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
		return
	}
	tenantID, err := uuid.Parse(tenantIDRaw.(string))
	if err != nil {
		c.JSON(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
		return
	}

	orderId, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, "validation.invalid-uuid"))
		return
	}

	order, err := ctrl.service.CompleteOrder(c.Request.Context(), tenantID, orderId)
	if err != nil {
		status := utils.GetHTTPStatusFromError(err)
		c.JSON(status, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": order})
}

// ========== PHASE 2: Reporting & Cancellation ==========

// CancelOrder godoc
// @Summary Cancel a sales order and release batch assignments
// @Tags Sales - Orders
// @Accept json
// @Produce json
// @Param id path string true "Order ID"
// @Success 200 {object} models.SalesOrder
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 409 {object} coreErrors.ErrorResponse
// @Router /sales/orders/{id}/cancel [patch]
func (ctrl *SalesOrderController) CancelOrder(c *gin.Context) {
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists || tenantIDRaw == nil {
		c.JSON(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
		return
	}
	tenantID, err := uuid.Parse(tenantIDRaw.(string))
	if err != nil {
		c.JSON(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
		return
	}

	orderId, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, "validation.invalid-uuid"))
		return
	}

	order, err := ctrl.service.CancelOrder(c.Request.Context(), tenantID, orderId)
	if err != nil {
		status := utils.GetHTTPStatusFromError(err)
		c.JSON(status, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": order})
}

// GetOrderWithBatches godoc
// @Summary Get order details with batch assignments
// @Tags Sales - Orders
// @Produce json
// @Param id path string true "Order ID"
// @Success 200 {object} models.OrderWithBatches
// @Failure 404 {object} coreErrors.ErrorResponse
// @Router /sales/orders/{id}/with-batches [get]
func (ctrl *SalesOrderController) GetOrderWithBatches(c *gin.Context) {
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists || tenantIDRaw == nil {
		c.JSON(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
		return
	}
	tenantID, err := uuid.Parse(tenantIDRaw.(string))
	if err != nil {
		c.JSON(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
		return
	}

	orderId, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, "validation.invalid-uuid"))
		return
	}

	order, err := ctrl.service.GetOrderWithBatches(c.Request.Context(), tenantID, orderId)
	if err != nil {
		status := utils.GetHTTPStatusFromError(err)
		c.JSON(status, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": order})
}

// GetSalesReport godoc
// @Summary Get sales report for date range
// @Tags Sales - Reports
// @Produce json
// @Param start_date query string true "Start date (YYYY-MM-DD)"
// @Param end_date query string true "End date (YYYY-MM-DD)"
// @Success 200 {object} models.SalesReportResponse
// @Failure 400 {object} coreErrors.ErrorResponse
// @Router /sales/reports/sales [get]
func (ctrl *SalesOrderController) GetSalesReport(c *gin.Context) {
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists || tenantIDRaw == nil {
		c.JSON(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
		return
	}
	tenantID, err := uuid.Parse(tenantIDRaw.(string))
	if err != nil {
		c.JSON(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
		return
	}

	startDateStr := c.Query("start_date")
	endDateStr := c.Query("end_date")

	if startDateStr == "" || endDateStr == "" {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.failed", "start_date and end_date are required"))
		return
	}

	var startDate coreModels.DateOnly
	errStart := startDate.UnmarshalJSON([]byte(`"` + startDateStr + `"`))
	if errStart != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.failed", "invalid start_date format"))
		return
	}

	var endDate coreModels.DateOnly
	errEnd := endDate.UnmarshalJSON([]byte(`"` + endDateStr + `"`))
	if errEnd != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.failed", "invalid end_date format"))
		return
	}

	reports, err := ctrl.service.GetSalesReport(c.Request.Context(), tenantID, startDate, endDate)
	if err != nil {
		status := utils.GetHTTPStatusFromError(err)
		c.JSON(status, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": models.SalesReportResponse{Metrics: reports}})
}

// ========== PHASE 3: Returns & Payments ==========

// CreateReturn godoc
// @Summary Create a return request for a completed order
// @Tags Sales - Returns
// @Accept json
// @Produce json
// @Param request body models.CreateReturnRequestDto true "Return data"
// @Success 201 {object} models.Return
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 409 {object} coreErrors.ErrorResponse
// @Router /sales/returns [post]
func (ctrl *SalesOrderController) CreateReturn(c *gin.Context) {
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists || tenantIDRaw == nil {
		c.JSON(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
		return
	}
	tenantID, err := uuid.Parse(tenantIDRaw.(string))
	if err != nil {
		c.JSON(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
		return
	}

	var dto models.CreateReturnRequestDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.failed", err.Error()))
		return
	}

	ret, err := ctrl.service.CreateReturn(c.Request.Context(), tenantID, dto.OrderID, dto.Reason)
	if err != nil {
		status := utils.GetHTTPStatusFromError(err)
		c.JSON(status, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": ret})
}

// ApproveReturn godoc
// @Summary Approve a return request and restore inventory
// @Tags Sales - Returns
// @Accept json
// @Produce json
// @Param id path string true "Return ID"
// @Success 200 {object} models.Return
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 409 {object} coreErrors.ErrorResponse
// @Router /sales/returns/{id}/approve [patch]
func (ctrl *SalesOrderController) ApproveReturn(c *gin.Context) {
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists || tenantIDRaw == nil {
		c.JSON(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
		return
	}
	tenantID, err := uuid.Parse(tenantIDRaw.(string))
	if err != nil {
		c.JSON(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
		return
	}

	returnId, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, "validation.invalid-uuid"))
		return
	}

	ret, err := ctrl.service.ApproveReturn(c.Request.Context(), tenantID, returnId)
	if err != nil {
		status := utils.GetHTTPStatusFromError(err)
		c.JSON(status, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": ret})
}

// GetReturns godoc
// @Summary Get returns for an order
// @Tags Sales - Returns
// @Produce json
// @Param order_id query string required "Order ID"
// @Success 200 {object} []models.Return
// @Failure 400 {object} coreErrors.ErrorResponse
// @Router /sales/returns [get]
func (ctrl *SalesOrderController) GetReturns(c *gin.Context) {
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists || tenantIDRaw == nil {
		c.JSON(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
		return
	}
	tenantID, err := uuid.Parse(tenantIDRaw.(string))
	if err != nil {
		c.JSON(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
		return
	}

	orderIdStr := c.Query("order_id")
	if orderIdStr == "" {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.failed", "order_id is required"))
		return
	}

	orderId, err := uuid.Parse(orderIdStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, "validation.invalid-uuid"))
		return
	}

	returns, err := ctrl.service.GetReturnsByOrder(c.Request.Context(), tenantID, orderId)
	if err != nil {
		status := utils.GetHTTPStatusFromError(err)
		c.JSON(status, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": returns})
}

// CreatePayment godoc
// @Summary Create a payment record for an order
// @Tags Sales - Payments
// @Accept json
// @Produce json
// @Param request body models.CreatePaymentRequestDto true "Payment data"
// @Success 201 {object} models.Payment
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Router /sales/payments [post]
func (ctrl *SalesOrderController) CreatePayment(c *gin.Context) {
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists || tenantIDRaw == nil {
		c.JSON(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
		return
	}
	tenantID, err := uuid.Parse(tenantIDRaw.(string))
	if err != nil {
		c.JSON(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
		return
	}

	var dto models.CreatePaymentRequestDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.failed", err.Error()))
		return
	}

	payment, err := ctrl.service.CreatePayment(c.Request.Context(), tenantID, dto.OrderID, dto.Amount, dto.PaymentMethod, dto.ReferenceNumber, dto.Notes)
	if err != nil {
		status := utils.GetHTTPStatusFromError(err)
		c.JSON(status, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": payment})
}

// GetPayments godoc
// @Summary Get payments for an order
// @Tags Sales - Payments
// @Produce json
// @Param order_id query string required "Order ID"
// @Success 200 {object} []models.Payment
// @Failure 400 {object} coreErrors.ErrorResponse
// @Router /sales/payments [get]
func (ctrl *SalesOrderController) GetPayments(c *gin.Context) {
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists || tenantIDRaw == nil {
		c.JSON(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
		return
	}
	tenantID, err := uuid.Parse(tenantIDRaw.(string))
	if err != nil {
		c.JSON(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
		return
	}

	orderIdStr := c.Query("order_id")
	if orderIdStr == "" {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.failed", "order_id is required"))
		return
	}

	orderId, err := uuid.Parse(orderIdStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, "validation.invalid-uuid"))
		return
	}

	payments, err := ctrl.service.GetPayments(c.Request.Context(), tenantID, orderId)
	if err != nil {
		status := utils.GetHTTPStatusFromError(err)
		c.JSON(status, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": payments})
}
