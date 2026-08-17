package controllers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	coreErrors "josex/web/modules/core/errors"
	inventoryErrors "josex/web/modules/inventory/errors"
	inventoryInterfaces "josex/web/modules/inventory/interfaces"
	"josex/web/modules/inventory/models"
)

type MovementController struct {
	movementService inventoryInterfaces.MovementService
}

func NewMovementController(movementService inventoryInterfaces.MovementService) *MovementController {
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
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Message {
			case inventoryErrors.ProductNotFound:
				c.JSON(http.StatusNotFound, coreErrors.BuildErrorSingle(c, inventoryErrors.ProductNotFound))
				return
			case inventoryErrors.SkuNotFound:
				c.JSON(http.StatusNotFound, coreErrors.BuildErrorSingle(c, inventoryErrors.SkuNotFound))
				return
			// INV-011 D1 — a movement on a stock-by-variant product has to name its
			// combination; falling back to the unassigned bucket is what this refuses.
			case inventoryErrors.MovementSkuRequired:
				c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, inventoryErrors.MovementSkuRequired))
				return
			case inventoryErrors.InsufficientStock:
				c.JSON(http.StatusConflict, coreErrors.BuildErrorSingle(c, inventoryErrors.InsufficientStock))
				return
			case inventoryErrors.InvalidMovementType:
				c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, inventoryErrors.InvalidMovementType))
				return
			// INV-013 D1/D3 — three ways the body can be internally inconsistent, all
			// of them the caller's to fix and none of them a conflict with stored
			// state, so 400 rather than the 409 insufficient-stock takes.
			case inventoryErrors.MovementDirectionRequired:
				c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, inventoryErrors.MovementDirectionRequired))
				return
			case inventoryErrors.MovementDirectionConflict:
				c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, inventoryErrors.MovementDirectionConflict))
				return
			case inventoryErrors.MovementUnitCostRequired:
				c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, inventoryErrors.MovementUnitCostRequired))
				return
			}
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
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
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Message {
			case inventoryErrors.MovementNotFound:
				c.JSON(http.StatusNotFound, coreErrors.BuildErrorSingle(c, inventoryErrors.MovementNotFound))
				return
			}
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusOK, movement)
}

// ListMovements godoc
// @Summary List inventory movements
// @Description Paginated movement history, optionally filtered by product, type and date range
// @Tags inventory
// @Produce json
// @Param product_id query string false "Product ID"
// @Param movement_type query string false "Movement type (PURCHASE, SALE, ADJUSTMENT, TRANSFER, RETURN, WASTE, PRODUCTION)"
// @Param date_from query string false "Inclusive lower bound, YYYY-MM-DD"
// @Param date_to query string false "Inclusive upper bound, YYYY-MM-DD"
// @Param page query int false "Page number, 1-based" default(1)
// @Param page_size query int false "Rows per page, max 100" default(20)
// @Success 200 {object} models.ListMovementsResponse
// @Failure 400 {object} map[string]interface{}
// @Router /api/v1/inventory/movements [get]
func (ctrl *MovementController) ListMovements(c *gin.Context) {
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

	var query models.ListMovementsQuery

	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.invalid", err.Error()))
		return
	}

	result, err := ctrl.movementService.ListMovements(c.Request.Context(), tenantID, query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusOK, result)
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
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Message {
			case inventoryErrors.ProductNotFound:
				c.JSON(http.StatusNotFound, coreErrors.BuildErrorSingle(c, inventoryErrors.ProductNotFound))
				return
			}
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusOK, stock)
}
