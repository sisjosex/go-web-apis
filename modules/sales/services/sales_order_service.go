package services

import (
	"context"
	"time"

	"github.com/google/uuid"

	coreModels "josex/web/modules/core/models"
	"josex/web/modules/sales/interfaces"
	"josex/web/modules/sales/models"
)

// SalesOrderService handles sales order business logic
type SalesOrderService struct {
	orderRepository interfaces.SalesOrderRepository
}

// NewSalesOrderService creates a new sales order service
func NewSalesOrderService(repo interfaces.SalesOrderRepository) *SalesOrderService {
	return &SalesOrderService{
		orderRepository: repo,
	}
}

// CreateSalesOrder creates a new sales order
func (s *SalesOrderService) CreateSalesOrder(ctx context.Context, tenantID uuid.UUID, dto *models.CreateSalesOrderRequestDto) (*models.SalesOrder, error) {
	return s.orderRepository.CreateSalesOrder(ctx, tenantID, dto)
}

// GetSalesOrderByID retrieves a sales order by ID
func (s *SalesOrderService) GetSalesOrderByID(ctx context.Context, tenantID uuid.UUID, id uuid.UUID) (*models.SalesOrder, error) {
	return s.orderRepository.GetSalesOrderByID(ctx, tenantID, id)
}

// GetAllSalesOrders retrieves all sales orders
func (s *SalesOrderService) GetAllSalesOrders(ctx context.Context, tenantID uuid.UUID, limit int, offset int) ([]models.SalesOrder, error) {
	return s.orderRepository.GetAllSalesOrders(ctx, tenantID, limit, offset)
}

// UpdateSalesOrder updates a sales order
func (s *SalesOrderService) UpdateSalesOrder(ctx context.Context, tenantID uuid.UUID, id uuid.UUID, dto *models.UpdateSalesOrderRequestDto) (*models.SalesOrder, error) {
	return s.orderRepository.UpdateSalesOrder(ctx, tenantID, id, dto)
}

// GetSalesOrdersByCustomer retrieves all orders for a customer
func (s *SalesOrderService) GetSalesOrdersByCustomer(ctx context.Context, tenantID uuid.UUID, customerID uuid.UUID) ([]models.SalesOrder, error) {
	return s.orderRepository.GetSalesOrdersByCustomer(ctx, tenantID, customerID)
}

// AddOrderItem adds an item to an order
func (s *SalesOrderService) AddOrderItem(ctx context.Context, tenantID uuid.UUID, orderID uuid.UUID, dto *models.CreateOrderItemRequestDto) (*models.OrderItem, error) {
	return s.orderRepository.AddOrderItem(ctx, tenantID, orderID, dto)
}

// GetOrderItems retrieves all items for an order
func (s *SalesOrderService) GetOrderItems(ctx context.Context, tenantID uuid.UUID, orderID uuid.UUID) ([]models.OrderItem, error) {
	return s.orderRepository.GetOrderItems(ctx, tenantID, orderID)
}

// ========== PHASE 1: Batch Assignment & Order Completion ==========

// AddOrderItemWithBatch adds an item to an order with FIFO batch assignment
func (s *SalesOrderService) AddOrderItemWithBatch(ctx context.Context, tenantID uuid.UUID, orderID uuid.UUID, productID uuid.UUID, quantity float64, unitPrice float64, skuID *uuid.UUID) (*models.OrderItem, error) {
	return s.orderRepository.AddOrderItemWithBatch(ctx, tenantID, orderID, productID, quantity, unitPrice, skuID)
}

// CompleteOrder completes an order and consumes inventory from batches
func (s *SalesOrderService) CompleteOrder(ctx context.Context, tenantID uuid.UUID, orderID uuid.UUID) (*models.SalesOrder, error) {
	return s.orderRepository.CompleteOrder(ctx, tenantID, orderID)
}

// ========== PHASE 2: Reporting & Cancellation ==========

// CancelOrder cancels an order and releases batch assignments
func (s *SalesOrderService) CancelOrder(ctx context.Context, tenantID uuid.UUID, orderID uuid.UUID) (*models.SalesOrder, error) {
	return s.orderRepository.CancelOrder(ctx, tenantID, orderID)
}

// GetOrderWithBatches retrieves an order with batch assignment details
func (s *SalesOrderService) GetOrderWithBatches(ctx context.Context, tenantID uuid.UUID, orderID uuid.UUID) (*models.OrderWithBatches, error) {
	return s.orderRepository.GetOrderWithBatches(ctx, tenantID, orderID)
}

// GetSalesReport retrieves sales metrics for a date range
func (s *SalesOrderService) GetSalesReport(ctx context.Context, tenantID uuid.UUID, startDate, endDate coreModels.DateOnly) ([]models.SalesReport, error) {
	return s.orderRepository.GetSalesReport(ctx, tenantID, time.Time(startDate), time.Time(endDate))
}

// ========== PHASE 3: Returns & Payments ==========

// CreateReturn creates a return request for a completed order
func (s *SalesOrderService) CreateReturn(ctx context.Context, tenantID uuid.UUID, orderID uuid.UUID, reason string) (*models.Return, error) {
	return s.orderRepository.CreateReturn(ctx, tenantID, orderID, reason)
}

// ApproveReturn approves a return and restores inventory
func (s *SalesOrderService) ApproveReturn(ctx context.Context, tenantID uuid.UUID, returnID uuid.UUID) (*models.Return, error) {
	return s.orderRepository.ApproveReturn(ctx, tenantID, returnID)
}

// GetReturn retrieves a return by ID
func (s *SalesOrderService) GetReturn(ctx context.Context, tenantID uuid.UUID, returnID uuid.UUID) (*models.Return, error) {
	return s.orderRepository.GetReturn(ctx, tenantID, returnID)
}

// GetReturnsByOrder retrieves all returns for an order
func (s *SalesOrderService) GetReturnsByOrder(ctx context.Context, tenantID uuid.UUID, orderID uuid.UUID) ([]models.Return, error) {
	return s.orderRepository.GetReturnsByOrder(ctx, tenantID, orderID)
}

// CreatePayment creates a payment record for an order
func (s *SalesOrderService) CreatePayment(ctx context.Context, tenantID uuid.UUID, orderID uuid.UUID, amount float64, paymentMethod string, referenceNumber string, notes string) (*models.Payment, error) {
	return s.orderRepository.CreatePayment(ctx, tenantID, orderID, amount, paymentMethod, referenceNumber, notes)
}

// GetPayments retrieves all payments for an order
func (s *SalesOrderService) GetPayments(ctx context.Context, tenantID uuid.UUID, orderID uuid.UUID) ([]models.Payment, error) {
	return s.orderRepository.GetPayments(ctx, tenantID, orderID)
}

// GetPaymentByID retrieves a payment by ID
func (s *SalesOrderService) GetPaymentByID(ctx context.Context, tenantID uuid.UUID, paymentID uuid.UUID) (*models.Payment, error) {
	return s.orderRepository.GetPaymentByID(ctx, tenantID, paymentID)
}
