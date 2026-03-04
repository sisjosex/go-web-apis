package interfaces

import (
	"context"

	"josex/web/modules/sales/models"

	"github.com/google/uuid"
)

// SalesOrderRepository defines sales order data access operations
type SalesOrderRepository interface {
	CreateSalesOrder(ctx context.Context, tenantID uuid.UUID, dto *models.CreateSalesOrderRequestDto) (*models.SalesOrder, error)
	GetSalesOrderByID(ctx context.Context, id uuid.UUID) (*models.SalesOrder, error)
	GetSalesOrdersByTenant(ctx context.Context, tenantID uuid.UUID, limit int, offset int) ([]models.SalesOrder, error)
	UpdateSalesOrder(ctx context.Context, id uuid.UUID, dto *models.UpdateSalesOrderRequestDto) (*models.SalesOrder, error)
	GetSalesOrdersByCustomer(ctx context.Context, customerID uuid.UUID) ([]models.SalesOrder, error)
	GetSalesOrderByOrderNumber(ctx context.Context, orderNumber string, tenantID uuid.UUID) (*models.SalesOrder, error)
	AddOrderItem(ctx context.Context, orderID uuid.UUID, dto *models.CreateOrderItemRequestDto) (*models.OrderItem, error)
	GetOrderItems(ctx context.Context, orderID uuid.UUID) ([]models.OrderItem, error)
}
