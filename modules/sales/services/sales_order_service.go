package services

import (
	"context"

	"josex/web/modules/sales/interfaces"
	"josex/web/modules/sales/models"

	"github.com/google/uuid"
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
func (s *SalesOrderService) GetSalesOrderByID(ctx context.Context, id uuid.UUID) (*models.SalesOrder, error) {
	return s.orderRepository.GetSalesOrderByID(ctx, id)
}

// GetSalesOrdersByTenant retrieves all orders for a tenant
func (s *SalesOrderService) GetSalesOrdersByTenant(ctx context.Context, tenantID uuid.UUID, limit int, offset int) ([]models.SalesOrder, error) {
	return s.orderRepository.GetSalesOrdersByTenant(ctx, tenantID, limit, offset)
}

// UpdateSalesOrder updates a sales order
func (s *SalesOrderService) UpdateSalesOrder(ctx context.Context, id uuid.UUID, dto *models.UpdateSalesOrderRequestDto) (*models.SalesOrder, error) {
	return s.orderRepository.UpdateSalesOrder(ctx, id, dto)
}

// GetSalesOrdersByCustomer retrieves all orders for a customer
func (s *SalesOrderService) GetSalesOrdersByCustomer(ctx context.Context, customerID uuid.UUID) ([]models.SalesOrder, error) {
	return s.orderRepository.GetSalesOrdersByCustomer(ctx, customerID)
}

// AddOrderItem adds an item to an order
func (s *SalesOrderService) AddOrderItem(ctx context.Context, orderID uuid.UUID, dto *models.CreateOrderItemRequestDto) (*models.OrderItem, error) {
	return s.orderRepository.AddOrderItem(ctx, orderID, dto)
}

// GetOrderItems retrieves all items for an order
func (s *SalesOrderService) GetOrderItems(ctx context.Context, orderID uuid.UUID) ([]models.OrderItem, error) {
	return s.orderRepository.GetOrderItems(ctx, orderID)
}
