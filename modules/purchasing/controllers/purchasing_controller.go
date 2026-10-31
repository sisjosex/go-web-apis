package controllers

import (
	"context"
	goerrors "errors"
	"net/http"
	"strconv"

	"josex/web/modules/core/api"
	coreErrors "josex/web/modules/core/errors"
	"josex/web/modules/core/utils"
	inventoryErrors "josex/web/modules/inventory/errors"
	purchasingErrors "josex/web/modules/purchasing/errors"
	purchasingInterfaces "josex/web/modules/purchasing/interfaces"
	"josex/web/modules/purchasing/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

// PurchasingController handles HTTP requests for purchasing operations
type PurchasingController struct {
	service purchasingInterfaces.PurchasingService
}

// NewPurchasingController creates a new instance
func NewPurchasingController(service purchasingInterfaces.PurchasingService) *PurchasingController {
	return &PurchasingController{
		service: service,
	}
}

// ========== SUPPLIER ENDPOINTS ==========

// CreateSupplierInput is CreateSupplier's request.
type CreateSupplierInput struct {
	JSON api.Body[models.CreateSupplierRequestDto]
}

func (ctrl *PurchasingController) CreateSupplier(ctx context.Context, in *CreateSupplierInput) (*api.Out[models.CreateSupplierResponse], error) {
	c := api.Gin(ctx)
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists || tenantIDRaw == nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}
	tenantID, err := uuid.Parse(tenantIDRaw.(string))
	if err != nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}

	var dto models.CreateSupplierRequestDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, purchasingErrors.SupplierValidationFailed, utils.ExtractValidationError(c, err)))
	}

	supplier, err := ctrl.service.CreateSupplier(c.Request.Context(), tenantID, &dto)
	if err != nil {
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}

	response := models.CreateSupplierResponse{
		SupplierID: supplier.ID.String(),
		Name:       supplier.Name,
		Message:    "Supplier created successfully",
	}
	return api.OK(response)
}

// GetSupplierInput is GetSupplier's request.
type GetSupplierInput struct {
	ID string `path:"id"`
}

func (ctrl *PurchasingController) GetSupplier(ctx context.Context, in *GetSupplierInput) (*api.Out[*models.Supplier], error) {
	c := api.Gin(ctx)
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists || tenantIDRaw == nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}
	tenantID, err := uuid.Parse(tenantIDRaw.(string))
	if err != nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}

	supplierID, err := uuid.Parse(in.ID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, "validation.invalid-uuid"))
	}

	supplier, err := ctrl.service.GetSupplier(c.Request.Context(), tenantID, supplierID)
	if err != nil {
		var pgErr *pgconn.PgError
		if goerrors.As(err, &pgErr) {
			switch pgErr.Message {
			case purchasingErrors.SupplierNotFound:
				return nil, api.Fail(http.StatusNotFound, coreErrors.BuildErrorSingle(c, purchasingErrors.SupplierNotFound))
			}
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}

	return api.OK(supplier)
}

// ListSuppliersInput is ListSuppliers's request.
type ListSuppliersInput struct {
	Page     string `query:"page"`
	PageSize string `query:"page_size"`
}

// ListSuppliersResponse is ListSuppliers's answer.
type ListSuppliersResponse struct {
	Suppliers  []models.Supplier `json:"suppliers"`
	TotalCount int               `json:"total_count"`
	Page       int               `json:"page"`
	PageSize   int               `json:"page_size"`
}

func (ctrl *PurchasingController) ListSuppliers(ctx context.Context, in *ListSuppliersInput) (*api.Out[ListSuppliersResponse], error) {
	c := api.Gin(ctx)
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists || tenantIDRaw == nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}
	tenantID, err := uuid.Parse(tenantIDRaw.(string))
	if err != nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}

	page := 1
	pageSize := 20

	if p := in.Page; p != "" {
		if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
			page = parsed
		}
	}
	if ps := in.PageSize; ps != "" {
		if parsed, err := strconv.Atoi(ps); err == nil && parsed > 0 && parsed <= 100 {
			pageSize = parsed
		}
	}

	suppliers, totalCount, err := ctrl.service.ListSuppliers(c.Request.Context(), tenantID, page, pageSize)
	if err != nil {
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}

	return api.OK(ListSuppliersResponse{Suppliers: suppliers, TotalCount: totalCount, Page: page, PageSize: pageSize})
}

// ========== PURCHASE ORDER ENDPOINTS ==========

// CreatePurchaseOrderInput is CreatePurchaseOrder's request.
type CreatePurchaseOrderInput struct {
	JSON api.Body[models.CreatePurchaseOrderRequestDto]
}

func (ctrl *PurchasingController) CreatePurchaseOrder(ctx context.Context, in *CreatePurchaseOrderInput) (*api.Out[models.CreatePurchaseOrderResponse], error) {
	c := api.Gin(ctx)
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists || tenantIDRaw == nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}
	tenantID, err := uuid.Parse(tenantIDRaw.(string))
	if err != nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}

	var dto models.CreatePurchaseOrderRequestDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, purchasingErrors.POValidationFailed, utils.ExtractValidationError(c, err)))
	}

	userID := c.GetString("user_id")
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
	}
	supplierUUID, err := uuid.Parse(dto.SupplierID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
	}

	po, err := ctrl.service.CreatePurchaseOrder(c.Request.Context(), tenantID, supplierUUID, dto.ExpectedDeliveryDate, dto.Notes, userUUID)
	if err != nil {
		var pgErr *pgconn.PgError
		if goerrors.As(err, &pgErr) {
			switch pgErr.Message {
			case purchasingErrors.SupplierNotFound:
				return nil, api.Fail(http.StatusNotFound, coreErrors.BuildErrorSingle(c, purchasingErrors.SupplierNotFound))
			}
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}

	response := models.CreatePurchaseOrderResponse{
		POID:        po.ID.String(),
		PONumber:    po.PONumber,
		SupplierID:  po.SupplierID.String(),
		Status:      po.Status,
		TotalAmount: po.TotalAmount,
		Message:     "Purchase order created successfully",
	}
	return api.OK(response)
}

// GetPurchaseOrderInput is GetPurchaseOrder's request.
type GetPurchaseOrderInput struct {
	ID string `path:"id"`
}

func (ctrl *PurchasingController) GetPurchaseOrder(ctx context.Context, in *GetPurchaseOrderInput) (*api.Out[*models.PurchaseOrder], error) {
	c := api.Gin(ctx)
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists || tenantIDRaw == nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}
	tenantID, err := uuid.Parse(tenantIDRaw.(string))
	if err != nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}

	poID, err := uuid.Parse(in.ID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, "validation.invalid-uuid"))
	}

	po, err := ctrl.service.GetPurchaseOrder(c.Request.Context(), tenantID, poID)
	if err != nil {
		var pgErr *pgconn.PgError
		if goerrors.As(err, &pgErr) {
			switch pgErr.Message {
			case purchasingErrors.PONotFound:
				return nil, api.Fail(http.StatusNotFound, coreErrors.BuildErrorSingle(c, purchasingErrors.PONotFound))
			}
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}

	return api.OK(po)
}

// ListPurchaseOrdersInput is ListPurchaseOrders's request.
type ListPurchaseOrdersInput struct {
	Page     string `query:"page"`
	PageSize string `query:"page_size"`
	Status   string `query:"status"`
}

// ListPurchaseOrdersResponse is ListPurchaseOrders's answer.
type ListPurchaseOrdersResponse struct {
	PurchaseOrders []models.PurchaseOrder `json:"purchase_orders"`
	TotalCount     int                    `json:"total_count"`
	Page           int                    `json:"page"`
	PageSize       int                    `json:"page_size"`
}

func (ctrl *PurchasingController) ListPurchaseOrders(ctx context.Context, in *ListPurchaseOrdersInput) (*api.Out[ListPurchaseOrdersResponse], error) {
	c := api.Gin(ctx)
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists || tenantIDRaw == nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}
	tenantID, err := uuid.Parse(tenantIDRaw.(string))
	if err != nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}

	page := 1
	pageSize := 20
	var status *string

	if p := in.Page; p != "" {
		if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
			page = parsed
		}
	}
	if ps := in.PageSize; ps != "" {
		if parsed, err := strconv.Atoi(ps); err == nil && parsed > 0 && parsed <= 100 {
			pageSize = parsed
		}
	}
	if s := in.Status; s != "" {
		status = &s
	}

	pos, totalCount, err := ctrl.service.ListPurchaseOrders(c.Request.Context(), tenantID, status, page, pageSize)
	if err != nil {
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}

	return api.OK(ListPurchaseOrdersResponse{PurchaseOrders: pos, TotalCount: totalCount, Page: page, PageSize: pageSize})
}

// AddPurchaseOrderItemInput is AddPurchaseOrderItem's request.
type AddPurchaseOrderItemInput struct {
	ID   string `path:"id"`
	JSON api.Body[models.AddPurchaseOrderItemRequestDto]
}

func (ctrl *PurchasingController) AddPurchaseOrderItem(ctx context.Context, in *AddPurchaseOrderItemInput) (*api.Out[models.AddPurchaseOrderItemResponse], error) {
	c := api.Gin(ctx)
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists || tenantIDRaw == nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}
	tenantID, err := uuid.Parse(tenantIDRaw.(string))
	if err != nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}

	poID, err := uuid.Parse(in.ID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, "validation.invalid-uuid"))
	}

	var dto models.AddPurchaseOrderItemRequestDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, purchasingErrors.POItemValidationFailed, utils.ExtractValidationError(c, err)))
	}

	productUUID, err := uuid.Parse(dto.ProductID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
	}

	item, err := ctrl.service.AddItemToPurchaseOrder(c.Request.Context(), tenantID, poID, productUUID, dto.Quantity, dto.UnitCost, dto.ProductUnitID)
	if err != nil {
		return nil, writeSPError(c, err, map[string]int{
			purchasingErrors.POInvalidStatus:   http.StatusConflict,
			purchasingErrors.ProductNotFound:   http.StatusNotFound,
			purchasingErrors.POInvalidQuantity: http.StatusBadRequest,
			purchasingErrors.POInvalidCost:     http.StatusBadRequest,
			inventoryErrors.UnitNotFound:       http.StatusNotFound,
			inventoryErrors.UnitNoFraction:     http.StatusUnprocessableEntity,
		})
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
	return api.OK(response)
}

// ApprovePurchaseOrderInput is ApprovePurchaseOrder's request.
type ApprovePurchaseOrderInput struct {
	ID string `path:"id"`
}

func (ctrl *PurchasingController) ApprovePurchaseOrder(ctx context.Context, in *ApprovePurchaseOrderInput) (*api.Out[models.PurchaseOrderActionResponse], error) {
	c := api.Gin(ctx)
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists || tenantIDRaw == nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}
	tenantID, err := uuid.Parse(tenantIDRaw.(string))
	if err != nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}

	poID, err := uuid.Parse(in.ID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, "validation.invalid-uuid"))
	}

	po, err := ctrl.service.ApprovePurchaseOrder(c.Request.Context(), tenantID, poID)
	if err != nil {
		return nil, writeSPError(c, err, map[string]int{
			purchasingErrors.PONotFound:      http.StatusNotFound,
			purchasingErrors.POCannotApprove: http.StatusConflict,
			purchasingErrors.PONoItems:       http.StatusConflict,
		})
	}

	response := models.PurchaseOrderActionResponse{
		POID:        po.ID.String(),
		PONumber:    po.PONumber,
		Status:      po.Status,
		TotalAmount: po.TotalAmount,
		Message:     "Purchase order approved",
	}
	return api.OK(response)
}

// ReceivePurchaseOrderInput is ReceivePurchaseOrder's request.
type ReceivePurchaseOrderInput struct {
	ID   string `path:"id"`
	JSON api.Body[models.ReceivePurchaseOrderRequestDto]
}

// ReceivePurchaseOrderResponse is ReceivePurchaseOrder's answer.
type ReceivePurchaseOrderResponse struct {
	ReceiptID     string `json:"receipt_id"`
	ReceiptNumber string `json:"receipt_number"`
	Message       string `json:"message"`
}

func (ctrl *PurchasingController) ReceivePurchaseOrder(ctx context.Context, in *ReceivePurchaseOrderInput) (*api.Out[ReceivePurchaseOrderResponse], error) {
	c := api.Gin(ctx)
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists || tenantIDRaw == nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}
	tenantID, err := uuid.Parse(tenantIDRaw.(string))
	if err != nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}

	poID, err := uuid.Parse(in.ID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, "validation.invalid-uuid"))
	}

	var dto models.ReceivePurchaseOrderRequestDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, purchasingErrors.POReceiveValidation, utils.ExtractValidationError(c, err)))
	}

	var receivedBy *uuid.UUID
	if dto.ReceivedBy != nil {
		if uid, err := uuid.Parse(*dto.ReceivedBy); err == nil {
			receivedBy = &uid
		}
	}

	receipt, err := ctrl.service.ReceivePurchaseOrder(c.Request.Context(), tenantID, poID, receivedBy, dto.Notes)
	if err != nil {
		return nil, writeSPError(c, err, map[string]int{
			purchasingErrors.POCannotReceive: http.StatusConflict,
		})
	}

	return api.OK(ReceivePurchaseOrderResponse{ReceiptID: receipt.ID.String(), ReceiptNumber: receipt.ReceiptNumber, Message: "Purchase order received"})
}

// AddInvoiceInput is AddInvoice's request.
type AddInvoiceInput struct {
	ID   string `path:"id"`
	JSON api.Body[models.AddInvoiceRequestDto]
}

func (ctrl *PurchasingController) AddInvoice(ctx context.Context, in *AddInvoiceInput) (*api.Out[*models.PurchaseOrderInvoice], error) {
	c := api.Gin(ctx)
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists || tenantIDRaw == nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}
	tenantID, err := uuid.Parse(tenantIDRaw.(string))
	if err != nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}

	poID, err := uuid.Parse(in.ID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, "validation.invalid-uuid"))
	}

	var dto models.AddInvoiceRequestDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, purchasingErrors.InvoiceValidationFailed, utils.ExtractValidationError(c, err)))
	}

	invoice, err := ctrl.service.AddInvoice(c.Request.Context(), tenantID, poID, &dto)
	if err != nil {
		return nil, writeSPError(c, err, map[string]int{
			purchasingErrors.PONotFound: http.StatusNotFound,
		})
	}

	return api.OK(invoice)
}

// GetPendingPaymentsResponse is GetPendingPayments's answer.
type GetPendingPaymentsResponse struct {
	PendingPayments []models.PurchaseOrder `json:"pending_payments"`
	TotalCount      int                    `json:"total_count"`
}

func (ctrl *PurchasingController) GetPendingPayments(ctx context.Context, _ *api.None) (*api.Out[GetPendingPaymentsResponse], error) {
	c := api.Gin(ctx)
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists || tenantIDRaw == nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}
	tenantID, err := uuid.Parse(tenantIDRaw.(string))
	if err != nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}

	pos, err := ctrl.service.GetPendingPayments(c.Request.Context(), tenantID)
	if err != nil {
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}

	return api.OK(GetPendingPaymentsResponse{PendingPayments: pos, TotalCount: len(pos)})
}

// ========== PHASE 2A: PRICE COMPARISON ==========

// GetPriceComparisonInput is GetPriceComparison's request.
type GetPriceComparisonInput struct {
	ProductID string `query:"product_id"`
}

// PriceComparisonResponse is a product's suppliers by price; the best and average are the cheapest
// supplier's, null when no supplier has sold it yet.
type PriceComparisonResponse struct {
	ProductID     string                   `json:"product_id"`
	Comparisons   []models.PriceComparison `json:"comparisons"`
	BestPrice     *float64                 `json:"best_price"`
	AverageCost   *float64                 `json:"average_cost"`
	SupplierCount int                      `json:"supplier_count"`
}

func (ctrl *PurchasingController) GetPriceComparison(ctx context.Context, in *GetPriceComparisonInput) (*api.Out[PriceComparisonResponse], error) {
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
	if productID == "" {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, purchasingErrors.InvalidProductID))
	}

	productUUID, err := uuid.Parse(productID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, "validation.invalid-uuid"))
	}

	comparisons, err := ctrl.service.GetPriceComparison(c.Request.Context(), tenantID, productUUID)
	if err != nil {
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}

	// Ordered by unit cost: the first row is the best price. A product no supplier priced has neither.
	response := PriceComparisonResponse{ProductID: productID, Comparisons: comparisons, SupplierCount: len(comparisons)}
	if len(comparisons) > 0 {
		response.BestPrice = &comparisons[0].UnitCost
		response.AverageCost = &comparisons[0].AverageCost
	}
	return api.OK(response)
}

// ========== PHASE 2A: FIFO BATCH TRACKING ==========

// CreateProductBatchInput is CreateProductBatch's request.
type CreateProductBatchInput struct {
	JSON api.Body[models.ProductBatch]
}

func (ctrl *PurchasingController) CreateProductBatch(ctx context.Context, in *CreateProductBatchInput) (*api.Out[*models.ProductBatch], error) {
	c := api.Gin(ctx)
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists || tenantIDRaw == nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}
	tenantID, err := uuid.Parse(tenantIDRaw.(string))
	if err != nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}

	var batch models.ProductBatch
	if err := in.JSON.Into(ctx, &batch); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, purchasingErrors.BatchValidationFailed, utils.ExtractValidationError(c, err)))
	}

	newBatch, err := ctrl.service.CreateProductBatch(c.Request.Context(), tenantID, &batch)
	if err != nil {
		return nil, writeSPError(c, err, map[string]int{
			purchasingErrors.ProductNotFound: http.StatusNotFound,
		})
	}

	return api.OK(newBatch)
}

// GetProductBatchesInput is GetProductBatches's request.
type GetProductBatchesInput struct {
	ProductID string `query:"product_id"`
}

// GetProductBatchesResponse is GetProductBatches's answer.
type GetProductBatchesResponse struct {
	ProductID  string                `json:"product_id"`
	Batches    []models.ProductBatch `json:"batches"`
	BatchCount int                   `json:"batch_count"`
}

func (ctrl *PurchasingController) GetProductBatches(ctx context.Context, in *GetProductBatchesInput) (*api.Out[GetProductBatchesResponse], error) {
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
	if productID == "" {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, purchasingErrors.InvalidProductID))
	}

	productUUID, err := uuid.Parse(productID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, "validation.invalid-uuid"))
	}

	batches, err := ctrl.service.GetProductBatches(c.Request.Context(), tenantID, productUUID)
	if err != nil {
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}

	return api.OK(GetProductBatchesResponse{ProductID: productID, Batches: batches, BatchCount: len(batches)})
}

// GetOldestBatchForSaleInput is GetOldestBatchForSale's request.
type GetOldestBatchForSaleInput struct {
	ProductID string `query:"product_id"`
}

func (ctrl *PurchasingController) GetOldestBatchForSale(ctx context.Context, in *GetOldestBatchForSaleInput) (*api.Out[*models.ProductBatch], error) {
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
	if productID == "" {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, purchasingErrors.InvalidProductID))
	}

	productUUID, err := uuid.Parse(productID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, "validation.invalid-uuid"))
	}

	batch, err := ctrl.service.GetOldestBatchForSale(c.Request.Context(), tenantID, productUUID)
	if err != nil {
		var pgErr *pgconn.PgError
		if goerrors.As(err, &pgErr) {
			switch pgErr.Message {
			case purchasingErrors.BatchNotFound:
				return nil, api.Fail(http.StatusNotFound, coreErrors.BuildErrorSingle(c, purchasingErrors.BatchNotFound))
			}
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}

	return api.OK(batch)
}

// ========== PHASE 2A: RFQ (REQUEST FOR QUOTE) ==========

// CreateRFQInput is CreateRFQ's request.
type CreateRFQInput struct {
	JSON api.Body[models.CreateRFQRequestDto]
}

func (ctrl *PurchasingController) CreateRFQ(ctx context.Context, in *CreateRFQInput) (*api.Out[*models.RequestForQuote], error) {
	c := api.Gin(ctx)
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists || tenantIDRaw == nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}
	tenantID, err := uuid.Parse(tenantIDRaw.(string))
	if err != nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}

	var dto models.CreateRFQRequestDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, purchasingErrors.RFQValidationFailed, utils.ExtractValidationError(c, err)))
	}

	rfq := &models.RequestForQuote{
		RFQNumber: "RFQ-" + uuid.New().String()[:8],
		Status:    "draft",
	}
	newRFQ, err := ctrl.service.CreateRFQ(c.Request.Context(), tenantID, rfq)
	if err != nil {
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}

	for _, itemReq := range dto.Items {
		productUUID, err := uuid.Parse(itemReq.ProductID)
		if err != nil {
			return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, purchasingErrors.InvalidProductID))
		}

		item := &models.RFQItem{
			ProductID:   productUUID,
			Quantity:    itemReq.Quantity,
			Description: itemReq.Description,
		}
		_, err = ctrl.service.AddRFQItem(c.Request.Context(), tenantID, newRFQ.ID, item)
		if err != nil {
			return nil, writeSPError(c, err, map[string]int{
				purchasingErrors.ProductNotFound: http.StatusNotFound,
			})
		}
	}

	newRFQ.Status = "sent"
	return api.OK(newRFQ)
}

// GetRFQResponsesInput is GetRFQResponses's request.
type GetRFQResponsesInput struct {
	RfqID string `path:"rfq_id"`
}

// GetRFQResponsesResponse is GetRFQResponses's answer.
type GetRFQResponsesResponse struct {
	RFQID         string               `json:"rfq_id"`
	Responses     []models.RFQResponse `json:"responses"`
	ResponseCount int                  `json:"response_count"`
}

func (ctrl *PurchasingController) GetRFQResponses(ctx context.Context, in *GetRFQResponsesInput) (*api.Out[GetRFQResponsesResponse], error) {
	c := api.Gin(ctx)
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists || tenantIDRaw == nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}
	tenantID, err := uuid.Parse(tenantIDRaw.(string))
	if err != nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}

	rfqID := in.RfqID
	if rfqID == "" {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, "missing.rfq-id"))
	}

	rfqUUID, err := uuid.Parse(rfqID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, "validation.invalid-uuid"))
	}

	responses, err := ctrl.service.GetRFQResponses(c.Request.Context(), tenantID, rfqUUID)
	if err != nil {
		return nil, writeSPError(c, err, map[string]int{
			purchasingErrors.RFQNotFound: http.StatusNotFound,
		})
	}

	return api.OK(GetRFQResponsesResponse{RFQID: rfqID, Responses: responses, ResponseCount: len(responses)})
}

// GetRFQComparisonInput is GetRFQComparison's request.
type GetRFQComparisonInput struct {
	RfqID string `path:"rfq_id"`
}

func (ctrl *PurchasingController) GetRFQComparison(ctx context.Context, in *GetRFQComparisonInput) (*api.Out[map[string]interface{}], error) {
	c := api.Gin(ctx)
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists || tenantIDRaw == nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}
	tenantID, err := uuid.Parse(tenantIDRaw.(string))
	if err != nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}

	rfqID := in.RfqID
	if rfqID == "" {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, "missing.rfq-id"))
	}

	rfqUUID, err := uuid.Parse(rfqID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, "validation.invalid-uuid"))
	}

	comparison, err := ctrl.service.GetRFQComparison(c.Request.Context(), tenantID, rfqUUID)
	if err != nil {
		return nil, writeSPError(c, err, map[string]int{
			purchasingErrors.RFQNotFound: http.StatusNotFound,
		})
	}

	return api.OK(comparison)
}

// SelectRFQResponseInput is SelectRFQResponse's request.
type SelectRFQResponseInput struct {
	ResponseID string `path:"response_id"`
	RfqID      string `path:"rfq_id"`
}

// SelectRFQResponseResponse is SelectRFQResponse's answer.
type SelectRFQResponseResponse struct {
	Status             string `json:"status"`
	RFQID              string `json:"rfq_id"`
	SelectedResponseID string `json:"selected_response_id"`
}

func (ctrl *PurchasingController) SelectRFQResponse(ctx context.Context, in *SelectRFQResponseInput) (*api.Out[SelectRFQResponseResponse], error) {
	c := api.Gin(ctx)
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists || tenantIDRaw == nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}
	tenantID, err := uuid.Parse(tenantIDRaw.(string))
	if err != nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}

	rfqID := in.RfqID
	responseID := in.ResponseID

	if rfqID == "" || responseID == "" {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, "missing.parameters"))
	}

	rfqUUID, err := uuid.Parse(rfqID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, "validation.invalid-uuid"))
	}

	responseUUID, err := uuid.Parse(responseID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, "validation.invalid-uuid"))
	}

	err = ctrl.service.SelectBestRFQResponse(c.Request.Context(), tenantID, rfqUUID, responseUUID)
	if err != nil {
		return nil, writeSPError(c, err, map[string]int{
			purchasingErrors.RFQNotFound:         http.StatusNotFound,
			purchasingErrors.RFQResponseNotFound: http.StatusNotFound,
		})
	}

	return api.OK(SelectRFQResponseResponse{Status: "RFQ closed", RFQID: rfqID, SelectedResponseID: responseID})
}

// writeSPError answers the status a stored procedure's refusal maps to, or 500 when it maps to none.
func writeSPError(c *gin.Context, err error, statuses map[string]int) error {
	var pgErr *pgconn.PgError
	if goerrors.As(err, &pgErr) {
		if status, ok := statuses[pgErr.Message]; ok {
			return api.Fail(status, coreErrors.BuildErrorSingle(c, pgErr.Message))
		}
	}
	return api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
}
