package controllers

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"josex/web/modules/core/api"
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

// RecordMovementInput is RecordMovement's request.
type RecordMovementInput struct {
	JSON api.Body[models.RecordMovementDto]
}

// RecordMovement is POST /movements: Record an inventory movement.
func (ctrl *MovementController) RecordMovement(ctx context.Context, in *RecordMovementInput) (*api.Out[*models.RecordMovementResponse], error) {
	c := api.Gin(ctx)
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists || tenantIDRaw == nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}
	tenantID, err := uuid.Parse(tenantIDRaw.(string))
	if err != nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}

	var dto models.RecordMovementDto

	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.invalid", err.Error()))
	}

	userID := c.GetString("user_id")
	movement, err := ctrl.movementService.RecordMovement(c.Request.Context(), tenantID, dto, &userID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Message {
			case inventoryErrors.ProductNotFound:
				return nil, api.Fail(http.StatusNotFound, coreErrors.BuildErrorSingle(c, inventoryErrors.ProductNotFound))
			case inventoryErrors.WarehouseNotFound:
				return nil, api.Fail(http.StatusNotFound, coreErrors.BuildErrorSingle(c, inventoryErrors.WarehouseNotFound))
			case inventoryErrors.SkuNotFound:
				return nil, api.Fail(http.StatusNotFound, coreErrors.BuildErrorSingle(c, inventoryErrors.SkuNotFound))
			// INV-011 D1 — a movement on a stock-by-variant product has to name its
			// combination; falling back to the unassigned bucket is what this refuses.
			case inventoryErrors.MovementSkuRequired:
				return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, inventoryErrors.MovementSkuRequired))
			case inventoryErrors.StockNotFound:
				return nil, api.Fail(http.StatusNotFound, coreErrors.BuildErrorSingle(c, inventoryErrors.StockNotFound))
			case inventoryErrors.InsufficientStock:
				return nil, api.Fail(http.StatusConflict, coreErrors.BuildErrorSingle(c, inventoryErrors.InsufficientStock))
			case inventoryErrors.InvalidMovementType:
				return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, inventoryErrors.InvalidMovementType))
			// INV-013 D1/D3 — three ways the body can be internally inconsistent, all
			// of them the caller's to fix and none of them a conflict with stored
			// state, so 400 rather than the 409 insufficient-stock takes.
			case inventoryErrors.MovementDirectionRequired:
				return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, inventoryErrors.MovementDirectionRequired))
			case inventoryErrors.MovementDirectionConflict:
				return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, inventoryErrors.MovementDirectionConflict))
			case inventoryErrors.MovementUnitCostRequired:
				return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, inventoryErrors.MovementUnitCostRequired))
			}
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}

	return api.OK(movement)
}

// GetMovementInput is GetMovement's request.
type GetMovementInput struct {
	ID string `path:"id"`
}

// GetMovement is GET /movements/{id}: Get movement by ID.
func (ctrl *MovementController) GetMovement(ctx context.Context, in *GetMovementInput) (*api.Out[*models.InventoryMovement], error) {
	c := api.Gin(ctx)
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists || tenantIDRaw == nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}
	tenantID, err := uuid.Parse(tenantIDRaw.(string))
	if err != nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}

	movementID := in.ID

	movement, err := ctrl.movementService.GetMovement(c.Request.Context(), tenantID, movementID)
	if err != nil {
		// The SP answers no row for an id the tenant does not have.
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, api.Fail(http.StatusNotFound, coreErrors.BuildErrorSingle(c, inventoryErrors.MovementNotFound))
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Message {
			case inventoryErrors.MovementNotFound:
				return nil, api.Fail(http.StatusNotFound, coreErrors.BuildErrorSingle(c, inventoryErrors.MovementNotFound))
			}
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}

	return api.OK(movement)
}

// ListMovementsInput is ListMovements's request.
type ListMovementsInput struct {
	Query api.Query[models.ListMovementsQuery]
}

// ListMovements is GET /movements: List inventory movements.
func (ctrl *MovementController) ListMovements(ctx context.Context, in *ListMovementsInput) (*api.Out[*models.ListMovementsResponse], error) {
	c := api.Gin(ctx)
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists || tenantIDRaw == nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}
	tenantID, err := uuid.Parse(tenantIDRaw.(string))
	if err != nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}

	var query models.ListMovementsQuery

	if err := in.Query.Into(ctx, &query); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.invalid", err.Error()))
	}

	if query.Group == "" {
		query.Group = "none"
		if query.ProductID == nil {
			query.Group = "document"
		}
	}

	result, err := ctrl.movementService.ListMovements(c.Request.Context(), tenantID, query)
	if err != nil {
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}

	return api.OK(result)
}

// GetProductStockInput is GetProductStock's request.
type GetProductStockInput struct {
	ProductID string `path:"product_id"`
}

// GetProductStock is GET /stock/{product_id}: Get current product stock.
func (ctrl *MovementController) GetProductStock(ctx context.Context, in *GetProductStockInput) (*api.Out[*models.ProductStock], error) {
	c := api.Gin(ctx)
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists || tenantIDRaw == nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}
	tenantID, err := uuid.Parse(tenantIDRaw.(string))
	if err != nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}

	productID := in.ProductID

	stock, err := ctrl.movementService.GetProductStock(c.Request.Context(), tenantID, productID)
	if err != nil {
		// The SP answers no row for a product outside the tenant.
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, api.Fail(http.StatusNotFound, coreErrors.BuildErrorSingle(c, inventoryErrors.ProductNotFound))
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Message {
			case inventoryErrors.ProductNotFound:
				return nil, api.Fail(http.StatusNotFound, coreErrors.BuildErrorSingle(c, inventoryErrors.ProductNotFound))
			}
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}

	return api.OK(stock)
}
