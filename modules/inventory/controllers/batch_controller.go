package controllers

import (
	"net/http"
	"strconv"

	coreErrors "josex/web/modules/core/errors"
	"josex/web/modules/inventory/errors"
	"josex/web/modules/inventory/interfaces"
	"josex/web/modules/inventory/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type BatchController struct {
	service interfaces.BatchService
}

func NewBatchController(service interfaces.BatchService) *BatchController {
	return &BatchController{
		service: service,
	}
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
	var dto models.CreateBatchDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.error", err.Error()))
		return
	}

	batch, err := ctrl.service.CreateBatch(c.Request.Context(), &dto)
	if err != nil {
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
	batchID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, errors.BatchNotFound))
		return
	}

	batch, err := ctrl.service.GetBatch(c.Request.Context(), batchID)
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
// @Success 200 {object} models.ListBatchesResponse
// @Failure 400 {object} map[string]interface{} "Invalid request"
// @Router /products/{productId}/batches [get]
func (ctrl *BatchController) ListBatchesByProduct(c *gin.Context) {
	productID, err := uuid.Parse(c.Param("productId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, errors.ProductNotFound))
		return
	}

	onlyActive := true
	if c.Query("onlyActive") == "false" {
		onlyActive = false
	}

	batches, err := ctrl.service.ListBatchesByProduct(c.Request.Context(), productID, onlyActive)
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
// @Success 200 {object} models.BatchResponse
// @Failure 404 {object} map[string]interface{} "No active batches found"
// @Router /products/{productId}/batches/oldest [get]
func (ctrl *BatchController) GetOldestBatchForSale(c *gin.Context) {
	productID, err := uuid.Parse(c.Param("productId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, errors.ProductNotFound))
		return
	}

	batch, err := ctrl.service.GetOldestBatchForSale(c.Request.Context(), productID)
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

	batches, err := ctrl.service.GetExpiringBatches(c.Request.Context(), warningDays)
	if err != nil {
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusOK, batches)
}
