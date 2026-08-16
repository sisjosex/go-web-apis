package models

import (
	"time"

	"github.com/google/uuid"
)

// OrderBatchAssignment links an order item to a product batch (FIFO fulfillment)
type OrderBatchAssignment struct {
	ID               uuid.UUID `json:"id"`
	OrderID          uuid.UUID `json:"order_id"`
	OrderItemID      uuid.UUID `json:"order_item_id"`
	ProductBatchID   uuid.UUID `json:"product_batch_id"`
	QuantityAssigned float64   `json:"quantity_assigned"`
	CreatedAt        time.Time `json:"created_at"`
}

// Return represents a customer return request
type Return struct {
	ID           uuid.UUID `json:"id"`
	OrderID      uuid.UUID `json:"order_id"`
	CustomerID   uuid.UUID `json:"customer_id"`
	ReturnNumber string    `json:"return_number"`
	TotalAmount  float64   `json:"total_amount"`
	Reason       string    `json:"reason,omitempty"`
	Status       string    `json:"status"` // pending, approved, rejected, completed
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// CreateReturnRequestDto for creating a return
type CreateReturnRequestDto struct {
	OrderID uuid.UUID `json:"order_id" binding:"required,uuidv4"`
	Reason  string    `json:"reason" binding:"required,min=1,max=255"`
}

// ApproveReturnRequestDto for approving a return
type ApproveReturnRequestDto struct {
	Approved bool   `json:"approved" binding:"required"`
	Notes    string `json:"notes,omitempty"`
}

// Payment represents an order payment
type Payment struct {
	ID              uuid.UUID `json:"id"`
	OrderID         uuid.UUID `json:"order_id"`
	CustomerID      uuid.UUID `json:"customer_id"`
	Amount          float64   `json:"amount"`
	PaymentMethod   string    `json:"payment_method"` // cash, card, bank_transfer, check, other
	Status          string    `json:"status"`         // pending, completed, failed, refunded
	ReferenceNumber string    `json:"reference_number,omitempty"`
	Notes           string    `json:"notes,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// CreatePaymentRequestDto for creating a payment
type CreatePaymentRequestDto struct {
	OrderID         uuid.UUID `json:"order_id" binding:"required,uuidv4"`
	Amount          float64   `json:"amount" binding:"required,gt=0"`
	PaymentMethod   string    `json:"payment_method" binding:"required,oneof=cash card bank_transfer check other"`
	ReferenceNumber string    `json:"reference_number,omitempty"`
	Notes           string    `json:"notes,omitempty"`
}

// RequestItemDto for adding items to a sales order with batch assignment
type AddOrderItemRequestDto struct {
	ProductID uuid.UUID `json:"product_id" binding:"required,uuidv4"`
	Quantity  float64   `json:"quantity" binding:"required,gt=0"`
	UnitPrice float64   `json:"unit_price" binding:"required,gte=0"`
	// SkuID names the combination to sell. Optional (INV-010 D2): omitted, the SP
	// resolves the product's default SKU, which is what every pre-INV-010 client
	// keeps getting. On a stock_by_variant product, omitting it is a 400.
	SkuID *uuid.UUID `json:"sku_id" binding:"omitempty,uuidv4"`
}

// SalesReport represents sales metrics
type SalesReport struct {
	MetricName  string  `json:"metric_name"`
	MetricValue float64 `json:"metric_value"`
	MetricType  string  `json:"metric_type"` // count, currency, percentage
}

// SalesReportResponse wraps multiple metrics
type SalesReportResponse struct {
	StartDate time.Time     `json:"start_date"`
	EndDate   time.Time     `json:"end_date"`
	Metrics   []SalesReport `json:"metrics"`
	Message   string        `json:"message"`
}

// OrderWithBatches represents an order with batch assignment details
type OrderWithBatches struct {
	OrderID         uuid.UUID `json:"order_id"`
	OrderNumber     string    `json:"order_number"`
	CustomerID      uuid.UUID `json:"customer_id"`
	CustomerName    string    `json:"customer_name"`
	Status          string    `json:"status"`
	SubTotal        float64   `json:"sub_total"`
	TaxAmount       float64   `json:"tax_amount"`
	Total           float64   `json:"total"`
	DiscountAmount  float64   `json:"discount_amount"`
	ShippingAddress string    `json:"shipping_address"`
	ItemCount       int       `json:"item_count"`
	BatchCount      int       `json:"batch_count"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}
