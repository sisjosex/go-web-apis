package controllers

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"josex/web/modules/core/api"
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

// ListProductSkusInput is ListProductSkus's request.
type ListProductSkusInput struct {
	ID    string `path:"id"`
	Query api.Query[models.ListProductSkusQuery]
}

// ListProductSkus is GET /products/{id}/skus: List the SKUs of a product.
func (ctrl *SkuController) ListProductSkus(ctx context.Context, in *ListProductSkusInput) (*api.Out[*models.ListProductSkusResponse], error) {
	c := api.Gin(ctx)
	tenantID, err := tenantIDFromContext(c)
	if err != nil {
		return nil, err
	}

	var query models.ListProductSkusQuery
	if err := in.Query.Into(ctx, &query); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.invalid", err.Error()))
	}

	result, err := ctrl.skuService.ListProductSkus(
		c.Request.Context(), tenantID, in.ID, query,
	)
	if err != nil {
		return nil, respondWithSkuError(c, err)
	}

	return api.OK(result)
}

// GenerateSkusInput is GenerateSkus's request.
type GenerateSkusInput struct {
	ID string `path:"id"`
}

// GenerateSkus is POST /products/{id}/skus/generate: Generate the combinations of a product.
func (ctrl *SkuController) GenerateSkus(ctx context.Context, in *GenerateSkusInput) (*api.Out[*models.GenerateSkusResponse], error) {
	c := api.Gin(ctx)
	tenantID, err := tenantIDFromContext(c)
	if err != nil {
		return nil, err
	}

	result, err := ctrl.skuService.GenerateSkus(
		c.Request.Context(), tenantID, in.ID,
	)
	if err != nil {
		return nil, respondWithSkuError(c, err)
	}

	return api.OK(result)
}

// RedistributeStockInput is RedistributeStock's request.
type RedistributeStockInput struct {
	ID   string `path:"id"`
	JSON api.Body[models.RedistributeStockDto]
}

// RedistributeStock is POST /products/{id}/skus/redistribute: Move the default SKU's stock onto the combinations.
func (ctrl *SkuController) RedistributeStock(ctx context.Context, in *RedistributeStockInput) (*api.Out[*models.RedistributeStockResponse], error) {
	c := api.Gin(ctx)
	tenantID, err := tenantIDFromContext(c)
	if err != nil {
		return nil, err
	}

	var dto models.RedistributeStockDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.invalid", err.Error()))
	}

	var createdBy *string
	if userID := c.GetString("user_id"); userID != "" {
		createdBy = &userID
	}

	result, err := ctrl.skuService.RedistributeStock(
		c.Request.Context(), tenantID, in.ID, dto, createdBy,
	)
	if err != nil {
		return nil, respondWithSkuError(c, err)
	}

	return api.OK(result)
}

// tenantIDFromContext reads the tenant the tenancy middleware resolved. It is
// never taken from the body or the path.
func tenantIDFromContext(c *gin.Context) (uuid.UUID, error) {
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists || tenantIDRaw == nil {
		return uuid.Nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}

	tenantID, err := uuid.Parse(tenantIDRaw.(string))
	if err != nil {
		return uuid.Nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}

	return tenantID, nil
}

// respondWithSkuError maps what the three SPs raise. Everything they refuse is a
// conflict with the product's current state except the redistribution sum, which
// D4 fixed as a 400: it is the client's arithmetic that is wrong.
func respondWithSkuError(c *gin.Context, err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Message {
		case inventoryErrors.ProductNotFound:
			return api.Fail(http.StatusNotFound, coreErrors.BuildErrorSingle(c, inventoryErrors.ProductNotFound))
		case inventoryErrors.SkuNotFound:
			return api.Fail(http.StatusNotFound, coreErrors.BuildErrorSingle(c, inventoryErrors.SkuNotFound))
		case inventoryErrors.SkusRedistributionMismatch:
			return api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, inventoryErrors.SkusRedistributionMismatch))
		case inventoryErrors.SkusNoAxes:
			return api.Fail(http.StatusConflict, coreErrors.BuildErrorSingle(c, inventoryErrors.SkusNoAxes))
		case inventoryErrors.SkusCapExceeded:
			return api.Fail(http.StatusConflict, coreErrors.BuildErrorSingle(c, inventoryErrors.SkusCapExceeded))
		case inventoryErrors.SkusSkuCollision:
			return api.Fail(http.StatusConflict, coreErrors.BuildErrorSingle(c, inventoryErrors.SkusSkuCollision))
		case inventoryErrors.SkusRedistributionReserved:
			return api.Fail(http.StatusConflict, coreErrors.BuildErrorSingle(c, inventoryErrors.SkusRedistributionReserved))
		case inventoryErrors.SkuCombinationKeyMismatch:
			return api.Fail(http.StatusConflict, coreErrors.BuildErrorSingle(c, inventoryErrors.SkuCombinationKeyMismatch))
		}
	}

	return api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
}
