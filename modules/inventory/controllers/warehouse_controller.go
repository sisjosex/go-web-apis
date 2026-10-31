package controllers

import (
	"context"
	"net/http"
	"time"

	"josex/web/modules/core/api"
	coreErrors "josex/web/modules/core/errors"
	"josex/web/modules/core/limits"
	"josex/web/modules/inventory/interfaces"
	"josex/web/modules/inventory/models"
	"josex/web/modules/inventory/utils"
)

// summaryTTL keeps the inventory home a minute per tenant and warehouse (D4).
const summaryTTL = 60 * time.Second

// WarehouseController serves warehouses, the stock across them, the inventory home and transfers (INV-025).
type WarehouseController struct {
	warehouses interfaces.WarehouseRepository
	summaries  *ttlCache[*models.InventorySummary]
}

// NewWarehouseController builds the controller.
func NewWarehouseController(warehouses interfaces.WarehouseRepository) *WarehouseController {
	return &WarehouseController{warehouses: warehouses, summaries: newTTLCache[*models.InventorySummary](summaryTTL)}
}

// ListWarehouses is GET /warehouses.
func (ctrl *WarehouseController) ListWarehouses(ctx context.Context, _ *api.None) (*api.Out[models.ListWarehousesResponse], error) {
	c := api.Gin(ctx)
	tenantID, err := requestTenant(c)
	if err != nil {
		return nil, err
	}
	rows, err := ctrl.warehouses.ListWarehouses(c.Request.Context(), tenantID, nil)
	if err != nil {
		return nil, failUnit(c, err)
	}
	return api.OK(models.ListWarehousesResponse{Warehouses: rows})
}

// CreateWarehouseInput is CreateWarehouse's request.
type CreateWarehouseInput struct {
	JSON api.Body[models.UpsertWarehouseDto]
}

// CreateWarehouse is POST /warehouses, within the plan's limit.
func (ctrl *WarehouseController) CreateWarehouse(ctx context.Context, in *CreateWarehouseInput) (*api.Out[models.Warehouse], error) {
	saved, err := ctrl.upsert(ctx, nil, &in.JSON)
	if err != nil {
		return nil, err
	}
	return api.OK(saved)
}

// UpdateWarehouseInput is UpdateWarehouse's request.
type UpdateWarehouseInput struct {
	ID   string `path:"id"`
	JSON api.Body[models.UpsertWarehouseDto]
}

// UpdateWarehouse is PUT /warehouses/{id}.
func (ctrl *WarehouseController) UpdateWarehouse(ctx context.Context, in *UpdateWarehouseInput) (*api.Out[models.Warehouse], error) {
	saved, err := ctrl.upsert(ctx, &in.ID, &in.JSON)
	if err != nil {
		return nil, err
	}
	return api.OK(saved)
}

func (ctrl *WarehouseController) upsert(ctx context.Context, id *string, body *api.Body[models.UpsertWarehouseDto]) (models.Warehouse, error) {
	c := api.Gin(ctx)
	tenantID, err := requestTenant(c)
	if err != nil {
		return models.Warehouse{}, err
	}
	var dto models.UpsertWarehouseDto
	if err := body.Into(ctx, &dto); err != nil {
		return models.Warehouse{}, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.invalid", err.Error()))
	}
	savedID, err := ctrl.warehouses.UpsertWarehouse(c.Request.Context(), tenantID, id, dto)
	if err != nil {
		if refused := limits.Refuse(c, err); refused != nil {
			return models.Warehouse{}, refused
		}
		return models.Warehouse{}, failUnit(c, err)
	}
	rows, err := ctrl.warehouses.ListWarehouses(c.Request.Context(), tenantID, &savedID)
	if err != nil || len(rows) == 0 {
		return models.Warehouse{}, failUnit(c, err)
	}
	return rows[0], nil
}

// SetWarehouseStatusInput is SetWarehouseStatus's request.
type SetWarehouseStatusInput struct {
	ID   string `path:"id"`
	JSON api.Body[models.SetWarehouseStatusDto]
}

// WarehouseChanged confirms a change.
type WarehouseChanged struct {
	Message string `json:"message"`
}

// SetWarehouseStatus is PATCH /warehouses/{id}/status.
func (ctrl *WarehouseController) SetWarehouseStatus(ctx context.Context, in *SetWarehouseStatusInput) (*api.Out[WarehouseChanged], error) {
	c := api.Gin(ctx)
	tenantID, err := requestTenant(c)
	if err != nil {
		return nil, err
	}
	var dto models.SetWarehouseStatusDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.invalid", err.Error()))
	}
	if err := ctrl.warehouses.SetWarehouseStatus(c.Request.Context(), tenantID, in.ID, dto.Status); err != nil {
		return nil, failUnit(c, err)
	}
	return api.OK(WarehouseChanged{Message: "Warehouse updated"})
}

// WarehouseIDInput names one warehouse.
type WarehouseIDInput struct {
	ID string `path:"id"`
}

// SetDefaultWarehouse is PATCH /warehouses/{id}/default.
func (ctrl *WarehouseController) SetDefaultWarehouse(ctx context.Context, in *WarehouseIDInput) (*api.Out[WarehouseChanged], error) {
	c := api.Gin(ctx)
	tenantID, err := requestTenant(c)
	if err != nil {
		return nil, err
	}
	if err := ctrl.warehouses.SetDefaultWarehouse(c.Request.Context(), tenantID, in.ID); err != nil {
		return nil, failUnit(c, err)
	}
	return api.OK(WarehouseChanged{Message: "Warehouse updated"})
}

// ListStockInput is ListStock's request.
type ListStockInput struct {
	Query api.Query[models.ListStockQuery]
}

// ListStock is GET /stock: every product across warehouses.
func (ctrl *WarehouseController) ListStock(ctx context.Context, in *ListStockInput) (*api.Out[*models.ListStockResponse], error) {
	c := api.Gin(ctx)
	tenantID, err := requestTenant(c)
	if err != nil {
		return nil, err
	}
	var query models.ListStockQuery
	if err := in.Query.Into(ctx, &query); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.invalid", err.Error()))
	}
	out, err := ctrl.warehouses.ListStock(c.Request.Context(), tenantID, utils.TenantToday(c), query)
	if err != nil {
		return nil, failUnit(c, err)
	}
	return api.OK(out)
}

// ListStockSkusInput is ListStockSkus's request.
type ListStockSkusInput struct {
	ProductID string `path:"product_id"`
}

// ListStockSkus is GET /stock/{product_id}/skus.
func (ctrl *WarehouseController) ListStockSkus(ctx context.Context, in *ListStockSkusInput) (*api.Out[models.ListStockSkusResponse], error) {
	c := api.Gin(ctx)
	tenantID, err := requestTenant(c)
	if err != nil {
		return nil, err
	}
	rows, err := ctrl.warehouses.ListStockSkus(c.Request.Context(), tenantID, in.ProductID)
	if err != nil {
		return nil, failUnit(c, err)
	}
	return api.OK(models.ListStockSkusResponse{Skus: rows})
}

// SummaryInput is Summary's request.
type SummaryInput struct {
	WarehouseID string `query:"warehouse_id" doc:"One warehouse; none is all of them"`
}

// Summary is GET /summary, the inventory home, kept a minute per tenant and warehouse.
func (ctrl *WarehouseController) Summary(ctx context.Context, in *SummaryInput) (*api.Out[*models.InventorySummary], error) {
	c := api.Gin(ctx)
	tenantID, err := requestTenant(c)
	if err != nil {
		return nil, err
	}
	key := tenantID.String() + "|" + in.WarehouseID
	if cached, hit := ctrl.summaries.get(key); hit {
		return api.OK(cached)
	}
	var warehouseID *string
	if in.WarehouseID != "" {
		warehouseID = &in.WarehouseID
	}
	out, err := ctrl.warehouses.Summary(c.Request.Context(), tenantID, utils.TenantToday(c), warehouseID)
	if err != nil {
		return nil, failUnit(c, err)
	}
	ctrl.summaries.set(key, out)
	return api.OK(out)
}

// NavBadges is GET /nav-badges.
func (ctrl *WarehouseController) NavBadges(ctx context.Context, _ *api.None) (*api.Out[*models.InventoryNavBadges], error) {
	c := api.Gin(ctx)
	tenantID, err := requestTenant(c)
	if err != nil {
		return nil, err
	}
	out, err := ctrl.warehouses.NavBadges(c.Request.Context(), tenantID)
	if err != nil {
		return nil, failUnit(c, err)
	}
	return api.OK(out)
}

// CreateTransferInput is CreateTransfer's request.
type CreateTransferInput struct {
	JSON api.Body[models.CreateTransferDto]
}

// CreateTransfer is POST /transfers: prepared, its lines reserved at the origin.
func (ctrl *WarehouseController) CreateTransfer(ctx context.Context, in *CreateTransferInput) (*api.Out[*models.Transfer], error) {
	c := api.Gin(ctx)
	tenantID, err := requestTenant(c)
	if err != nil {
		return nil, err
	}
	var dto models.CreateTransferDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.invalid", err.Error()))
	}
	id, err := ctrl.warehouses.CreateTransfer(c.Request.Context(), tenantID, dto, actingUser(c))
	if err != nil {
		return nil, failUnit(c, err)
	}
	return ctrl.transfer(ctx, id)
}

func (ctrl *WarehouseController) transfer(ctx context.Context, id string) (*api.Out[*models.Transfer], error) {
	c := api.Gin(ctx)
	tenantID, err := requestTenant(c)
	if err != nil {
		return nil, err
	}
	out, err := ctrl.warehouses.GetTransfer(c.Request.Context(), tenantID, id)
	if err != nil {
		return nil, failUnit(c, err)
	}
	return api.OK(out)
}

// TransferIDInput names one transfer.
type TransferIDInput struct {
	ID string `path:"id"`
}

// GetTransfer is GET /transfers/{id}.
func (ctrl *WarehouseController) GetTransfer(ctx context.Context, in *TransferIDInput) (*api.Out[*models.Transfer], error) {
	return ctrl.transfer(ctx, in.ID)
}

// ListTransfersInput is ListTransfers's request.
type ListTransfersInput struct {
	Query api.Query[models.ListTransfersQuery]
}

// ListTransfers is GET /transfers.
func (ctrl *WarehouseController) ListTransfers(ctx context.Context, in *ListTransfersInput) (*api.Out[*models.ListTransfersResponse], error) {
	c := api.Gin(ctx)
	tenantID, err := requestTenant(c)
	if err != nil {
		return nil, err
	}
	var query models.ListTransfersQuery
	if err := in.Query.Into(ctx, &query); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.invalid", err.Error()))
	}
	out, err := ctrl.warehouses.ListTransfers(c.Request.Context(), tenantID, query)
	if err != nil {
		return nil, failUnit(c, err)
	}
	return api.OK(out)
}

// ShipTransferInput is ShipTransfer's request.
type ShipTransferInput struct {
	ID   string `path:"id"`
	JSON api.Body[models.ShipTransferDto]
}

// ShipTransfer is POST /transfers/{id}/ship.
func (ctrl *WarehouseController) ShipTransfer(ctx context.Context, in *ShipTransferInput) (*api.Out[*models.Transfer], error) {
	c := api.Gin(ctx)
	tenantID, err := requestTenant(c)
	if err != nil {
		return nil, err
	}
	var dto models.ShipTransferDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.invalid", err.Error()))
	}
	if err := ctrl.warehouses.ShipTransfer(c.Request.Context(), tenantID, in.ID, dto.CarrierNote, actingUser(c)); err != nil {
		return nil, failUnit(c, err)
	}
	return ctrl.transfer(ctx, in.ID)
}

// ReceiveTransferInput is ReceiveTransfer's request.
type ReceiveTransferInput struct {
	ID   string `path:"id"`
	JSON api.Body[models.ReceiveTransferDto]
}

// ReceiveTransfer is POST /transfers/{id}/receive.
func (ctrl *WarehouseController) ReceiveTransfer(ctx context.Context, in *ReceiveTransferInput) (*api.Out[*models.Transfer], error) {
	c := api.Gin(ctx)
	tenantID, err := requestTenant(c)
	if err != nil {
		return nil, err
	}
	var dto models.ReceiveTransferDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.invalid", err.Error()))
	}
	if err := ctrl.warehouses.ReceiveTransfer(c.Request.Context(), tenantID, in.ID, dto.Lines, actingUser(c)); err != nil {
		return nil, failUnit(c, err)
	}
	return ctrl.transfer(ctx, in.ID)
}

// CancelTransfer is POST /transfers/{id}/cancel.
func (ctrl *WarehouseController) CancelTransfer(ctx context.Context, in *TransferIDInput) (*api.Out[*models.Transfer], error) {
	c := api.Gin(ctx)
	tenantID, err := requestTenant(c)
	if err != nil {
		return nil, err
	}
	if err := ctrl.warehouses.CancelTransfer(c.Request.Context(), tenantID, in.ID, actingUser(c)); err != nil {
		return nil, failUnit(c, err)
	}
	return ctrl.transfer(ctx, in.ID)
}
