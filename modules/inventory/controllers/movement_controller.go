package controllers

import (
	"net/http"

	coreErrors "josex/web/modules/core/errors"
	"josex/web/modules/inventory/models"
	"josex/web/modules/inventory/services"
	"josex/web/modules/inventory/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type MovementController struct {
	movementService *services.MovementService
}

func NewMovementController(movementService *services.MovementService) *MovementController {
	return &MovementController{
		movementService: movementService,
	}
}

// RecordMovement godoc
// @Summary Record an inventory movement
// @Description Record a stock movement (purchase, sale, adjustment, etc.)
// @Tags inventory
// @Accept json
// @Produce json
// @Param request body models.RecordMovementDto true "Movement request"
// @Success 201 {object} models.RecordMovementResponse
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 409 {object} map[string]interface{}
// @Router /api/v1/inventory/movements [post]
func (ctrl *MovementController) RecordMovement(c *gin.Context) {
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

	var dto models.RecordMovementDto

	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.invalid", err.Error()))
		return
	}

	userID := c.GetString("user_id")
	movement, err := ctrl.movementService.RecordMovement(c.Request.Context(), tenantID, dto, &userID)
	if err != nil {
		status := utils.GetHTTPStatusFromError(err)
		c.JSON(status, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusCreated, movement)
}

// GetMovement godoc
// @Summary Get movement by ID
// @Description Retrieve a specific inventory movement
// @Tags inventory
// @Produce json
// @Param id path string true "Movement ID"
// @Success 200 {object} models.InventoryMovement
// @Failure 404 {object} map[string]interface{}
// @Router /api/v1/inventory/movements/:id [get]
func (ctrl *MovementController) GetMovement(c *gin.Context) {
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

	movementID := c.Param("id")

	movement, err := ctrl.movementService.GetMovement(c.Request.Context(), tenantID, movementID)
	if err != nil {
		status := utils.GetHTTPStatusFromError(err)
		c.JSON(status, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusOK, movement)
}

// GetProductStock godoc
// @Summary Get current product stock
// @Description Retrieve current stock level for a product
// @Tags inventory
// @Produce json
// @Param product_id path string true "Product ID"
// @Success 200 {object} models.ProductStock
// @Failure 404 {object} map[string]interface{}
// @Router /api/v1/inventory/stock/:product_id [get]
func (ctrl *MovementController) GetProductStock(c *gin.Context) {
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

	productID := c.Param("product_id")

	stock, err := ctrl.movementService.GetProductStock(c.Request.Context(), tenantID, productID)
	if err != nil {
		status := utils.GetHTTPStatusFromError(err)
		c.JSON(status, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusOK, stock)
}
