package models

import (
	"time"

	"github.com/google/uuid"
)

// Supplier represents a vendor
type Supplier struct {
	ID            uuid.UUID `json:"id"`
	Name          string    `json:"name"`
	ContactPerson *string   `json:"contact_person"`
	Email         *string   `json:"email"`
	Phone         *string   `json:"phone"`
	Address       *string   `json:"address"`
	PaymentTerms  int       `json:"payment_terms"` // Days credit
	IsActive      bool      `json:"is_active"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// PurchaseOrder represents a purchase order from suppliers
type PurchaseOrder struct {
	ID                   uuid.UUID              `json:"id"`
	SupplierID           uuid.UUID              `json:"supplier_id"`
	PONumber             string                 `json:"po_number"`
	Status               string                 `json:"status"` // draft, approved, received, invoiced, paid, cancelled
	OrderDate            time.Time              `json:"order_date"`
	ExpectedDeliveryDate *time.Time             `json:"expected_delivery_date"`
	TotalAmount          float64                `json:"total_amount"`
	PaidAmount           float64                `json:"paid_amount"`
	Notes                *string                `json:"notes"`
	CreatedBy            *uuid.UUID             `json:"created_by"`
	Items                []PurchaseOrderItem    `json:"items,omitempty"`
	Invoices             []PurchaseOrderInvoice `json:"invoices,omitempty"`
	CreatedAt            time.Time              `json:"created_at"`
	UpdatedAt            time.Time              `json:"updated_at"`
}

// PurchaseOrderItem represents an item in a purchase order
type PurchaseOrderItem struct {
	ID               uuid.UUID `json:"id"`
	PurchaseOrderID  uuid.UUID `json:"purchase_order_id"`
	ProductID        uuid.UUID `json:"product_id"`
	Quantity         int       `json:"quantity"`
	UnitCost         float64   `json:"unit_cost"`
	LineTotal        float64   `json:"line_total"`
	ReceivedQuantity int       `json:"received_quantity"`
	Status           string    `json:"status"` // pending, partial, received
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// PurchaseOrderReceipt represents goods received
type PurchaseOrderReceipt struct {
	ID              uuid.UUID  `json:"id"`
	PurchaseOrderID uuid.UUID  `json:"purchase_order_id"`
	ReceiptNumber   string     `json:"receipt_number"`
	ReceiptDate     time.Time  `json:"receipt_date"`
	ReceivedBy      *uuid.UUID `json:"received_by"`
	Notes           *string    `json:"notes"`
	CreatedAt       time.Time  `json:"created_at"`
}

// PurchaseOrderInvoice represents supplier invoice for payment
type PurchaseOrderInvoice struct {
	ID              uuid.UUID `json:"id"`
	PurchaseOrderID uuid.UUID `json:"purchase_order_id"`
	InvoiceNumber   string    `json:"invoice_number"`
	InvoiceDate     time.Time `json:"invoice_date"`
	InvoiceAmount   float64   `json:"invoice_amount"`
	TaxAmount       float64   `json:"tax_amount"`
	DueDate         time.Time `json:"due_date"`
	Status          string    `json:"status"` // received, validated, partially_paid, paid, disputed
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// PHASE 2A: Price Comparison, FIFO, RFQ

// PriceComparison represents price comparison across suppliers
type PriceComparison struct {
	ProductID      uuid.UUID `json:"product_id"`
	SupplierID     uuid.UUID `json:"supplier_id"`
	SupplierName   string    `json:"supplier_name"`
	UnitCost       float64   `json:"unit_cost"`
	PaymentTerms   int       `json:"payment_terms"`
	LastPurchaseAt time.Time `json:"last_purchase_at"`
	AverageCost    float64   `json:"average_cost"`
}

// ProductBatch represents batch for FIFO tracking
type ProductBatch struct {
	ID             uuid.UUID  `json:"id"`
	ProductID      uuid.UUID  `json:"product_id"`
	BatchNumber    string     `json:"batch_number"`
	Quantity       int        `json:"quantity"`
	UnitCost       float64    `json:"unit_cost"`
	ReceiptDate    time.Time  `json:"receipt_date"`
	ExpirationDate *time.Time `json:"expiration_date"`
	Status         string     `json:"status"` // received, partial_sold, sold, expired
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// RequestForQuote represents a quote request to suppliers
type RequestForQuote struct {
	ID        uuid.UUID     `json:"id"`
	RFQNumber string        `json:"rfq_number"`
	Status    string        `json:"status"` // draft, sent, responded, closed
	Items     []RFQItem     `json:"items"`
	Responses []RFQResponse `json:"responses"`
	Notes     *string       `json:"notes"`
	CreatedBy *uuid.UUID    `json:"created_by"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
}

// RFQItem represents an item in a quote request
type RFQItem struct {
	ID          uuid.UUID `json:"id"`
	RFQID       uuid.UUID `json:"rfq_id"`
	ProductID   uuid.UUID `json:"product_id"`
	Quantity    int       `json:"quantity"`
	Description *string   `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

// RFQResponse represents supplier response to RFQ
type RFQResponse struct {
	ID           uuid.UUID `json:"id"`
	RFQID        uuid.UUID `json:"rfq_id"`
	SupplierID   uuid.UUID `json:"supplier_id"`
	SupplierName string    `json:"supplier_name"`
	TotalPrice   float64   `json:"total_price"`
	DeliveryDays int       `json:"delivery_days"`
	PaymentTerms int       `json:"payment_terms"`
	Notes        *string   `json:"notes"`
	Status       string    `json:"status"` // pending, received, accepted, rejected
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
