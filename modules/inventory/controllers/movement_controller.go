package controllers

import (
	"net/http"

	coreErrors "josex/web/modules/core/errors"
	"josex/web/modules/inventory/models"
	"josex/web/modules/inventory/services"
	"josex/web/modules/inventory/utils"

	"github.com/gin-gonic/gin"
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
	var dto models.RecordMovementDto

	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.invalid", err.Error()))
		return
	}

	userID := c.GetString("user_id")
	movement, err := ctrl.movementService.RecordMovement(c.Request.Context(), dto, &userID)
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
	movementID := c.Param("id")

	movement, err := ctrl.movementService.GetMovement(c.Request.Context(), movementID)
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
	productID := c.Param("product_id")

	stock, err := ctrl.movementService.GetProductStock(c.Request.Context(), productID)
	if err != nil {
		status := utils.GetHTTPStatusFromError(err)
		c.JSON(status, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusOK, stock)
}
