package controllers

import (
	goErrors "errors"
	"net/http"
	"strconv"

	coreErrors "josex/web/modules/core/errors"
	"josex/web/modules/inventory/errors"
	"josex/web/modules/inventory/interfaces"
	"josex/web/modules/inventory/models"

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

// CreateBatch godoc
// @Summary Create a new batch for a product
// @Description Create a new inventory batch with lot number and expiry date
// @Tags Batches
// @Accept json
// @Produce json
// @Param batch body models.CreateBatchDto true "Batch data"
// @Success 201 {object} models.BatchResponse
// @Failure 400 {object} map[string]interface{} "Invalid request"
// @Failure 404 {object} map[string]interface{} "Product not found"
// @Router /batches [post]
func (ctrl *BatchController) CreateBatch(c *gin.Context) {
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

	var dto models.CreateBatchDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.error", err.Error()))
		return
	}

	batch, err := ctrl.service.CreateBatch(c.Request.Context(), tenantID, &dto)
	if err != nil {
		var pgErr *pgconn.PgError
		if goErrors.As(err, &pgErr) {
			switch pgErr.Message {
			case errors.ProductNotFound:
				c.JSON(http.StatusNotFound, coreErrors.BuildErrorSingle(c, errors.ProductNotFound))
				return
			case errors.SkuNotFound:
				c.JSON(http.StatusNotFound, coreErrors.BuildErrorSingle(c, errors.SkuNotFound))
				return
			// INV-014 D1 — a lot on a stock-by-variant product has to name its
			// combination; falling through to the unassigned bucket is what this
			// refuses, because sp_add_order_item can never sell out of it.
			case errors.SkuRequired:
				c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, errors.SkuRequired))
				return
			}
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusCreated, batch)
}

// GetBatch godoc
// @Summary Get batch details
// @Description Get detailed information about a specific batch
// @Tags Batches
// @Accept json
// @Produce json
// @Param id path string true "Batch ID"
// @Success 200 {object} models.BatchResponse
// @Failure 404 {object} map[string]interface{} "Batch not found"
// @Router /batches/{id} [get]
func (ctrl *BatchController) GetBatch(c *gin.Context) {
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

	batchID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, errors.BatchNotFound))
		return
	}

	batch, err := ctrl.service.GetBatch(c.Request.Context(), tenantID, batchID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}

	if batch == nil {
		c.JSON(http.StatusNotFound, coreErrors.BuildErrorSingle(c, errors.BatchNotFound))
		return
	}

	c.JSON(http.StatusOK, batch)
}

// ListBatchesByProduct godoc
// @Summary List all batches for a product
// @Description Get all batches for a specific product, ordered by expiry date (FIFO)
// @Tags Batches
// @Accept json
// @Produce json
// @Param productId path string true "Product ID"
// @Param onlyActive query bool false "Only active batches (default: true)"
// @Param skuId query string false "Only batches of this combination"
// @Success 200 {object} models.ListBatchesResponse
// @Failure 400 {object} map[string]interface{} "Invalid request"
// @Router /products/{productId}/batches [get]
func (ctrl *BatchController) ListBatchesByProduct(c *gin.Context) {
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

	productID, err := uuid.Parse(c.Param("productId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, errors.ProductNotFound))
		return
	}

	onlyActive := true
	if c.Query("onlyActive") == "false" {
		onlyActive = false
	}

	skuID, ok := parseSkuIDQuery(c)
	if !ok {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, errors.SkuNotFound))
		return
	}

	batches, err := ctrl.service.ListBatchesByProduct(c.Request.Context(), tenantID, productID, onlyActive, skuID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusOK, batches)
}

// GetOldestBatchForSale godoc
// @Summary Get oldest batch for sale (FIFO)
// @Description Get the oldest active batch for a product, used for FIFO sales
// @Tags Batches
// @Accept json
// @Produce json
// @Param productId path string true "Product ID"
// @Param skuId query string false "Combination to pick from (default: the product SKU)"
// @Success 200 {object} models.BatchResponse
// @Failure 404 {object} map[string]interface{} "No active batches found"
// @Router /products/{productId}/batches/oldest [get]
func (ctrl *BatchController) GetOldestBatchForSale(c *gin.Context) {
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

	productID, err := uuid.Parse(c.Param("productId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, errors.ProductNotFound))
		return
	}

	skuID, ok := parseSkuIDQuery(c)
	if !ok {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, errors.SkuNotFound))
		return
	}

	batch, err := ctrl.service.GetOldestBatchForSale(c.Request.Context(), tenantID, productID, skuID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}

	if batch == nil {
		c.JSON(http.StatusNotFound, coreErrors.BuildErrorSingle(c, errors.BatchNotFound))
		return
	}

	c.JSON(http.StatusOK, batch)
}

// mapBatchWriteError turns the guards sp_update_batch and sp_void_batch raise
// into their HTTP shapes. has-movements is a 409: the request is well formed,
// it collides with state that is already booked (INV-014 D3).
func mapBatchWriteError(c *gin.Context, err error) bool {
	var pgErr *pgconn.PgError
	if !goErrors.As(err, &pgErr) {
		return false
	}

	switch pgErr.Message {
	case errors.BatchNotFound:
		c.JSON(http.StatusNotFound, coreErrors.BuildErrorSingle(c, errors.BatchNotFound))
	case errors.BatchHasMovements:
		c.JSON(http.StatusConflict, coreErrors.BuildErrorSingle(c, errors.BatchHasMovements))
	case errors.BatchVoided:
		c.JSON(http.StatusConflict, coreErrors.BuildErrorSingle(c, errors.BatchVoided))
	case errors.BatchLotNumberRequired:
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, errors.BatchLotNumberRequired))
	case errors.BatchExpiryDateMustBeFuture:
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, errors.BatchExpiryDateMustBeFuture))
	default:
		return false
	}

	return true
}

// UpdateBatch godoc
// @Summary Correct a lot
// @Description Correct a lot's lot number and expiry date, while it has no movements
// @Tags Batches
// @Accept json
// @Produce json
// @Param id path string true "Batch ID"
// @Param batch body models.UpdateBatchDto true "Fields to correct"
// @Success 200 {object} models.BatchResponse
// @Failure 400 {object} map[string]interface{} "Invalid request"
// @Failure 404 {object} map[string]interface{} "Batch not found"
// @Failure 409 {object} map[string]interface{} "Batch already consumed or voided"
// @Router /batches/{id} [patch]
func (ctrl *BatchController) UpdateBatch(c *gin.Context) {
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

	batchID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, errors.BatchNotFound))
		return
	}

	var dto models.UpdateBatchDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.error", err.Error()))
		return
	}

	batch, err := ctrl.service.UpdateBatch(c.Request.Context(), tenantID, batchID, &dto)
	if err != nil {
		if mapBatchWriteError(c, err) {
			return
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}

	if batch == nil {
		c.JSON(http.StatusNotFound, coreErrors.BuildErrorSingle(c, errors.BatchNotFound))
		return
	}

	c.JSON(http.StatusOK, batch)
}

// VoidBatch godoc
// @Summary Void a lot
// @Description Write a lot off by setting its status to void. Never deletes the row.
// @Tags Batches
// @Accept json
// @Produce json
// @Param id path string true "Batch ID"
// @Success 200 {object} models.BatchResponse
// @Failure 404 {object} map[string]interface{} "Batch not found"
// @Failure 409 {object} map[string]interface{} "Batch already consumed or voided"
// @Router /batches/{id} [delete]
func (ctrl *BatchController) VoidBatch(c *gin.Context) {
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

	batchID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, errors.BatchNotFound))
		return
	}

	batch, err := ctrl.service.VoidBatch(c.Request.Context(), tenantID, batchID)
	if err != nil {
		if mapBatchWriteError(c, err) {
			return
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}

	if batch == nil {
		c.JSON(http.StatusNotFound, coreErrors.BuildErrorSingle(c, errors.BatchNotFound))
		return
	}

	c.JSON(http.StatusOK, batch)
}

// RedistributeBatches godoc
// @Summary Split a stranded lot onto real combinations
// @Description Split a default-SKU lot into per-combination lots keeping its lot number, dates and unit cost
// @Tags Batches
// @Accept json
// @Produce json
// @Param id path string true "Product ID"
// @Param redistribution body models.RedistributeBatchesDto true "Lot and targets"
// @Success 200 {object} models.RedistributeBatchesResponse
// @Failure 400 {object} map[string]interface{} "Amounts do not add up, or a target is not a combination"
// @Failure 404 {object} map[string]interface{} "Batch not found"
// @Failure 409 {object} map[string]interface{} "Batch already consumed or voided"
// @Router /products/{id}/batches/redistribute [post]
func (ctrl *BatchController) RedistributeBatches(c *gin.Context) {
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

	productID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, errors.ProductNotFound))
		return
	}

	var dto models.RedistributeBatchesDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.error", err.Error()))
		return
	}

	var createdBy *uuid.UUID
	if userID, parseErr := uuid.Parse(c.GetString("user_id")); parseErr == nil {
		createdBy = &userID
	}

	result, err := ctrl.service.RedistributeBatches(c.Request.Context(), tenantID, productID, &dto, createdBy)
	if err != nil {
		if mapBatchWriteError(c, err) {
			return
		}

		var pgErr *pgconn.PgError
		if goErrors.As(err, &pgErr) {
			switch pgErr.Message {
			case errors.ProductNotFound:
				c.JSON(http.StatusNotFound, coreErrors.BuildErrorSingle(c, errors.ProductNotFound))
				return
			case errors.SkuNotFound:
				c.JSON(http.StatusNotFound, coreErrors.BuildErrorSingle(c, errors.SkuNotFound))
				return
			// The amounts have to add up to exactly the lot: a body that does not
			// is the caller's to fix, not a conflict with stored state.
			case errors.SkusRedistributionMismatch:
				c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, errors.SkusRedistributionMismatch))
				return
			}
		}

		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusOK, result)
}

// GetExpiringBatches godoc
// @Summary Get all batches expiring soon
// @Description Get all batches that will expire within the specified warning days
// @Tags Batches
// @Accept json
// @Produce json
// @Param warningDays query int false "Number of days to check for expiry (default: 30)"
// @Success 200 {array} models.BatchResponse
// @Failure 400 {object} map[string]interface{} "Invalid request"
// @Router /batches/expiring [get]
func (ctrl *BatchController) GetExpiringBatches(c *gin.Context) {
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

	warningDays := 30 // default value
	if days := c.Query("warningDays"); days != "" {
		parsedDays, err := strconv.Atoi(days)
		if err != nil {
			c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.error", "warningDays must be a valid integer"))
			return
		}
		if parsedDays <= 0 {
			c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.error", "warningDays must be greater than 0"))
			return
		}
		warningDays = parsedDays
	}

	batches, err := ctrl.service.GetExpiringBatches(c.Request.Context(), tenantID, warningDays)
	if err != nil {
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusOK, batches)
}
