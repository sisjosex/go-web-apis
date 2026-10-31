package controllers

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"josex/web/modules/core/api"
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

// UpdateReorderLevelInput is UpdateReorderLevel's request.
type UpdateReorderLevelInput struct {
	ID   string `path:"id"`
	JSON api.Body[models.UpdateReorderLevelDto]
}

// UpdateReorderLevel is PATCH /products/{id}/reorder-level: Update product reorder level.
func (ctrl *StockController) UpdateReorderLevel(ctx context.Context, in *UpdateReorderLevelInput) (*api.Out[*models.UpdateReorderLevelResponse], error) {
	c := api.Gin(ctx)
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists || tenantIDRaw == nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}
	tenantID, err := uuid.Parse(tenantIDRaw.(string))
	if err != nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}

	productID := in.ID
	var dto models.UpdateReorderLevelDto

	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.invalid", err.Error()))
	}

	userID := c.GetString("user_id")
	result, err := ctrl.stockService.UpdateReorderLevel(c.Request.Context(), tenantID, productID, dto.ReorderLevel, &userID, dto.SkuID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Message {
			case inventoryErrors.ProductNotFound:
				return nil, api.Fail(http.StatusNotFound, coreErrors.BuildErrorSingle(c, inventoryErrors.ProductNotFound))
			}
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}

	return api.OK(result)
}

// ReserveStockInput is ReserveStock's request.
type ReserveStockInput struct {
	JSON api.Body[models.ReserveStockDto]
}

// ReserveStock is POST /reserve: Reserve stock for a sales order.
func (ctrl *StockController) ReserveStock(ctx context.Context, in *ReserveStockInput) (*api.Out[*models.StockReservationResponse], error) {
	c := api.Gin(ctx)
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists || tenantIDRaw == nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}
	tenantID, err := uuid.Parse(tenantIDRaw.(string))
	if err != nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}

	var dto models.ReserveStockDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, inventoryErrors.InsufficientStock, err.Error()))
	}

	result, err := ctrl.stockService.ReserveStock(c.Request.Context(), tenantID, dto.ProductID, dto.Quantity, actingUser(c), dto.SkuID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Message {
			case inventoryErrors.ProductNotFound:
				return nil, api.Fail(http.StatusNotFound, coreErrors.BuildErrorSingle(c, inventoryErrors.ProductNotFound))
			case inventoryErrors.SkuNotFound:
				return nil, api.Fail(http.StatusNotFound, coreErrors.BuildErrorSingle(c, inventoryErrors.SkuNotFound))
			// INV-011 D1 — the client asked to reserve "the product" on a product
			// whose stock lives on combinations; its arithmetic is wrong, not its state.
			case inventoryErrors.ReserveSkuRequired:
				return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, inventoryErrors.ReserveSkuRequired))
			case inventoryErrors.InsufficientStock:
				return nil, api.Fail(http.StatusConflict, coreErrors.BuildErrorSingle(c, inventoryErrors.InsufficientStock))
			}
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}

	return api.OK(result)
}

// ReleaseReservedStockInput is ReleaseReservedStock's request.
type ReleaseReservedStockInput struct {
	JSON api.Body[models.ReleaseReservedStockDto]
}

// ReleaseReservedStock is POST /release-reserved: Release reserved stock.
func (ctrl *StockController) ReleaseReservedStock(ctx context.Context, in *ReleaseReservedStockInput) (*api.Out[*models.StockReservationResponse], error) {
	c := api.Gin(ctx)
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists || tenantIDRaw == nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}
	tenantID, err := uuid.Parse(tenantIDRaw.(string))
	if err != nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}

	var dto models.ReleaseReservedStockDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, inventoryErrors.InsufficientStock, err.Error()))
	}

	result, err := ctrl.stockService.ReleaseReservedStock(c.Request.Context(), tenantID, dto.ProductID, dto.Quantity, actingUser(c), dto.SkuID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Message {
			case inventoryErrors.ProductNotFound:
				return nil, api.Fail(http.StatusNotFound, coreErrors.BuildErrorSingle(c, inventoryErrors.ProductNotFound))
			case inventoryErrors.SkuNotFound:
				return nil, api.Fail(http.StatusNotFound, coreErrors.BuildErrorSingle(c, inventoryErrors.SkuNotFound))
			case inventoryErrors.ReserveSkuRequired:
				return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, inventoryErrors.ReserveSkuRequired))
			}
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}

	return api.OK(result)
}
