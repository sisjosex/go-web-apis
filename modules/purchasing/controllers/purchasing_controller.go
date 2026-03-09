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

// CreateSupplier godoc
// @Summary Create a new supplier
// @Description Create a new supplier for the tenant
// @Tags Suppliers
// @Accept json
// @Produce json
// @Param request body models.CreateSupplierRequestDto true "Supplier data"
// @Success 201 {object} models.CreateSupplierResponse
// @Failure 400 {object} map[string]interface{}
// @Router /purchasing/suppliers [post]
func (ctrl *PurchasingController) CreateSupplier(c *gin.Context) {
	var dto models.CreateSupplierRequestDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, err.Error(), err))
		return
	}

	tenantID := c.GetString("tenant_id")
	tenantUUID, err := uuid.Parse(tenantID)
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, "invalid.tenant-id"))
		return
	}

	supplier, err := ctrl.service.CreateSupplier(c.Request.Context(), tenantUUID, &dto)
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

// GetSupplier godoc
// @Summary Get supplier details
// @Tags Suppliers
// @Param id path string true "Supplier ID"
// @Success 200 {object} models.Supplier
// @Failure 404 {object} map[string]interface{}
// @Router /purchasing/suppliers/{id} [get]
func (ctrl *PurchasingController) GetSupplier(c *gin.Context) {
	supplierID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, "validation.invalid-uuid"))
		return
	}

	tenantID := c.GetString("tenant_id")
	tenantUUID, _ := uuid.Parse(tenantID)

	supplier, err := ctrl.service.GetSupplier(c.Request.Context(), supplierID, tenantUUID)
	if err != nil {
		c.JSON(http.StatusNotFound, coreErrors.BuildErrorSingle(c, errors.SupplierNotFound))
		return
	}

	c.JSON(http.StatusOK, supplier)
}

// ListSuppliers godoc
// @Summary List all suppliers
// @Tags Suppliers
// @Param page query int false "Page number"
// @Param page_size query int false "Page size"
// @Success 200 {object} map[string]interface{}
// @Router /purchasing/suppliers [get]
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

	tenantID := c.GetString("tenant_id")
	tenantUUID, _ := uuid.Parse(tenantID)

	suppliers, totalCount, err := ctrl.service.ListSuppliers(c.Request.Context(), tenantUUID, page, pageSize)
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

// CreatePurchaseOrder godoc
// @Summary Create a new purchase order
// @Tags Purchase Orders
// @Accept json
// @Produce json
// @Param request body models.CreatePurchaseOrderRequestDto true "PO data"
// @Success 201 {object} models.CreatePurchaseOrderResponse
// @Router /purchasing/purchase-orders [post]
func (ctrl *PurchasingController) CreatePurchaseOrder(c *gin.Context) {
	var dto models.CreatePurchaseOrderRequestDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, err.Error(), err))
		return
	}

	tenantID := c.GetString("tenant_id")
	userID := c.GetString("user_id")
	tenantUUID, _ := uuid.Parse(tenantID)
	userUUID, _ := uuid.Parse(userID)
	supplierUUID, _ := uuid.Parse(dto.SupplierID)

	po, err := ctrl.service.CreatePurchaseOrder(c.Request.Context(), tenantUUID, supplierUUID, dto.ExpectedDeliveryDate, dto.Notes, userUUID)
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

// GetPurchaseOrder godoc
// @Summary Get purchase order details
// @Tags Purchase Orders
// @Param id path string true "Purchase Order ID"
// @Success 200 {object} models.PurchaseOrder
// @Router /purchasing/purchase-orders/{id} [get]
func (ctrl *PurchasingController) GetPurchaseOrder(c *gin.Context) {
	poID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, "validation.invalid-uuid"))
		return
	}

	tenantID := c.GetString("tenant_id")
	tenantUUID, _ := uuid.Parse(tenantID)

	po, err := ctrl.service.GetPurchaseOrder(c.Request.Context(), poID, tenantUUID)
	if err != nil {
		c.JSON(http.StatusNotFound, coreErrors.BuildErrorSingle(c, errors.PONotFound))
		return
	}

	c.JSON(http.StatusOK, po)
}

// ListPurchaseOrders godoc
// @Summary List purchase orders
// @Tags Purchase Orders
// @Param status query string false "Filter by status"
// @Param page query int false "Page number"
// @Param page_size query int false "Page size"
// @Success 200 {object} map[string]interface{}
// @Router /purchasing/purchase-orders [get]
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

	tenantID := c.GetString("tenant_id")
	tenantUUID, _ := uuid.Parse(tenantID)

	pos, totalCount, err := ctrl.service.ListPurchaseOrders(c.Request.Context(), tenantUUID, status, page, pageSize)
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

// AddPurchaseOrderItem godoc
// @Summary Add item to purchase order
// @Tags Purchase Orders
// @Accept json
// @Param id path string true "Purchase Order ID"
// @Param request body models.AddPurchaseOrderItemRequestDto true "Item data"
// @Success 201 {object} models.AddPurchaseOrderItemResponse
// @Router /purchasing/purchase-orders/{id}/items [post]
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

// ApprovePurchaseOrder godoc
// @Summary Approve a purchase order
// @Tags Purchase Orders
// @Param id path string true "Purchase Order ID"
// @Success 200 {object} models.PurchaseOrderActionResponse
// @Router /purchasing/purchase-orders/{id}/approve [patch]
func (ctrl *PurchasingController) ApprovePurchaseOrder(c *gin.Context) {
	poID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, "validation.invalid-uuid"))
		return
	}

	tenantID := c.GetString("tenant_id")
	tenantUUID, _ := uuid.Parse(tenantID)

	po, err := ctrl.service.ApprovePurchaseOrder(c.Request.Context(), poID, tenantUUID)
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

// ReceivePurchaseOrder godoc
// @Summary Mark purchase order as received
// @Tags Purchase Orders
// @Accept json
// @Param id path string true "Purchase Order ID"
// @Param request body models.ReceivePurchaseOrderRequestDto true "Receipt data"
// @Success 200 {object} models.PurchaseOrderActionResponse
// @Router /purchasing/purchase-orders/{id}/receive [patch]
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

// AddInvoice godoc
// @Summary Add supplier invoice to PO
// @Tags Purchase Orders
// @Accept json
// @Param id path string true "Purchase Order ID"
// @Param request body models.AddInvoiceRequestDto true "Invoice data"
// @Success 201 {object} models.PurchaseOrderInvoice
// @Router /purchasing/purchase-orders/{id}/invoices [post]
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

// GetPendingPayments godoc
// @Summary Get all pending payments (accounts payable)
// @Tags Reports
// @Success 200 {object} map[string]interface{}
// @Router /purchasing/pending-payments [get]
func (ctrl *PurchasingController) GetPendingPayments(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	tenantUUID, _ := uuid.Parse(tenantID)

	pos, err := ctrl.service.GetPendingPayments(c.Request.Context(), tenantUUID)
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

// GetPriceComparison godoc
// @Summary Compare supplier prices for a product
// @Description Get all suppliers' prices for a specific product
// @Tags Reports
// @Param product_id query string true "Product ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /purchasing/price-comparison [get]
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

	tenantID := c.GetString("tenant_id")
	tenantUUID, _ := uuid.Parse(tenantID)

	comparisons, err := ctrl.service.GetPriceComparison(c.Request.Context(), tenantUUID, productUUID)
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

// CreateProductBatch godoc
// @Summary Create a new product batch
// @Description Record a new batch for inventory/FIFO tracking
// @Tags Inventory
// @Accept json
// @Produce json
// @Param request body models.ProductBatch true "Batch data"
// @Success 201 {object} models.ProductBatch
// @Failure 400 {object} map[string]interface{}
// @Router /purchasing/batches [post]
func (ctrl *PurchasingController) CreateProductBatch(c *gin.Context) {
	var batch models.ProductBatch
	if err := c.ShouldBindJSON(&batch); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, err.Error(), err))
		return
	}

	tenantID := c.GetString("tenant_id")
	tenantUUID, _ := uuid.Parse(tenantID)

	newBatch, err := ctrl.service.CreateProductBatch(c.Request.Context(), tenantUUID, &batch)
	if err != nil {
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusCreated, newBatch)
}

// GetProductBatches godoc
// @Summary Get all batches for a product
// @Tags Inventory
// @Param product_id query string true "Product ID"
// @Success 200 {object} map[string]interface{}
// @Router /purchasing/batches [get]
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

	tenantID := c.GetString("tenant_id")
	tenantUUID, _ := uuid.Parse(tenantID)

	batches, err := ctrl.service.GetProductBatches(c.Request.Context(), tenantUUID, productUUID)
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

// GetOldestBatchForSale godoc
// @Summary Get oldest batch with inventory (FIFO)
// @Tags Inventory
// @Param product_id query string true "Product ID"
// @Success 200 {object} models.ProductBatch
// @Failure 404 {object} map[string]interface{}
// @Router /purchasing/batches/oldest [get]
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

	tenantID := c.GetString("tenant_id")
	tenantUUID, _ := uuid.Parse(tenantID)

	batch, err := ctrl.service.GetOldestBatchForSale(c.Request.Context(), tenantUUID, productUUID)
	if err != nil {
		c.JSON(http.StatusNotFound, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusOK, batch)
}

// ========== PHASE 2A: RFQ (REQUEST FOR QUOTE) ==========

// CreateRFQ godoc
// @Summary Create a new Request for Quote
// @Description Submit RFQ to multiple suppliers
// @Tags RFQ
// @Accept json
// @Produce json
// @Param request body models.CreateRFQRequestDto true "RFQ data"
// @Success 201 {object} models.RequestForQuote
// @Failure 400 {object} map[string]interface{}
// @Router /purchasing/rfq [post]
func (ctrl *PurchasingController) CreateRFQ(c *gin.Context) {
	var dto models.CreateRFQRequestDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, err.Error(), err))
		return
	}

	tenantID := c.GetString("tenant_id")
	tenantUUID, _ := uuid.Parse(tenantID)

	// Create RFQ
	rfq := &models.RequestForQuote{
		RFQNumber: "RFQ-" + uuid.New().String()[:8],
		Status:    "draft",
	}
	newRFQ, err := ctrl.service.CreateRFQ(c.Request.Context(), tenantUUID, rfq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}

	// Add items
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

	// Send to suppliers (mark as sent)
	newRFQ.Status = "sent"

	c.JSON(http.StatusCreated, newRFQ)
}

// GetRFQResponses godoc
// @Summary Get all responses for an RFQ
// @Tags RFQ
// @Param rfq_id path string true "RFQ ID"
// @Success 200 {object} map[string]interface{}
// @Router /purchasing/rfq/{rfq_id}/responses [get]
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

// GetRFQComparison godoc
// @Summary Get side-by-side RFQ comparison
// @Tags RFQ
// @Param rfq_id path string true "RFQ ID"
// @Success 200 {object} map[string]interface{}
// @Router /purchasing/rfq/{rfq_id}/comparison [get]
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

// SelectRFQResponse godoc
// @Summary Select the best RFQ response and close RFQ
// @Tags RFQ
// @Param rfq_id path string true "RFQ ID"
// @Param response_id path string true "Response ID"
// @Success 200 {object} map[string]interface{}
// @Router /purchasing/rfq/{rfq_id}/responses/{response_id}/select [patch]
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

	tenantID := c.GetString("tenant_id")
	tenantUUID, _ := uuid.Parse(tenantID)

	err = ctrl.service.SelectBestRFQResponse(c.Request.Context(), rfqUUID, responseUUID, tenantUUID)
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
