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

type SkuController struct {
	skuService inventoryInterfaces.SkuService
}

func NewSkuController(skuService inventoryInterfaces.SkuService) *SkuController {
	return &SkuController{
		skuService: skuService,
	}
}

// ListProductSkus godoc
// @Summary List the SKUs of a product
// @Description One row per sellable combination — the default SKU first, then the generated ones — with its option labels, quantities and stock status
// @Tags inventory
// @Produce json
// @Param id path string true "Product ID"
// @Param page query int false "Page (default 1)"
// @Param page_size query int false "Page size (default 100, max 100)"
// @Success 200 {object} models.ListProductSkusResponse
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /api/v1/inventory/products/{id}/skus [get]
func (ctrl *SkuController) ListProductSkus(c *gin.Context) {
	tenantID, ok := tenantIDFromContext(c)
	if !ok {
		return
	}

	var query models.ListProductSkusQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.invalid", err.Error()))
		return
	}

	result, err := ctrl.skuService.ListProductSkus(
		c.Request.Context(), tenantID, c.Param("id"), query,
	)
	if err != nil {
		respondWithSkuError(c, err)
		return
	}

	c.JSON(http.StatusOK, result)
}

// GenerateSkus godoc
// @Summary Generate the combinations of a product
// @Description Creates one SKU per missing combination of the product's inventory axes. Re-runnable: existing combinations are left alone and the default SKU keeps its quantity.
// @Tags inventory
// @Produce json
// @Param id path string true "Product ID"
// @Success 201 {object} models.GenerateSkusResponse
// @Failure 404 {object} map[string]interface{}
// @Failure 409 {object} map[string]interface{}
// @Router /api/v1/inventory/products/{id}/skus/generate [post]
func (ctrl *SkuController) GenerateSkus(c *gin.Context) {
	tenantID, ok := tenantIDFromContext(c)
	if !ok {
		return
	}

	result, err := ctrl.skuService.GenerateSkus(
		c.Request.Context(), tenantID, c.Param("id"),
	)
	if err != nil {
		respondWithSkuError(c, err)
		return
	}

	c.JSON(http.StatusCreated, result)
}

// RedistributeStock godoc
// @Summary Move the default SKU's stock onto the combinations
// @Description Atomic move of the whole unassigned bucket. The amounts must sum to exactly the default SKU's current quantity; anything else changes nothing and returns 400.
// @Tags inventory
// @Accept json
// @Produce json
// @Param id path string true "Product ID"
// @Param request body models.RedistributeStockDto true "Redistribution request"
// @Success 200 {object} models.RedistributeStockResponse
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 409 {object} map[string]interface{}
// @Router /api/v1/inventory/products/{id}/skus/redistribute [post]
func (ctrl *SkuController) RedistributeStock(c *gin.Context) {
	tenantID, ok := tenantIDFromContext(c)
	if !ok {
		return
	}

	var dto models.RedistributeStockDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.invalid", err.Error()))
		return
	}

	var createdBy *string
	if userID := c.GetString("user_id"); userID != "" {
		createdBy = &userID
	}

	result, err := ctrl.skuService.RedistributeStock(
		c.Request.Context(), tenantID, c.Param("id"), dto, createdBy,
	)
	if err != nil {
		respondWithSkuError(c, err)
		return
	}

	c.JSON(http.StatusOK, result)
}

// tenantIDFromContext reads the tenant the tenancy middleware resolved. It is
// never taken from the body or the path.
func tenantIDFromContext(c *gin.Context) (uuid.UUID, bool) {
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists || tenantIDRaw == nil {
		c.JSON(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
		return uuid.Nil, false
	}

	tenantID, err := uuid.Parse(tenantIDRaw.(string))
	if err != nil {
		c.JSON(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
		return uuid.Nil, false
	}

	return tenantID, true
}

// respondWithSkuError maps what the three SPs raise. Everything they refuse is a
// conflict with the product's current state except the redistribution sum, which
// D4 fixed as a 400: it is the client's arithmetic that is wrong.
func respondWithSkuError(c *gin.Context, err error) {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Message {
		case inventoryErrors.ProductNotFound:
			c.JSON(http.StatusNotFound, coreErrors.BuildErrorSingle(c, inventoryErrors.ProductNotFound))
			return
		case inventoryErrors.SkuNotFound:
			c.JSON(http.StatusNotFound, coreErrors.BuildErrorSingle(c, inventoryErrors.SkuNotFound))
			return
		case inventoryErrors.SkusRedistributionMismatch:
			c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, inventoryErrors.SkusRedistributionMismatch))
			return
		case inventoryErrors.SkusNoAxes:
			c.JSON(http.StatusConflict, coreErrors.BuildErrorSingle(c, inventoryErrors.SkusNoAxes))
			return
		case inventoryErrors.SkusCapExceeded:
			c.JSON(http.StatusConflict, coreErrors.BuildErrorSingle(c, inventoryErrors.SkusCapExceeded))
			return
		case inventoryErrors.SkusSkuCollision:
			c.JSON(http.StatusConflict, coreErrors.BuildErrorSingle(c, inventoryErrors.SkusSkuCollision))
			return
		case inventoryErrors.SkusRedistributionReserved:
			c.JSON(http.StatusConflict, coreErrors.BuildErrorSingle(c, inventoryErrors.SkusRedistributionReserved))
			return
		case inventoryErrors.SkuCombinationKeyMismatch:
			c.JSON(http.StatusConflict, coreErrors.BuildErrorSingle(c, inventoryErrors.SkuCombinationKeyMismatch))
			return
		}
	}

	c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
}
