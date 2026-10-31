package controllers

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"josex/web/modules/core/api"
	coreErrors "josex/web/modules/core/errors"
	coreModels "josex/web/modules/core/models"
	inventoryUtils "josex/web/modules/inventory/utils"
	salesErrors "josex/web/modules/sales/errors"
	"josex/web/modules/sales/interfaces"
	"josex/web/modules/sales/models"
	"josex/web/modules/sales/utils"
)

// SalesOrderController handles sales order HTTP requests
type SalesOrderController struct {
	service interfaces.SalesOrderService
}

// NewSalesOrderController creates a new sales order controller
func NewSalesOrderController(service interfaces.SalesOrderService) *SalesOrderController {
	return &SalesOrderController{
		service: service,
	}
}

// CreateSalesOrderInput is CreateSalesOrder's request.
type CreateSalesOrderInput struct {
	JSON api.Body[models.CreateSalesOrderRequestDto]
}

// CreateSalesOrderResponse is CreateSalesOrder's answer.
type CreateSalesOrderResponse struct {
	Data *models.SalesOrder `json:"data"`
}

// CreateSalesOrder is POST : Create a new sales order.
func (ctrl *SalesOrderController) CreateSalesOrder(ctx context.Context, in *CreateSalesOrderInput) (*api.Out[CreateSalesOrderResponse], error) {
	c := api.Gin(ctx)
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists || tenantIDRaw == nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}
	tenantID, err := uuid.Parse(tenantIDRaw.(string))
	if err != nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}

	var dto models.CreateSalesOrderRequestDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.failed", err.Error()))
	}

	order, err := ctrl.service.CreateSalesOrder(c.Request.Context(), tenantID, &dto)
	if err != nil {
		status := utils.GetHTTPStatusFromError(err)
		return nil, api.Fail(status, coreErrors.BuildError(c, err))
	}

	return api.OK(CreateSalesOrderResponse{Data: order})
}

// GetSalesOrderInput is GetSalesOrder's request.
type GetSalesOrderInput struct {
	ID string `path:"id"`
}

// GetSalesOrderResponse is GetSalesOrder's answer.
type GetSalesOrderResponse struct {
	Data *models.SalesOrder `json:"data"`
}

// GetSalesOrder is GET /{id}: Get a sales order by ID.
func (ctrl *SalesOrderController) GetSalesOrder(ctx context.Context, in *GetSalesOrderInput) (*api.Out[GetSalesOrderResponse], error) {
	c := api.Gin(ctx)
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists || tenantIDRaw == nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}
	tenantID, err := uuid.Parse(tenantIDRaw.(string))
	if err != nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}

	id, err := uuid.Parse(in.ID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, "validation.invalid-uuid"))
	}

	order, err := ctrl.service.GetSalesOrderByID(c.Request.Context(), tenantID, id)
	if err != nil {
		status := utils.GetHTTPStatusFromError(err)
		return nil, api.Fail(status, coreErrors.BuildError(c, err))
	}

	// Get order items
	items, _ := ctrl.service.GetOrderItems(c.Request.Context(), tenantID, id)
	order.Items = items

	return api.OK(GetSalesOrderResponse{Data: order})
}

// ListSalesOrdersInput is ListSalesOrders's request.
type ListSalesOrdersInput struct {
	Limit  string `query:"limit"`
	Offset string `query:"offset"`
}

// ListSalesOrdersResponse is ListSalesOrders's answer.
type ListSalesOrdersResponse struct {
	Data []models.SalesOrder `json:"data"`
}

// ListSalesOrders is GET : List sales orders for tenant.
func (ctrl *SalesOrderController) ListSalesOrders(ctx context.Context, in *ListSalesOrdersInput) (*api.Out[ListSalesOrdersResponse], error) {
	c := api.Gin(ctx)
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists || tenantIDRaw == nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}
	tenantID, err := uuid.Parse(tenantIDRaw.(string))
	if err != nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}

	limit := 20
	offset := 0
	if l := in.Limit; l != "" {
		if parsed, err := strconv.Atoi(l); err == nil {
			limit = parsed
		}
	}
	if o := in.Offset; o != "" {
		if parsed, err := strconv.Atoi(o); err == nil {
			offset = parsed
		}
	}

	orders, err := ctrl.service.GetAllSalesOrders(c.Request.Context(), tenantID, limit, offset)
	if err != nil {
		status := utils.GetHTTPStatusFromError(err)
		return nil, api.Fail(status, coreErrors.BuildError(c, err))
	}

	return api.OK(ListSalesOrdersResponse{Data: orders})
}

// UpdateSalesOrderInput is UpdateSalesOrder's request.
type UpdateSalesOrderInput struct {
	ID   string `path:"id"`
	JSON api.Body[models.UpdateSalesOrderRequestDto]
}

// UpdateSalesOrderResponse is UpdateSalesOrder's answer.
type UpdateSalesOrderResponse struct {
	Data *models.SalesOrder `json:"data"`
}

// UpdateSalesOrder is PATCH /{id}: Update a sales order.
func (ctrl *SalesOrderController) UpdateSalesOrder(ctx context.Context, in *UpdateSalesOrderInput) (*api.Out[UpdateSalesOrderResponse], error) {
	c := api.Gin(ctx)
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists || tenantIDRaw == nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}
	tenantID, err := uuid.Parse(tenantIDRaw.(string))
	if err != nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}

	id, err := uuid.Parse(in.ID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, "validation.invalid-uuid"))
	}

	var dto models.UpdateSalesOrderRequestDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.failed", err.Error()))
	}

	order, err := ctrl.service.UpdateSalesOrder(c.Request.Context(), tenantID, id, &dto)
	if err != nil {
		status := utils.GetHTTPStatusFromError(err)
		return nil, api.Fail(status, coreErrors.BuildError(c, err))
	}

	return api.OK(UpdateSalesOrderResponse{Data: order})
}

// ========== PHASE 1: Batch Assignment & Order Completion ==========

// AddOrderItemInput is AddOrderItem's request.
type AddOrderItemInput struct {
	ID   string `path:"id"`
	JSON api.Body[models.AddOrderItemRequestDto]
}

// AddOrderItemResponse is AddOrderItem's answer.
type AddOrderItemResponse struct {
	Data *models.OrderItem `json:"data"`
}

// AddOrderItem is POST /{id}/items: Add an item to an order with batch assignment.
func (ctrl *SalesOrderController) AddOrderItem(ctx context.Context, in *AddOrderItemInput) (*api.Out[AddOrderItemResponse], error) {
	c := api.Gin(ctx)
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists || tenantIDRaw == nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}
	tenantID, err := uuid.Parse(tenantIDRaw.(string))
	if err != nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}

	orderID, err := uuid.Parse(in.ID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, "validation.invalid-uuid"))
	}

	var dto models.AddOrderItemRequestDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.failed", err.Error()))
	}

	item, err := ctrl.service.AddOrderItemWithBatch(c.Request.Context(), tenantID, inventoryUtils.TenantToday(c), orderID, dto.ProductID, dto.Quantity, dto.UnitPrice, dto.SkuID)
	if err != nil {
		// GetHTTPStatusFromError knows nothing about the INV-010 codes, and
		// sku-required matches none of its patterns, so it would fall through
		// to 500 for what is a plain client mistake.
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Message {
			case salesErrors.OrderItemSkuRequired:
				return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, salesErrors.OrderItemSkuRequired))
			case salesErrors.OrderItemSkuNotFound:
				return nil, api.Fail(http.StatusNotFound, coreErrors.BuildErrorSingle(c, salesErrors.OrderItemSkuNotFound))
			}
		}

		status := utils.GetHTTPStatusFromError(err)
		return nil, api.Fail(status, coreErrors.BuildError(c, err))
	}

	return api.OK(AddOrderItemResponse{Data: item})
}

// CompleteOrderInput is CompleteOrder's request.
type CompleteOrderInput struct {
	ID string `path:"id"`
}

// CompleteOrderResponse is CompleteOrder's answer.
type CompleteOrderResponse struct {
	Data *models.SalesOrder `json:"data"`
}

// CompleteOrder is PATCH /{id}/complete: Complete a sales order and consume inventory.
func (ctrl *SalesOrderController) CompleteOrder(ctx context.Context, in *CompleteOrderInput) (*api.Out[CompleteOrderResponse], error) {
	c := api.Gin(ctx)
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists || tenantIDRaw == nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}
	tenantID, err := uuid.Parse(tenantIDRaw.(string))
	if err != nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}

	orderID, err := uuid.Parse(in.ID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, "validation.invalid-uuid"))
	}

	order, err := ctrl.service.CompleteOrder(c.Request.Context(), tenantID, orderID)
	if err != nil {
		status := utils.GetHTTPStatusFromError(err)
		return nil, api.Fail(status, coreErrors.BuildError(c, err))
	}

	return api.OK(CompleteOrderResponse{Data: order})
}

// ========== PHASE 2: Reporting & Cancellation ==========

// CancelOrderInput is CancelOrder's request.
type CancelOrderInput struct {
	ID string `path:"id"`
}

// CancelOrderResponse is CancelOrder's answer.
type CancelOrderResponse struct {
	Data *models.SalesOrder `json:"data"`
}

// CancelOrder is PATCH /{id}/cancel: Cancel a sales order and release batch assignments.
func (ctrl *SalesOrderController) CancelOrder(ctx context.Context, in *CancelOrderInput) (*api.Out[CancelOrderResponse], error) {
	c := api.Gin(ctx)
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists || tenantIDRaw == nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}
	tenantID, err := uuid.Parse(tenantIDRaw.(string))
	if err != nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}

	orderID, err := uuid.Parse(in.ID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, "validation.invalid-uuid"))
	}

	order, err := ctrl.service.CancelOrder(c.Request.Context(), tenantID, orderID)
	if err != nil {
		status := utils.GetHTTPStatusFromError(err)
		return nil, api.Fail(status, coreErrors.BuildError(c, err))
	}

	return api.OK(CancelOrderResponse{Data: order})
}

// GetOrderWithBatchesInput is GetOrderWithBatches's request.
type GetOrderWithBatchesInput struct {
	ID string `path:"id"`
}

// GetOrderWithBatchesResponse is GetOrderWithBatches's answer.
type GetOrderWithBatchesResponse struct {
	Data *models.OrderWithBatches `json:"data"`
}

// GetOrderWithBatches is GET /{id}/with-batches: Get order details with batch assignments.
func (ctrl *SalesOrderController) GetOrderWithBatches(ctx context.Context, in *GetOrderWithBatchesInput) (*api.Out[GetOrderWithBatchesResponse], error) {
	c := api.Gin(ctx)
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists || tenantIDRaw == nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}
	tenantID, err := uuid.Parse(tenantIDRaw.(string))
	if err != nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}

	orderID, err := uuid.Parse(in.ID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, "validation.invalid-uuid"))
	}

	order, err := ctrl.service.GetOrderWithBatches(c.Request.Context(), tenantID, orderID)
	if err != nil {
		status := utils.GetHTTPStatusFromError(err)
		return nil, api.Fail(status, coreErrors.BuildError(c, err))
	}

	return api.OK(GetOrderWithBatchesResponse{Data: order})
}

// GetSalesReportInput is GetSalesReport's request.
type GetSalesReportInput struct {
	StartDate string `query:"start_date"`
	EndDate   string `query:"end_date"`
}

// GetSalesReportResponse is GetSalesReport's answer.
type GetSalesReportResponse struct {
	Data models.SalesReportResponse `json:"data"`
}

// GetSalesReport is GET /sales: Get sales report for date range.
func (ctrl *SalesOrderController) GetSalesReport(ctx context.Context, in *GetSalesReportInput) (*api.Out[GetSalesReportResponse], error) {
	c := api.Gin(ctx)
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists || tenantIDRaw == nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}
	tenantID, err := uuid.Parse(tenantIDRaw.(string))
	if err != nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}

	startDateStr := in.StartDate
	endDateStr := in.EndDate

	if startDateStr == "" || endDateStr == "" {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.failed", "start_date and end_date are required"))
	}

	var startDate coreModels.DateOnly
	errStart := startDate.UnmarshalJSON([]byte(`"` + startDateStr + `"`))
	if errStart != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.failed", "invalid start_date format"))
	}

	var endDate coreModels.DateOnly
	errEnd := endDate.UnmarshalJSON([]byte(`"` + endDateStr + `"`))
	if errEnd != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.failed", "invalid end_date format"))
	}

	reports, err := ctrl.service.GetSalesReport(c.Request.Context(), tenantID, startDate, endDate)
	if err != nil {
		status := utils.GetHTTPStatusFromError(err)
		return nil, api.Fail(status, coreErrors.BuildError(c, err))
	}

	return api.OK(GetSalesReportResponse{Data: models.SalesReportResponse{Metrics: reports}})
}

// ========== PHASE 3: Returns & Payments ==========

// CreateReturnInput is CreateReturn's request.
type CreateReturnInput struct {
	JSON api.Body[models.CreateReturnRequestDto]
}

// CreateReturnResponse is CreateReturn's answer.
type CreateReturnResponse struct {
	Data *models.Return `json:"data"`
}

// CreateReturn is POST : Create a return request for a completed order.
func (ctrl *SalesOrderController) CreateReturn(ctx context.Context, in *CreateReturnInput) (*api.Out[CreateReturnResponse], error) {
	c := api.Gin(ctx)
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists || tenantIDRaw == nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}
	tenantID, err := uuid.Parse(tenantIDRaw.(string))
	if err != nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}

	var dto models.CreateReturnRequestDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.failed", err.Error()))
	}

	ret, err := ctrl.service.CreateReturn(c.Request.Context(), tenantID, dto.OrderID, dto.Reason)
	if err != nil {
		status := utils.GetHTTPStatusFromError(err)
		return nil, api.Fail(status, coreErrors.BuildError(c, err))
	}

	return api.OK(CreateReturnResponse{Data: ret})
}

// ApproveReturnInput is ApproveReturn's request.
type ApproveReturnInput struct {
	ID string `path:"id"`
}

// ApproveReturnResponse is ApproveReturn's answer.
type ApproveReturnResponse struct {
	Data *models.Return `json:"data"`
}

// ApproveReturn is PATCH /{id}/approve: Approve a return request and restore inventory.
func (ctrl *SalesOrderController) ApproveReturn(ctx context.Context, in *ApproveReturnInput) (*api.Out[ApproveReturnResponse], error) {
	c := api.Gin(ctx)
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists || tenantIDRaw == nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}
	tenantID, err := uuid.Parse(tenantIDRaw.(string))
	if err != nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}

	returnID, err := uuid.Parse(in.ID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, "validation.invalid-uuid"))
	}

	ret, err := ctrl.service.ApproveReturn(c.Request.Context(), tenantID, returnID)
	if err != nil {
		status := utils.GetHTTPStatusFromError(err)
		return nil, api.Fail(status, coreErrors.BuildError(c, err))
	}

	return api.OK(ApproveReturnResponse{Data: ret})
}

// GetReturnsInput is GetReturns's request.
type GetReturnsInput struct {
	OrderID string `query:"order_id"`
}

// GetReturnsResponse is GetReturns's answer.
type GetReturnsResponse struct {
	Data []models.Return `json:"data"`
}

// GetReturns is GET : Get returns for an order.
func (ctrl *SalesOrderController) GetReturns(ctx context.Context, in *GetReturnsInput) (*api.Out[GetReturnsResponse], error) {
	c := api.Gin(ctx)
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists || tenantIDRaw == nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}
	tenantID, err := uuid.Parse(tenantIDRaw.(string))
	if err != nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}

	orderIDStr := in.OrderID
	if orderIDStr == "" {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.failed", "order_id is required"))
	}

	orderID, err := uuid.Parse(orderIDStr)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, "validation.invalid-uuid"))
	}

	returns, err := ctrl.service.GetReturnsByOrder(c.Request.Context(), tenantID, orderID)
	if err != nil {
		status := utils.GetHTTPStatusFromError(err)
		return nil, api.Fail(status, coreErrors.BuildError(c, err))
	}

	return api.OK(GetReturnsResponse{Data: returns})
}

// CreatePaymentInput is CreatePayment's request.
type CreatePaymentInput struct {
	JSON api.Body[models.CreatePaymentRequestDto]
}

// CreatePaymentResponse is CreatePayment's answer.
type CreatePaymentResponse struct {
	Data *models.Payment `json:"data"`
}

// CreatePayment is POST : Create a payment record for an order.
func (ctrl *SalesOrderController) CreatePayment(ctx context.Context, in *CreatePaymentInput) (*api.Out[CreatePaymentResponse], error) {
	c := api.Gin(ctx)
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists || tenantIDRaw == nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}
	tenantID, err := uuid.Parse(tenantIDRaw.(string))
	if err != nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}

	var dto models.CreatePaymentRequestDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.failed", err.Error()))
	}

	payment, err := ctrl.service.CreatePayment(c.Request.Context(), tenantID, dto.OrderID, dto.Amount, dto.PaymentMethod, dto.ReferenceNumber, dto.Notes)
	if err != nil {
		status := utils.GetHTTPStatusFromError(err)
		return nil, api.Fail(status, coreErrors.BuildError(c, err))
	}

	return api.OK(CreatePaymentResponse{Data: payment})
}

// GetPaymentsInput is GetPayments's request.
type GetPaymentsInput struct {
	OrderID string `query:"order_id"`
}

// GetPaymentsResponse is GetPayments's answer.
type GetPaymentsResponse struct {
	Data []models.Payment `json:"data"`
}

// GetPayments is GET : Get payments for an order.
func (ctrl *SalesOrderController) GetPayments(ctx context.Context, in *GetPaymentsInput) (*api.Out[GetPaymentsResponse], error) {
	c := api.Gin(ctx)
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists || tenantIDRaw == nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}
	tenantID, err := uuid.Parse(tenantIDRaw.(string))
	if err != nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}

	orderIDStr := in.OrderID
	if orderIDStr == "" {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.failed", "order_id is required"))
	}

	orderID, err := uuid.Parse(orderIDStr)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, "validation.invalid-uuid"))
	}

	payments, err := ctrl.service.GetPayments(c.Request.Context(), tenantID, orderID)
	if err != nil {
		status := utils.GetHTTPStatusFromError(err)
		return nil, api.Fail(status, coreErrors.BuildError(c, err))
	}

	return api.OK(GetPaymentsResponse{Data: payments})
}
