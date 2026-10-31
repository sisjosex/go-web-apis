package controllers

import (
	"context"
	goErrors "errors"
	"net/http"

	"josex/web/modules/core/api"
	coreErrors "josex/web/modules/core/errors"
	"josex/web/modules/inventory/errors"
	"josex/web/modules/inventory/interfaces"
	"josex/web/modules/inventory/models"
	"josex/web/modules/inventory/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

type BatchController struct {
	service interfaces.BatchService
}

func NewBatchController(service interfaces.BatchService) *BatchController {
	return &BatchController{
		service: service,
	}
}

// parseSkuIDQuery reads the optional skuId filter. Absent or empty means "every
// combination"; a malformed one is the caller's error, not a silent full list.
func parseSkuIDQuery(c *gin.Context) (*uuid.UUID, bool) {
	raw := c.Query("skuId")
	if raw == "" {
		return nil, true
	}

	skuID, err := uuid.Parse(raw)
	if err != nil {
		return nil, false
	}

	return &skuID, true
}

// CreateBatchInput is CreateBatch's request.
type CreateBatchInput struct {
	JSON api.Body[models.CreateBatchDto]
}

// CreateBatch is POST /batches: Create a new batch for a product.
func (ctrl *BatchController) CreateBatch(ctx context.Context, in *CreateBatchInput) (*api.Out[*models.BatchResponse], error) {
	c := api.Gin(ctx)
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists || tenantIDRaw == nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}
	tenantID, err := uuid.Parse(tenantIDRaw.(string))
	if err != nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}

	var dto models.CreateBatchDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.error", err.Error()))
	}

	dto.ActedBy = actingUserID(c)
	batch, err := ctrl.service.CreateBatch(c.Request.Context(), tenantID, utils.TenantToday(c), &dto)
	if err != nil {
		var pgErr *pgconn.PgError
		if goErrors.As(err, &pgErr) {
			switch pgErr.Message {
			case errors.ProductNotFound:
				return nil, api.Fail(http.StatusNotFound, coreErrors.BuildErrorSingle(c, errors.ProductNotFound))
			case errors.SkuNotFound:
				return nil, api.Fail(http.StatusNotFound, coreErrors.BuildErrorSingle(c, errors.SkuNotFound))
			// INV-014 D1 — a lot on a stock-by-variant product has to name its
			// combination; falling through to the unassigned bucket is what this
			// refuses, because sp_add_order_item can never sell out of it.
			case errors.SkuRequired:
				return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, errors.SkuRequired))
			}
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}

	return api.OK(batch)
}

// GetBatchInput is GetBatch's request.
type GetBatchInput struct {
	ID string `path:"id"`
}

// GetBatch is GET /batches/{id}: Get batch details.
func (ctrl *BatchController) GetBatch(ctx context.Context, in *GetBatchInput) (*api.Out[*models.BatchResponse], error) {
	c := api.Gin(ctx)
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists || tenantIDRaw == nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}
	tenantID, err := uuid.Parse(tenantIDRaw.(string))
	if err != nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}

	batchID, err := uuid.Parse(in.ID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, errors.BatchNotFound))
	}

	batch, err := ctrl.service.GetBatch(c.Request.Context(), tenantID, utils.TenantToday(c), batchID)
	if err != nil {
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}

	if batch == nil {
		return nil, api.Fail(http.StatusNotFound, coreErrors.BuildErrorSingle(c, errors.BatchNotFound))
	}

	return api.OK(batch)
}

// ListBatchesByProductInput is ListBatchesByProduct's request.
type ListBatchesByProductInput struct {
	ProductId string `path:"productId"`
	Query     api.Query[models.ListBatchesQuery]
}

// ListBatchesByProduct is GET /batches/product/{productId}: List all batches for a product.
func (ctrl *BatchController) ListBatchesByProduct(ctx context.Context, in *ListBatchesByProductInput) (*api.Out[*models.ListBatchesResponse], error) {
	c := api.Gin(ctx)
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists || tenantIDRaw == nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}
	tenantID, err := uuid.Parse(tenantIDRaw.(string))
	if err != nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}

	productID, err := uuid.Parse(in.ProductId)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, errors.ProductNotFound))
	}

	var query models.ListBatchesQuery
	if err := in.Query.Into(ctx, &query); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.invalid", err.Error()))
	}

	batches, err := ctrl.service.ListBatchesByProduct(c.Request.Context(), tenantID, utils.TenantToday(c), productID, query)
	if err != nil {
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}

	return api.OK(batches)
}

// GetOldestBatchForSaleInput is GetOldestBatchForSale's request.
type GetOldestBatchForSaleInput struct {
	ProductId string `path:"productId"`
	// WarehouseID picks within one warehouse; none is the default (INV-027).
	WarehouseID string `query:"warehouse_id"`
}

// GetOldestBatchForSale is GET /batches/product/{productId}/oldest: Get oldest batch for sale (FIFO).
func (ctrl *BatchController) GetOldestBatchForSale(ctx context.Context, in *GetOldestBatchForSaleInput) (*api.Out[*models.BatchResponse], error) {
	c := api.Gin(ctx)
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists || tenantIDRaw == nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}
	tenantID, err := uuid.Parse(tenantIDRaw.(string))
	if err != nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}

	productID, err := uuid.Parse(in.ProductId)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, errors.ProductNotFound))
	}

	skuID, ok := parseSkuIDQuery(c)
	if !ok {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, errors.SkuNotFound))
	}

	var warehouseID *uuid.UUID
	if in.WarehouseID != "" {
		parsed, err := uuid.Parse(in.WarehouseID)
		if err != nil {
			return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
		}
		warehouseID = &parsed
	}

	batch, err := ctrl.service.GetOldestBatchForSale(c.Request.Context(), tenantID, utils.TenantToday(c), productID, skuID, warehouseID)
	if err != nil {
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}

	if batch == nil {
		return nil, api.Fail(http.StatusNotFound, coreErrors.BuildErrorSingle(c, errors.BatchNotFound))
	}

	return api.OK(batch)
}

// mapBatchWriteError turns the guards sp_update_batch and sp_void_batch raise
// into their HTTP shapes. has-movements is a 409: the request is well formed,
// it collides with state that is already booked (INV-014 D3).
func mapBatchWriteError(c *gin.Context, err error) error {
	var pgErr *pgconn.PgError
	if !goErrors.As(err, &pgErr) {
		return nil
	}

	switch pgErr.Message {
	case errors.BatchNotFound:
		return api.Fail(http.StatusNotFound, coreErrors.BuildErrorSingle(c, errors.BatchNotFound))
	case errors.BatchHasMovements:
		return api.Fail(http.StatusConflict, coreErrors.BuildErrorSingle(c, errors.BatchHasMovements))
	case errors.BatchVoided:
		return api.Fail(http.StatusConflict, coreErrors.BuildErrorSingle(c, errors.BatchVoided))
	case errors.InsufficientStock:
		return api.Fail(http.StatusConflict, coreErrors.BuildErrorSingle(c, errors.InsufficientStock))
	case errors.BatchLotNumberRequired:
		return api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, errors.BatchLotNumberRequired))
	case errors.BatchExpiryDateMustBeFuture:
		return api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, errors.BatchExpiryDateMustBeFuture))
	default:
		return nil
	}
}

// UpdateBatchInput is UpdateBatch's request.
type UpdateBatchInput struct {
	ID   string `path:"id"`
	JSON api.Body[models.UpdateBatchDto]
}

// UpdateBatch is PATCH /batches/{id}: Correct a lot.
func (ctrl *BatchController) UpdateBatch(ctx context.Context, in *UpdateBatchInput) (*api.Out[*models.BatchResponse], error) {
	c := api.Gin(ctx)
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists || tenantIDRaw == nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}
	tenantID, err := uuid.Parse(tenantIDRaw.(string))
	if err != nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}

	batchID, err := uuid.Parse(in.ID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, errors.BatchNotFound))
	}

	var dto models.UpdateBatchDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.error", err.Error()))
	}

	dto.ActedBy = actingUserID(c)
	batch, err := ctrl.service.UpdateBatch(c.Request.Context(), tenantID, utils.TenantToday(c), batchID, &dto)
	if err != nil {
		if err := mapBatchWriteError(c, err); err != nil {
			return nil, err
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}

	if batch == nil {
		return nil, api.Fail(http.StatusNotFound, coreErrors.BuildErrorSingle(c, errors.BatchNotFound))
	}

	return api.OK(batch)
}

// VoidBatchInput is VoidBatch's request.
type VoidBatchInput struct {
	ID string `path:"id"`
}

// VoidBatch is DELETE /batches/{id}: Void a lot.
func (ctrl *BatchController) VoidBatch(ctx context.Context, in *VoidBatchInput) (*api.Out[*models.BatchResponse], error) {
	c := api.Gin(ctx)
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists || tenantIDRaw == nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}
	tenantID, err := uuid.Parse(tenantIDRaw.(string))
	if err != nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}

	batchID, err := uuid.Parse(in.ID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, errors.BatchNotFound))
	}

	batch, err := ctrl.service.VoidBatch(c.Request.Context(), tenantID, batchID)
	if err != nil {
		if err := mapBatchWriteError(c, err); err != nil {
			return nil, err
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}

	if batch == nil {
		return nil, api.Fail(http.StatusNotFound, coreErrors.BuildErrorSingle(c, errors.BatchNotFound))
	}

	return api.OK(batch)
}

// RedistributeBatchesInput is RedistributeBatches's request.
type RedistributeBatchesInput struct {
	ID   string `path:"id"`
	JSON api.Body[models.RedistributeBatchesDto]
}

// RedistributeBatches is POST /products/{id}/batches/redistribute: Split a stranded lot onto real combinations.
func (ctrl *BatchController) RedistributeBatches(ctx context.Context, in *RedistributeBatchesInput) (*api.Out[*models.RedistributeBatchesResponse], error) {
	c := api.Gin(ctx)
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists || tenantIDRaw == nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}
	tenantID, err := uuid.Parse(tenantIDRaw.(string))
	if err != nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}

	productID, err := uuid.Parse(in.ID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, errors.ProductNotFound))
	}

	var dto models.RedistributeBatchesDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.error", err.Error()))
	}

	var createdBy *uuid.UUID
	if userID, parseErr := uuid.Parse(c.GetString("user_id")); parseErr == nil {
		createdBy = &userID
	}

	result, err := ctrl.service.RedistributeBatches(c.Request.Context(), tenantID, utils.TenantToday(c), productID, &dto, createdBy)
	if err != nil {
		if err := mapBatchWriteError(c, err); err != nil {
			return nil, err
		}

		var pgErr *pgconn.PgError
		if goErrors.As(err, &pgErr) {
			switch pgErr.Message {
			case errors.ProductNotFound:
				return nil, api.Fail(http.StatusNotFound, coreErrors.BuildErrorSingle(c, errors.ProductNotFound))
			case errors.SkuNotFound:
				return nil, api.Fail(http.StatusNotFound, coreErrors.BuildErrorSingle(c, errors.SkuNotFound))
			// The amounts have to add up to exactly the lot: a body that does not
			// is the caller's to fix, not a conflict with stored state.
			case errors.SkusRedistributionMismatch:
				return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, errors.SkusRedistributionMismatch))
			}
		}

		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}

	return api.OK(result)
}

// GetExpiringBatchesInput is GetExpiringBatches's request.
type GetExpiringBatchesInput struct {
	Query api.Query[models.ListExpiringBatchesQuery]
}

// GetExpiringBatches is GET /batches/expiring: Get all batches expiring soon.
func (ctrl *BatchController) GetExpiringBatches(ctx context.Context, in *GetExpiringBatchesInput) (*api.Out[*models.ListBatchesResponse], error) {
	c := api.Gin(ctx)
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists || tenantIDRaw == nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}
	tenantID, err := uuid.Parse(tenantIDRaw.(string))
	if err != nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}

	var query models.ListExpiringBatchesQuery
	if err := in.Query.Into(ctx, &query); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.invalid", err.Error()))
	}

	batches, err := ctrl.service.GetExpiringBatches(c.Request.Context(), tenantID, utils.TenantToday(c), query)
	if err != nil {
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}

	return api.OK(batches)
}
