package interfaces

import (
	"context"
	"time"

	"josex/web/modules/sales/models"

	"github.com/google/uuid"
)

// SalesOrderRepository defines sales order data access operations
type SalesOrderRepository interface {
	CreateSalesOrder(ctx context.Context, tenantID uuid.UUID, dto *models.CreateSalesOrderRequestDto) (*models.SalesOrder, error)
	GetSalesOrderByID(ctx context.Context, tenantID uuid.UUID, id uuid.UUID) (*models.SalesOrder, error)
	GetAllSalesOrders(ctx context.Context, tenantID uuid.UUID, limit int, offset int) ([]models.SalesOrder, error)
	UpdateSalesOrder(ctx context.Context, tenantID uuid.UUID, id uuid.UUID, dto *models.UpdateSalesOrderRequestDto) (*models.SalesOrder, error)
	GetSalesOrdersByCustomer(ctx context.Context, tenantID uuid.UUID, customerID uuid.UUID) ([]models.SalesOrder, error)
	GetSalesOrderByOrderNumber(ctx context.Context, tenantID uuid.UUID, orderNumber string) (*models.SalesOrder, error)
	AddOrderItem(ctx context.Context, tenantID uuid.UUID, orderID uuid.UUID, dto *models.CreateOrderItemRequestDto) (*models.OrderItem, error)
	GetOrderItems(ctx context.Context, tenantID uuid.UUID, orderID uuid.UUID) ([]models.OrderItem, error)

	// Phase 1 methods
	AddOrderItemWithBatch(ctx context.Context, tenantID uuid.UUID, orderID uuid.UUID, productID uuid.UUID, quantity float64, unitPrice float64) (*models.OrderItem, error)
	CompleteOrder(ctx context.Context, tenantID uuid.UUID, orderID uuid.UUID) (*models.SalesOrder, error)

	// Phase 2 methods
	CancelOrder(ctx context.Context, tenantID uuid.UUID, orderID uuid.UUID) (*models.SalesOrder, error)
	GetOrderWithBatches(ctx context.Context, tenantID uuid.UUID, orderID uuid.UUID) (*models.OrderWithBatches, error)
	GetSalesReport(ctx context.Context, tenantID uuid.UUID, startDate time.Time, endDate time.Time) ([]models.SalesReport, error)

	// Phase 3 methods
	CreateReturn(ctx context.Context, tenantID uuid.UUID, orderID uuid.UUID, reason string) (*models.Return, error)
	ApproveReturn(ctx context.Context, tenantID uuid.UUID, returnID uuid.UUID) (*models.Return, error)
	GetReturn(ctx context.Context, tenantID uuid.UUID, returnID uuid.UUID) (*models.Return, error)
	GetReturnsByOrder(ctx context.Context, tenantID uuid.UUID, orderID uuid.UUID) ([]models.Return, error)
	CreatePayment(ctx context.Context, tenantID uuid.UUID, orderID uuid.UUID, amount float64, paymentMethod string, referenceNumber string, notes string) (*models.Payment, error)
	GetPayments(ctx context.Context, tenantID uuid.UUID, orderID uuid.UUID) ([]models.Payment, error)
	GetPaymentByID(ctx context.Context, tenantID uuid.UUID, paymentID uuid.UUID) (*models.Payment, error)
}
