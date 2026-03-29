package controllers

import (
	"fmt"
	"net/http"

	coreErrors "josex/web/modules/core/errors"
	"josex/web/modules/inventory/models"
	"josex/web/modules/inventory/services"
	"josex/web/modules/inventory/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type StockController struct {
	stockService *services.StockService
}

func NewStockController(stockService *services.StockService) *StockController {
	return &StockController{
		stockService: stockService,
	}
}

// UpdateReorderLevel godoc
// @Summary Update product reorder level
// @Description Update the minimum stock level for a product
// @Tags inventory
// @Accept json
// @Produce json
// @Param product_id path string true "Product ID"
// @Param request body models.UpdateReorderLevelDto true "Reorder level request"
// @Success 200 {object} models.UpdateReorderLevelResponse
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /api/v1/inventory/products/:product_id/reorder-level [patch]
func (ctrl *StockController) UpdateReorderLevel(c *gin.Context) {
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

	productID := c.Param("id")
	var dto models.UpdateReorderLevelDto

	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.invalid", err.Error()))
		return
	}

	userID := c.GetString("user_id")
	result, err := ctrl.stockService.UpdateReorderLevel(c.Request.Context(), tenantID, productID, dto.ReorderLevel, &userID)
	if err != nil {
		status := utils.GetHTTPStatusFromError(err)
		c.JSON(status, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusOK, result)
}

// ReserveStock godoc
// @Summary Reserve stock for a sales order
// @Description Reserve inventory when a sales order is created
// @Tags inventory
// @Accept json
// @Produce json
// @Param request body map[string]interface{} true "Reserve request"
// @Success 200 {object} models.StockReservationResponse
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 409 {object} map[string]interface{}
// @Router /api/v1/inventory/reserve [post]
func (ctrl *StockController) ReserveStock(c *gin.Context) {
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

	var body map[string]interface{}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.invalid", err.Error()))
		return
	}

	// Extract and validate fields
	productIDVal, ok := body["product_id"]
	if !ok {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.required", "product_id is required"))
		return
	}

	quantityVal, ok := body["quantity"]
	if !ok {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.required", "quantity is required"))
		return
	}

	productID := fmt.Sprintf("%v", productIDVal)
	quantity, ok := quantityVal.(float64)
	if !ok {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.invalid", "quantity must be a number"))
		return
	}

	if quantity <= 0 {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.invalid", "quantity must be greater than 0"))
		return
	}

	result, err := ctrl.stockService.ReserveStock(c.Request.Context(), tenantID, productID, quantity)
	if err != nil {
		status := utils.GetHTTPStatusFromError(err)
		c.JSON(status, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusOK, result)
}

// ReleaseReservedStock godoc
// @Summary Release reserved stock
// @Description Release inventory when a sales order is cancelled
// @Tags inventory
// @Accept json
// @Produce json
// @Param request body map[string]interface{} true "Release request"
// @Success 200 {object} models.StockReservationResponse
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /api/v1/inventory/release-reserved [post]
func (ctrl *StockController) ReleaseReservedStock(c *gin.Context) {
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

	var body map[string]interface{}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.invalid", err.Error()))
		return
	}

	// Extract and validate fields
	productIDVal, ok := body["product_id"]
	if !ok {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.required", "product_id is required"))
		return
	}

	quantityVal, ok := body["quantity"]
	if !ok {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.required", "quantity is required"))
		return
	}

	productID := fmt.Sprintf("%v", productIDVal)
	quantity, ok := quantityVal.(float64)
	if !ok {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.invalid", "quantity must be a number"))
		return
	}

	if quantity <= 0 {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.invalid", "quantity must be greater than 0"))
		return
	}

	result, err := ctrl.stockService.ReleaseReservedStock(c.Request.Context(), tenantID, productID, quantity)
	if err != nil {
		status := utils.GetHTTPStatusFromError(err)
		c.JSON(status, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusOK, result)
}
