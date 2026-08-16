package models

import (
	"time"

	"github.com/google/uuid"
)

// SalesOrder represents a customer order
type SalesOrder struct {
	ID              uuid.UUID   `json:"id"`
	CustomerID      uuid.UUID   `json:"customer_id"`
	OrderNumber     string      `json:"order_number"`
	Status          string      `json:"status"` // pending, confirmed, shipped, delivered, cancelled
	SubTotal        float64     `json:"sub_total"`
	TaxAmount       float64     `json:"tax_amount"`
	Total           float64     `json:"total"`
	DiscountAmount  float64     `json:"discount_amount"`
	ShippingAddress string      `json:"shipping_address"`
	Notes           string      `json:"notes"`
	Items           []OrderItem `json:"items,omitempty"`
	CreatedAt       time.Time   `json:"created_at"`
	UpdatedAt       time.Time   `json:"updated_at"`
}

// OrderItem represents an item in a sales order
type OrderItem struct {
	ID        uuid.UUID `json:"id"`
	OrderID   uuid.UUID `json:"order_id"`
	ProductID uuid.UUID `json:"product_id"`
	// SkuID is the sellable combination this line sold; never null since INV-010.
	SkuID uuid.UUID `json:"sku_id"`
	// ProductSku is the combination's code as it stood when the line was sold,
	// Sku the same combination's code as it stands now (INV-010 D4).
	ProductSku  string    `json:"product_sku"`
	Sku         string    `json:"sku"`
	ProductName string    `json:"product_name"`
	Quantity    int       `json:"quantity"`
	UnitPrice   float64   `json:"unit_price"`
	LineTotal   float64   `json:"line_total"`
	CreatedAt   time.Time `json:"created_at"`
}

// CreateSalesOrderRequestDto DTO for creating a sales order
type CreateSalesOrderRequestDto struct {
	CustomerID      string                      `json:"customer_id" binding:"required,uuidv4"`
	ShippingAddress string                      `json:"shipping_address" binding:"required,min=5,max=500"`
	Notes           string                      `json:"notes" binding:"omitempty,max=1000"`
	Items           []CreateOrderItemRequestDto `json:"items" binding:"required,min=1"`
	DiscountAmount  *float64                    `json:"discount_amount" binding:"omitempty,min=0"`
}

// CreateOrderItemRequestDto DTO for order items
type CreateOrderItemRequestDto struct {
	ProductID string `json:"product_id" binding:"required,uuidv4"`
	Quantity  int    `json:"quantity" binding:"required,min=1,max=9999"`
}

// UpdateSalesOrderRequestDto DTO for updating an order
type UpdateSalesOrderRequestDto struct {
	Status          *string  `json:"status" binding:"omitempty"`
	ShippingAddress *string  `json:"shipping_address" binding:"omitempty,min=5,max=500"`
	Notes           *string  `json:"notes" binding:"omitempty,max=1000"`
	DiscountAmount  *float64 `json:"discount_amount" binding:"omitempty,min=0"`
}
