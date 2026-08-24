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

type StockController struct {
	stockService inventoryInterfaces.StockService
}

func NewStockController(stockService inventoryInterfaces.StockService) *StockController {
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
	result, err := ctrl.stockService.UpdateReorderLevel(c.Request.Context(), tenantID, productID, dto.ReorderLevel, &userID, nil)
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

	c.JSON(http.StatusOK, result)
}

// ReserveStock godoc
// @Summary Reserve stock for a sales order
// @Description Reserve inventory when a sales order is created
// @Tags inventory
// @Accept json
// @Produce json
// @Param request body models.ReserveStockDto true "Reserve request"
// @Success 200 {object} models.StockReservationResponse
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 409 {object} map[string]string
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

	var dto models.ReserveStockDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, inventoryErrors.InsufficientStock, err.Error()))
		return
	}

	result, err := ctrl.stockService.ReserveStock(c.Request.Context(), tenantID, dto.ProductID, dto.Quantity, dto.SkuID)
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
			// INV-011 D1 — the client asked to reserve "the product" on a product
			// whose stock lives on combinations; its arithmetic is wrong, not its state.
			case inventoryErrors.ReserveSkuRequired:
				c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, inventoryErrors.ReserveSkuRequired))
				return
			case inventoryErrors.InsufficientStock:
				c.JSON(http.StatusConflict, coreErrors.BuildErrorSingle(c, inventoryErrors.InsufficientStock))
				return
			}
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
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
// @Param request body models.ReleaseReservedStockDto true "Release request"
// @Success 200 {object} models.StockReservationResponse
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
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

	var dto models.ReleaseReservedStockDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, inventoryErrors.InsufficientStock, err.Error()))
		return
	}

	result, err := ctrl.stockService.ReleaseReservedStock(c.Request.Context(), tenantID, dto.ProductID, dto.Quantity, dto.SkuID)
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
			case inventoryErrors.ReserveSkuRequired:
				c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, inventoryErrors.ReserveSkuRequired))
				return
			}
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusOK, result)
}
