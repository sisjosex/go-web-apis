package controllers

import (
	"net/http"
	"strconv"

	coreErrors "josex/web/modules/core/errors"
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
	var dto models.CreateSalesOrderRequestDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.failed", err.Error()))
		return
	}

	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		c.JSON(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.tenant-required"))
		return
	}

	tenantUUID, err := uuid.Parse(tenantID)
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, "validation.invalid-uuid"))
		return
	}

	order, err := ctrl.service.CreateSalesOrder(c.Request.Context(), tenantUUID, &dto)
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
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, "validation.invalid-uuid"))
		return
	}

	order, err := ctrl.service.GetSalesOrderByID(c.Request.Context(), id)
	if err != nil {
		status := utils.GetHTTPStatusFromError(err)
		c.JSON(status, coreErrors.BuildError(c, err))
		return
	}

	// Get order items
	items, _ := ctrl.service.GetOrderItems(c.Request.Context(), id)
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
	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		c.JSON(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.tenant-required"))
		return
	}

	tenantUUID, _ := uuid.Parse(tenantID)

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

	orders, err := ctrl.service.GetSalesOrdersByTenant(c.Request.Context(), tenantUUID, limit, offset)
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

	order, err := ctrl.service.UpdateSalesOrder(c.Request.Context(), id, &dto)
	if err != nil {
		status := utils.GetHTTPStatusFromError(err)
		c.JSON(status, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": order})
}
