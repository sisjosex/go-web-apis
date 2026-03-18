package controllers

import (
	"net/http"
	"strconv"

	coreErrors "josex/web/modules/core/errors"
	"josex/web/modules/purchasing/errors"
	"josex/web/modules/purchasing/interfaces"
	"josex/web/modules/purchasing/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// PurchasingController handles HTTP requests for purchasing operations
type PurchasingController struct {
	service interfaces.PurchasingService
}

// NewPurchasingController creates a new instance
func NewPurchasingController(service interfaces.PurchasingService) *PurchasingController {
	return &PurchasingController{
		service: service,
	}
}

// ========== SUPPLIER ENDPOINTS ==========

func (ctrl *PurchasingController) CreateSupplier(c *gin.Context) {
	var dto models.CreateSupplierRequestDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, err.Error(), err))
		return
	}

	supplier, err := ctrl.service.CreateSupplier(c.Request.Context(), &dto)
	if err != nil {
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}

	response := models.CreateSupplierResponse{
		SupplierID: supplier.ID.String(),
		Name:       supplier.Name,
		Message:    "Supplier created successfully",
	}
	c.JSON(http.StatusCreated, response)
}

func (ctrl *PurchasingController) GetSupplier(c *gin.Context) {
	supplierID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, "validation.invalid-uuid"))
		return
	}

	supplier, err := ctrl.service.GetSupplier(c.Request.Context(), supplierID)
	if err != nil {
		c.JSON(http.StatusNotFound, coreErrors.BuildErrorSingle(c, errors.SupplierNotFound))
		return
	}

	c.JSON(http.StatusOK, supplier)
}

func (ctrl *PurchasingController) ListSuppliers(c *gin.Context) {
	page := 1
	pageSize := 20

	if p := c.Query("page"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
			page = parsed
		}
	}
	if ps := c.Query("page_size"); ps != "" {
		if parsed, err := strconv.Atoi(ps); err == nil && parsed > 0 && parsed <= 100 {
			pageSize = parsed
		}
	}

	suppliers, totalCount, err := ctrl.service.ListSuppliers(c.Request.Context(), page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"suppliers":   suppliers,
		"total_count": totalCount,
		"page":        page,
		"page_size":   pageSize,
	})
}

// ========== PURCHASE ORDER ENDPOINTS ==========

func (ctrl *PurchasingController) CreatePurchaseOrder(c *gin.Context) {
	var dto models.CreatePurchaseOrderRequestDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, err.Error(), err))
		return
	}

	userID := c.GetString("user_id")
	userUUID, _ := uuid.Parse(userID)
	supplierUUID, _ := uuid.Parse(dto.SupplierID)

	po, err := ctrl.service.CreatePurchaseOrder(c.Request.Context(), supplierUUID, dto.ExpectedDeliveryDate, dto.Notes, userUUID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}

	response := models.CreatePurchaseOrderResponse{
		POID:        po.ID.String(),
		PONumber:    po.PONumber,
		SupplierID:  po.SupplierID.String(),
		Status:      po.Status,
		TotalAmount: po.TotalAmount,
		Message:     "Purchase order created successfully",
	}
	c.JSON(http.StatusCreated, response)
}

func (ctrl *PurchasingController) GetPurchaseOrder(c *gin.Context) {
	poID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, "validation.invalid-uuid"))
		return
	}

	po, err := ctrl.service.GetPurchaseOrder(c.Request.Context(), poID)
	if err != nil {
		c.JSON(http.StatusNotFound, coreErrors.BuildErrorSingle(c, errors.PONotFound))
		return
	}

	c.JSON(http.StatusOK, po)
}

func (ctrl *PurchasingController) ListPurchaseOrders(c *gin.Context) {
	page := 1
	pageSize := 20
	var status *string

	if p := c.Query("page"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
			page = parsed
		}
	}
	if ps := c.Query("page_size"); ps != "" {
		if parsed, err := strconv.Atoi(ps); err == nil && parsed > 0 && parsed <= 100 {
			pageSize = parsed
		}
	}
	if s := c.Query("status"); s != "" {
		status = &s
	}

	pos, totalCount, err := ctrl.service.ListPurchaseOrders(c.Request.Context(), status, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"purchase_orders": pos,
		"total_count":     totalCount,
		"page":            page,
		"page_size":       pageSize,
	})
}

func (ctrl *PurchasingController) AddPurchaseOrderItem(c *gin.Context) {
	poID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, "validation.invalid-uuid"))
		return
	}

	var dto models.AddPurchaseOrderItemRequestDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, err.Error(), err))
		return
	}

	productUUID, _ := uuid.Parse(dto.ProductID)

	item, err := ctrl.service.AddItemToPurchaseOrder(c.Request.Context(), poID, productUUID, dto.Quantity, dto.UnitCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}

	response := models.AddPurchaseOrderItemResponse{
		ItemID:    item.ID.String(),
		PoID:      item.PurchaseOrderID.String(),
		ProductID: item.ProductID.String(),
		Quantity:  item.Quantity,
		UnitCost:  item.UnitCost,
		LineTotal: item.LineTotal,
		Message:   "Item added to purchase order",
	}
	c.JSON(http.StatusCreated, response)
}

func (ctrl *PurchasingController) ApprovePurchaseOrder(c *gin.Context) {
	poID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, "validation.invalid-uuid"))
		return
	}

	po, err := ctrl.service.ApprovePurchaseOrder(c.Request.Context(), poID)
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildError(c, err))
		return
	}

	response := models.PurchaseOrderActionResponse{
		POID:        po.ID.String(),
		PONumber:    po.PONumber,
		Status:      po.Status,
		TotalAmount: po.TotalAmount,
		Message:     "Purchase order approved",
	}
	c.JSON(http.StatusOK, response)
}

func (ctrl *PurchasingController) ReceivePurchaseOrder(c *gin.Context) {
	poID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, "validation.invalid-uuid"))
		return
	}

	var dto models.ReceivePurchaseOrderRequestDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, err.Error(), err))
		return
	}

	var receivedBy *uuid.UUID
	if dto.ReceivedBy != nil {
		if uid, err := uuid.Parse(*dto.ReceivedBy); err == nil {
			receivedBy = &uid
		}
	}

	receipt, err := ctrl.service.ReceivePurchaseOrder(c.Request.Context(), poID, receivedBy, dto.Notes)
	if err != nil {
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"receipt_id":     receipt.ID.String(),
		"receipt_number": receipt.ReceiptNumber,
		"message":        "Purchase order received",
	})
}

func (ctrl *PurchasingController) AddInvoice(c *gin.Context) {
	poID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, "validation.invalid-uuid"))
		return
	}

	var dto models.AddInvoiceRequestDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, err.Error(), err))
		return
	}

	invoice, err := ctrl.service.AddInvoice(c.Request.Context(), poID, &dto)
	if err != nil {
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusCreated, invoice)
}

func (ctrl *PurchasingController) GetPendingPayments(c *gin.Context) {
	pos, err := ctrl.service.GetPendingPayments(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"pending_payments": pos,
		"total_count":      len(pos),
	})
}

// ========== PHASE 2A: PRICE COMPARISON ==========

func (ctrl *PurchasingController) GetPriceComparison(c *gin.Context) {
	productID := c.Query("product_id")
	if productID == "" {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, errors.InvalidProductID))
		return
	}

	productUUID, err := uuid.Parse(productID)
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, "validation.invalid-uuid"))
		return
	}

	comparisons, err := ctrl.service.GetPriceComparison(c.Request.Context(), productUUID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"product_id":     productID,
		"comparisons":    comparisons,
		"best_price":     comparisons[0].UnitCost,
		"average_cost":   comparisons[0].AverageCost,
		"supplier_count": len(comparisons),
	})
}

// ========== PHASE 2A: FIFO BATCH TRACKING ==========

func (ctrl *PurchasingController) CreateProductBatch(c *gin.Context) {
	var batch models.ProductBatch
	if err := c.ShouldBindJSON(&batch); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, err.Error(), err))
		return
	}

	newBatch, err := ctrl.service.CreateProductBatch(c.Request.Context(), &batch)
	if err != nil {
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusCreated, newBatch)
}

func (ctrl *PurchasingController) GetProductBatches(c *gin.Context) {
	productID := c.Query("product_id")
	if productID == "" {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, errors.InvalidProductID))
		return
	}

	productUUID, err := uuid.Parse(productID)
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, "validation.invalid-uuid"))
		return
	}

	batches, err := ctrl.service.GetProductBatches(c.Request.Context(), productUUID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"product_id":  productID,
		"batches":     batches,
		"batch_count": len(batches),
	})
}

func (ctrl *PurchasingController) GetOldestBatchForSale(c *gin.Context) {
	productID := c.Query("product_id")
	if productID == "" {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, errors.InvalidProductID))
		return
	}

	productUUID, err := uuid.Parse(productID)
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, "validation.invalid-uuid"))
		return
	}

	batch, err := ctrl.service.GetOldestBatchForSale(c.Request.Context(), productUUID)
	if err != nil {
		c.JSON(http.StatusNotFound, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusOK, batch)
}

// ========== PHASE 2A: RFQ (REQUEST FOR QUOTE) ==========

func (ctrl *PurchasingController) CreateRFQ(c *gin.Context) {
	var dto models.CreateRFQRequestDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, err.Error(), err))
		return
	}

	rfq := &models.RequestForQuote{
		RFQNumber: "RFQ-" + uuid.New().String()[:8],
		Status:    "draft",
	}
	newRFQ, err := ctrl.service.CreateRFQ(c.Request.Context(), rfq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}

	for _, itemReq := range dto.Items {
		productUUID, err := uuid.Parse(itemReq.ProductID)
		if err != nil {
			c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, errors.InvalidProductID))
			return
		}

		item := &models.RFQItem{
			ProductID:   productUUID,
			Quantity:    itemReq.Quantity,
			Description: itemReq.Description,
		}
		_, err = ctrl.service.AddRFQItem(c.Request.Context(), newRFQ.ID, item)
		if err != nil {
			c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
			return
		}
	}

	newRFQ.Status = "sent"
	c.JSON(http.StatusCreated, newRFQ)
}

func (ctrl *PurchasingController) GetRFQResponses(c *gin.Context) {
	rfqID := c.Param("rfq_id")
	if rfqID == "" {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, "missing.rfq-id"))
		return
	}

	rfqUUID, err := uuid.Parse(rfqID)
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, "validation.invalid-uuid"))
		return
	}

	responses, err := ctrl.service.GetRFQResponses(c.Request.Context(), rfqUUID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"rfq_id":         rfqID,
		"responses":      responses,
		"response_count": len(responses),
	})
}

func (ctrl *PurchasingController) GetRFQComparison(c *gin.Context) {
	rfqID := c.Param("rfq_id")
	if rfqID == "" {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, "missing.rfq-id"))
		return
	}

	rfqUUID, err := uuid.Parse(rfqID)
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, "validation.invalid-uuid"))
		return
	}

	comparison, err := ctrl.service.GetRFQComparison(c.Request.Context(), rfqUUID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusOK, comparison)
}

func (ctrl *PurchasingController) SelectRFQResponse(c *gin.Context) {
	rfqID := c.Param("rfq_id")
	responseID := c.Param("response_id")

	if rfqID == "" || responseID == "" {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, "missing.parameters"))
		return
	}

	rfqUUID, err := uuid.Parse(rfqID)
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, "validation.invalid-uuid"))
		return
	}

	responseUUID, err := uuid.Parse(responseID)
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, "validation.invalid-uuid"))
		return
	}

	err = ctrl.service.SelectBestRFQResponse(c.Request.Context(), rfqUUID, responseUUID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":               "RFQ closed",
		"rfq_id":               rfqID,
		"selected_response_id": responseID,
	})
}
