package repositories

import (
	"context"

	coreServices "josex/web/modules/core/services"
	"josex/web/modules/sales/models"

	"github.com/google/uuid"
)

// SalesOrderRepository implements sales order data operations
type SalesOrderRepository struct {
	dbService coreServices.DatabaseService
}

// NewSalesOrderRepository creates a new sales order repository
func NewSalesOrderRepository(dbService coreServices.DatabaseService) *SalesOrderRepository {
	return &SalesOrderRepository{
		dbService: dbService,
	}
}

// CreateSalesOrder creates a new sales order with items
func (r *SalesOrderRepository) CreateSalesOrder(ctx context.Context, tenantID uuid.UUID, dto *models.CreateSalesOrderRequestDto) (*models.SalesOrder, error) {
	order := &models.SalesOrder{}
	err := r.dbService.QueryRow(ctx,
		`CALL sales.sp_create_sales_order($1, $2, $3, $4, $5)`,
		tenantID,
		dto.CustomerID,
		dto.ShippingAddress,
		dto.Notes,
		dto.DiscountAmount,
	).Scan(
		&order.ID, &order.TenantID, &order.CustomerID, &order.OrderNumber,
		&order.Status, &order.SubTotal, &order.TaxAmount, &order.Total,
		&order.DiscountAmount, &order.ShippingAddress, &order.Notes,
		&order.CreatedAt, &order.UpdatedAt,
	)
	return order, err
}

// GetSalesOrderByID retrieves a sales order by ID
func (r *SalesOrderRepository) GetSalesOrderByID(ctx context.Context, id uuid.UUID) (*models.SalesOrder, error) {
	order := &models.SalesOrder{}
	err := r.dbService.QueryRow(ctx,
		`CALL sales.sp_get_sales_order_by_id($1)`,
		id,
	).Scan(
		&order.ID, &order.TenantID, &order.CustomerID, &order.OrderNumber,
		&order.Status, &order.SubTotal, &order.TaxAmount, &order.Total,
		&order.DiscountAmount, &order.ShippingAddress, &order.Notes,
		&order.CreatedAt, &order.UpdatedAt,
	)
	return order, err
}

// GetSalesOrdersByTenant retrieves all sales orders for a tenant
func (r *SalesOrderRepository) GetSalesOrdersByTenant(ctx context.Context, tenantID uuid.UUID, limit int, offset int) ([]models.SalesOrder, error) {
	var orders []models.SalesOrder
	rows, err := r.dbService.Query(ctx,
		`CALL sales.sp_get_sales_orders_by_tenant($1, $2, $3)`,
		tenantID, limit, offset,
	)
	if err != nil {
		return orders, err
	}
	defer rows.Close()

	for rows.Next() {
		var o models.SalesOrder
		err := rows.Scan(
			&o.ID, &o.TenantID, &o.CustomerID, &o.OrderNumber,
			&o.Status, &o.SubTotal, &o.TaxAmount, &o.Total,
			&o.DiscountAmount, &o.ShippingAddress, &o.Notes,
			&o.CreatedAt, &o.UpdatedAt,
		)
		if err != nil {
			continue
		}
		orders = append(orders, o)
	}
	return orders, nil
}

// UpdateSalesOrder updates a sales order
func (r *SalesOrderRepository) UpdateSalesOrder(ctx context.Context, id uuid.UUID, dto *models.UpdateSalesOrderRequestDto) (*models.SalesOrder, error) {
	order := &models.SalesOrder{}
	err := r.dbService.QueryRow(ctx,
		`CALL sales.sp_update_sales_order($1, $2, $3, $4, $5)`,
		id, dto.Status, dto.ShippingAddress, dto.Notes, dto.DiscountAmount,
	).Scan(
		&order.ID, &order.TenantID, &order.CustomerID, &order.OrderNumber,
		&order.Status, &order.SubTotal, &order.TaxAmount, &order.Total,
		&order.DiscountAmount, &order.ShippingAddress, &order.Notes,
		&order.CreatedAt, &order.UpdatedAt,
	)
	return order, err
}

// GetSalesOrdersByCustomer retrieves all orders for a customer
func (r *SalesOrderRepository) GetSalesOrdersByCustomer(ctx context.Context, customerID uuid.UUID) ([]models.SalesOrder, error) {
	var orders []models.SalesOrder
	rows, err := r.dbService.Query(ctx,
		`CALL sales.sp_get_sales_orders_by_customer($1)`,
		customerID,
	)
	if err != nil {
		return orders, err
	}
	defer rows.Close()

	for rows.Next() {
		var o models.SalesOrder
		err := rows.Scan(
			&o.ID, &o.TenantID, &o.CustomerID, &o.OrderNumber,
			&o.Status, &o.SubTotal, &o.TaxAmount, &o.Total,
			&o.DiscountAmount, &o.ShippingAddress, &o.Notes,
			&o.CreatedAt, &o.UpdatedAt,
		)
		if err != nil {
			continue
		}
		orders = append(orders, o)
	}
	return orders, nil
}

// GetSalesOrderByOrderNumber retrieves an order by order number
func (r *SalesOrderRepository) GetSalesOrderByOrderNumber(ctx context.Context, orderNumber string, tenantID uuid.UUID) (*models.SalesOrder, error) {
	order := &models.SalesOrder{}
	err := r.dbService.QueryRow(ctx,
		`CALL sales.sp_get_sales_order_by_number($1, $2)`,
		orderNumber, tenantID,
	).Scan(
		&order.ID, &order.TenantID, &order.CustomerID, &order.OrderNumber,
		&order.Status, &order.SubTotal, &order.TaxAmount, &order.Total,
		&order.DiscountAmount, &order.ShippingAddress, &order.Notes,
		&order.CreatedAt, &order.UpdatedAt,
	)
	return order, err
}

// AddOrderItem adds an item to an order
func (r *SalesOrderRepository) AddOrderItem(ctx context.Context, orderID uuid.UUID, dto *models.CreateOrderItemRequestDto) (*models.OrderItem, error) {
	item := &models.OrderItem{}
	err := r.dbService.QueryRow(ctx,
		`CALL sales.sp_add_order_item($1, $2, $3)`,
		orderID, dto.ProductID, dto.Quantity,
	).Scan(
		&item.ID, &item.OrderID, &item.ProductID, &item.ProductSku,
		&item.ProductName, &item.Quantity, &item.UnitPrice, &item.LineTotal, &item.CreatedAt,
	)
	return item, err
}

// GetOrderItems retrieves all items for an order
func (r *SalesOrderRepository) GetOrderItems(ctx context.Context, orderID uuid.UUID) ([]models.OrderItem, error) {
	var items []models.OrderItem
	rows, err := r.dbService.Query(ctx,
		`CALL sales.sp_get_order_items($1)`,
		orderID,
	)
	if err != nil {
		return items, err
	}
	defer rows.Close()

	for rows.Next() {
		var item models.OrderItem
		err := rows.Scan(
			&item.ID, &item.OrderID, &item.ProductID, &item.ProductSku,
			&item.ProductName, &item.Quantity, &item.UnitPrice, &item.LineTotal, &item.CreatedAt,
		)
		if err != nil {
			continue
		}
		items = append(items, item)
	}
	return items, nil
}
