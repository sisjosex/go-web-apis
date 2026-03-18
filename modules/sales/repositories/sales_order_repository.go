package repositories

import (
	"context"
	"time"

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
func (r *SalesOrderRepository) CreateSalesOrder(ctx context.Context, dto *models.CreateSalesOrderRequestDto) (*models.SalesOrder, error) {
	order := &models.SalesOrder{}
	err := r.dbService.QueryRow(ctx,
		`SELECT * FROM sales.sp_create_sales_order($1, $2, $3, $4)`,
		dto.CustomerID,
		dto.ShippingAddress,
		dto.Notes,
		dto.DiscountAmount,
	).Scan(
		&order.ID, &order.CustomerID, &order.OrderNumber,
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
		`SELECT * FROM sales.sp_get_sales_order_by_id($1)`,
		id,
	).Scan(
		&order.ID, &order.CustomerID, &order.OrderNumber,
		&order.Status, &order.SubTotal, &order.TaxAmount, &order.Total,
		&order.DiscountAmount, &order.ShippingAddress, &order.Notes,
		&order.CreatedAt, &order.UpdatedAt,
	)
	return order, err
}

// GetAllSalesOrders retrieves all sales orders
func (r *SalesOrderRepository) GetAllSalesOrders(ctx context.Context, limit int, offset int) ([]models.SalesOrder, error) {
	var orders []models.SalesOrder
	rows, err := r.dbService.Query(ctx,
		`SELECT * FROM sales.sp_get_all_sales_orders($1, $2)`,
		limit, offset,
	)
	if err != nil {
		return orders, err
	}
	defer rows.Close()

	for rows.Next() {
		var o models.SalesOrder
		err := rows.Scan(
			&o.ID, &o.CustomerID, &o.OrderNumber,
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
		`SELECT * FROM sales.sp_update_sales_order($1, $2, $3, $4, $5)`,
		id, dto.Status, dto.ShippingAddress, dto.Notes, dto.DiscountAmount,
	).Scan(
		&order.ID, &order.CustomerID, &order.OrderNumber,
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
		`SELECT * FROM sales.sp_get_sales_orders_by_customer($1)`,
		customerID,
	)
	if err != nil {
		return orders, err
	}
	defer rows.Close()

	for rows.Next() {
		var o models.SalesOrder
		err := rows.Scan(
			&o.ID, &o.CustomerID, &o.OrderNumber,
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
func (r *SalesOrderRepository) GetSalesOrderByOrderNumber(ctx context.Context, orderNumber string) (*models.SalesOrder, error) {
	order := &models.SalesOrder{}
	err := r.dbService.QueryRow(ctx,
		`SELECT * FROM sales.sp_get_sales_order_by_number($1)`,
		orderNumber,
	).Scan(
		&order.ID, &order.CustomerID, &order.OrderNumber,
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

// ========== PHASE 1: Batch Assignment & Order Completion ==========

// AddOrderItemWithBatch adds an item to an order with FIFO batch assignment
func (r *SalesOrderRepository) AddOrderItemWithBatch(ctx context.Context, orderID uuid.UUID, productID uuid.UUID, quantity float64, unitPrice float64) (*models.OrderItem, error) {
	var item models.OrderItem
	row := r.dbService.QueryRow(ctx,
		`SELECT id, order_id, product_id, product_sku, product_name, quantity, unit_price, line_total, created_at
		 FROM sales.sp_add_order_item_with_batch($1, $2, $3, $4)`,
		orderID, productID, quantity, unitPrice,
	)

	err := row.Scan(
		&item.ID, &item.OrderID, &item.ProductID, &item.ProductSku,
		&item.ProductName, &item.Quantity, &item.UnitPrice, &item.LineTotal, &item.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

// CompleteOrder completes an order and consumes inventory from batches
func (r *SalesOrderRepository) CompleteOrder(ctx context.Context, orderID uuid.UUID) (*models.SalesOrder, error) {
	var order models.SalesOrder
	row := r.dbService.QueryRow(ctx,
		`SELECT order_id, order_number, customer_id, status, sub_total, tax_amount, total, discount_amount, shipping_address, notes, created_at, updated_at
		 FROM sales.sp_complete_sales_order($1)`,
		orderID,
	)

	err := row.Scan(
		&order.ID, &order.OrderNumber, &order.CustomerID, &order.Status,
		&order.SubTotal, &order.TaxAmount, &order.Total, &order.DiscountAmount,
		&order.ShippingAddress, &order.Notes, &order.CreatedAt, &order.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &order, nil
}

// ========== PHASE 2: Reporting & Cancellation ==========

// CancelOrder cancels an order and releases batch assignments
func (r *SalesOrderRepository) CancelOrder(ctx context.Context, orderID uuid.UUID) (*models.SalesOrder, error) {
	var order models.SalesOrder
	row := r.dbService.QueryRow(ctx,
		`SELECT id, order_number, customer_id, status, sub_total, tax_amount, total, discount_amount, shipping_address, notes, created_at, updated_at
		 FROM sales.sp_cancel_sales_order($1)`,
		orderID,
	)

	err := row.Scan(
		&order.ID, &order.OrderNumber, &order.CustomerID, &order.Status,
		&order.SubTotal, &order.TaxAmount, &order.Total, &order.DiscountAmount,
		&order.ShippingAddress, &order.Notes, &order.CreatedAt, &order.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &order, nil
}

// GetOrderWithBatches retrieves an order with batch assignment details
func (r *SalesOrderRepository) GetOrderWithBatches(ctx context.Context, orderID uuid.UUID) (*models.OrderWithBatches, error) {
	var order models.OrderWithBatches
	row := r.dbService.QueryRow(ctx,
		`SELECT order_id, order_number, customer_id, customer_name, status, sub_total, tax_amount, total, discount_amount, shipping_address, item_count, batch_count, created_at, updated_at
		 FROM sales.sp_get_order_with_batches($1)`,
		orderID,
	)

	err := row.Scan(
		&order.OrderID, &order.OrderNumber, &order.CustomerID, &order.CustomerName, &order.Status,
		&order.SubTotal, &order.TaxAmount, &order.Total, &order.DiscountAmount, &order.ShippingAddress,
		&order.ItemCount, &order.BatchCount, &order.CreatedAt, &order.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &order, nil
}

// GetSalesReport retrieves sales metrics for a date range
func (r *SalesOrderRepository) GetSalesReport(ctx context.Context, startDate time.Time, endDate time.Time) ([]models.SalesReport, error) {
	var reports []models.SalesReport
	rows, err := r.dbService.Query(ctx,
		`SELECT metric_name, metric_value, metric_type FROM sales.sp_get_sales_report($1::DATE, $2::DATE)`,
		startDate, endDate,
	)
	if err != nil {
		return reports, err
	}
	defer rows.Close()

	for rows.Next() {
		var report models.SalesReport
		err := rows.Scan(&report.MetricName, &report.MetricValue, &report.MetricType)
		if err != nil {
			continue
		}
		reports = append(reports, report)
	}
	return reports, nil
}

// ========== PHASE 3: Returns & Payments ==========

// CreateReturn creates a return request for a completed order
func (r *SalesOrderRepository) CreateReturn(ctx context.Context, orderID uuid.UUID, reason string) (*models.Return, error) {
	var ret models.Return
	row := r.dbService.QueryRow(ctx,
		`SELECT id, order_id, customer_id, return_number, total_amount, reason, status, created_at, updated_at
		 FROM sales.sp_create_return($1, $2)`,
		orderID, reason,
	)

	err := row.Scan(
		&ret.ID, &ret.OrderID, &ret.CustomerID, &ret.ReturnNumber, &ret.TotalAmount,
		&ret.Reason, &ret.Status, &ret.CreatedAt, &ret.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &ret, nil
}

// ApproveReturn approves a return and restores inventory
func (r *SalesOrderRepository) ApproveReturn(ctx context.Context, returnID uuid.UUID) (*models.Return, error) {
	var ret models.Return
	row := r.dbService.QueryRow(ctx,
		`SELECT id, order_id, customer_id, return_number, total_amount, reason, status, created_at, updated_at
		 FROM sales.sp_approve_return($1)`,
		returnID,
	)

	err := row.Scan(
		&ret.ID, &ret.OrderID, &ret.CustomerID, &ret.ReturnNumber, &ret.TotalAmount,
		&ret.Reason, &ret.Status, &ret.CreatedAt, &ret.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &ret, nil
}

// GetReturn retrieves a return by ID
func (r *SalesOrderRepository) GetReturn(ctx context.Context, returnID uuid.UUID) (*models.Return, error) {
	var ret models.Return
	row := r.dbService.QueryRow(ctx,
		`SELECT id, order_id, customer_id, return_number, total_amount, reason, status, created_at, updated_at
		 FROM sales.sp_get_return($1)`,
		returnID,
	)

	err := row.Scan(
		&ret.ID, &ret.OrderID, &ret.CustomerID, &ret.ReturnNumber, &ret.TotalAmount,
		&ret.Reason, &ret.Status, &ret.CreatedAt, &ret.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &ret, nil
}

// GetReturnsByOrder retrieves all returns for an order
func (r *SalesOrderRepository) GetReturnsByOrder(ctx context.Context, orderID uuid.UUID) ([]models.Return, error) {
	var returns []models.Return
	rows, err := r.dbService.Query(ctx,
		`SELECT id, order_id, customer_id, return_number, total_amount, reason, status, created_at, updated_at
		 FROM sales.sp_get_returns_by_order($1)`,
		orderID,
	)
	if err != nil {
		return returns, err
	}
	defer rows.Close()

	for rows.Next() {
		var ret models.Return
		err := rows.Scan(
			&ret.ID, &ret.OrderID, &ret.CustomerID, &ret.ReturnNumber, &ret.TotalAmount,
			&ret.Reason, &ret.Status, &ret.CreatedAt, &ret.UpdatedAt,
		)
		if err != nil {
			continue
		}
		returns = append(returns, ret)
	}
	return returns, nil
}

// CreatePayment creates a payment record for an order
func (r *SalesOrderRepository) CreatePayment(ctx context.Context, orderID uuid.UUID, amount float64, paymentMethod string, referenceNumber string, notes string) (*models.Payment, error) {
	var payment models.Payment
	row := r.dbService.QueryRow(ctx,
		`SELECT id, order_id, customer_id, amount, payment_method, status, reference_number, notes, created_at, updated_at
		 FROM sales.sp_create_payment($1, $2, $3, $4, $5, $6)`,
		orderID, amount, paymentMethod, referenceNumber, notes,
	)

	err := row.Scan(
		&payment.ID, &payment.OrderID, &payment.CustomerID, &payment.Amount, &payment.PaymentMethod,
		&payment.Status, &payment.ReferenceNumber, &payment.Notes, &payment.CreatedAt, &payment.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &payment, nil
}

// GetPayments retrieves all payments for an order
func (r *SalesOrderRepository) GetPayments(ctx context.Context, orderID uuid.UUID) ([]models.Payment, error) {
	var payments []models.Payment
	rows, err := r.dbService.Query(ctx,
		`SELECT id, order_id, customer_id, amount, payment_method, status, reference_number, notes, created_at, updated_at
		 FROM sales.sp_get_payments($1)`,
		orderID,
	)
	if err != nil {
		return payments, err
	}
	defer rows.Close()

	for rows.Next() {
		var payment models.Payment
		err := rows.Scan(
			&payment.ID, &payment.OrderID, &payment.CustomerID, &payment.Amount, &payment.PaymentMethod,
			&payment.Status, &payment.ReferenceNumber, &payment.Notes, &payment.CreatedAt, &payment.UpdatedAt,
		)
		if err != nil {
			continue
		}
		payments = append(payments, payment)
	}
	return payments, nil
}

// GetPaymentByID retrieves a payment by ID
func (r *SalesOrderRepository) GetPaymentByID(ctx context.Context, paymentID uuid.UUID) (*models.Payment, error) {
	var payment models.Payment
	row := r.dbService.QueryRow(ctx,
		`SELECT id, order_id, customer_id, amount, payment_method, status, reference_number, notes, created_at, updated_at
		 FROM sales.sp_get_payment_by_id($1)`,
		paymentID,
	)

	err := row.Scan(
		&payment.ID, &payment.OrderID, &payment.CustomerID, &payment.Amount, &payment.PaymentMethod,
		&payment.Status, &payment.ReferenceNumber, &payment.Notes, &payment.CreatedAt, &payment.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &payment, nil
}
