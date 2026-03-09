package models

// DTOs for Purchase Order APIs

// CreateSupplierRequestDto DTO for creating a supplier
type CreateSupplierRequestDto struct {
	Name          string  `json:"name" binding:"required,min=3,max=255"`
	ContactPerson *string `json:"contact_person" binding:"omitempty,max=255"`
	Email         *string `json:"email" binding:"omitempty,email"`
	Phone         *string `json:"phone" binding:"omitempty,max=20"`
	Address       *string `json:"address" binding:"omitempty,max=500"`
	PaymentTerms  *int    `json:"payment_terms" binding:"omitempty,min=0"`
}

// UpdateSupplierRequestDto DTO for updating a supplier
type UpdateSupplierRequestDto struct {
	Name          *string `json:"name" binding:"omitempty,min=3,max=255"`
	ContactPerson *string `json:"contact_person" binding:"omitempty,max=255"`
	Email         *string `json:"email" binding:"omitempty,email"`
	Phone         *string `json:"phone" binding:"omitempty,max=20"`
	Address       *string `json:"address" binding:"omitempty,max=500"`
	PaymentTerms  *int    `json:"payment_terms" binding:"omitempty,min=0"`
	IsActive      *bool   `json:"is_active"`
}

// CreatePurchaseOrderRequestDto DTO for creating a PO
type CreatePurchaseOrderRequestDto struct {
	SupplierID           string  `json:"supplier_id" binding:"required,uuidv4"`
	ExpectedDeliveryDate *string `json:"expected_delivery_date" binding:"omitempty"` // YYYY-MM-DD
	Notes                *string `json:"notes" binding:"omitempty,max=1000"`
}

// AddPurchaseOrderItemRequestDto DTO for adding items to PO
type AddPurchaseOrderItemRequestDto struct {
	ProductID string  `json:"product_id" binding:"required,uuidv4"`
	Quantity  int     `json:"quantity" binding:"required,min=1"`
	UnitCost  float64 `json:"unit_cost" binding:"required,gt=0"`
}

// ApprovePurchaseOrderRequestDto DTO for approving PO
type ApprovePurchaseOrderRequestDto struct {
	// No fields needed, just the ID from URL
}

// ReceivePurchaseOrderRequestDto DTO for receiving PO items
type ReceivePurchaseOrderRequestDto struct {
	ReceivedBy *string `json:"received_by" binding:"omitempty,uuidv4"`
	Notes      *string `json:"notes" binding:"omitempty,max=1000"`
}

// AddInvoiceRequestDto DTO for adding invoice to PO
type AddInvoiceRequestDto struct {
	InvoiceNumber string   `json:"invoice_number" binding:"required,max=100"`
	InvoiceDate   string   `json:"invoice_date" binding:"required"` // YYYY-MM-DD
	InvoiceAmount float64  `json:"invoice_amount" binding:"required,gt=0"`
	TaxAmount     *float64 `json:"tax_amount" binding:"omitempty,min=0"`
	DueDate       string   `json:"due_date" binding:"required"` // YYYY-MM-DD
}

// PHASE 2A DTOs

// GetPriceComparisonRequestDto DTO for price comparison query
type GetPriceComparisonRequestDto struct {
	ProductID string `form:"product_id" binding:"required,uuidv4"`
}

// CreateRFQRequestDto DTO for creating RFQ
type CreateRFQRequestDto struct {
	SupplierIDs []string         `json:"supplier_ids" binding:"required,min=1"`
	Items       []RFQItemRequest `json:"items" binding:"required,min=1"`
	Notes       *string          `json:"notes" binding:"omitempty,max=1000"`
}

// RFQItemRequest DTO for RFQ items
type RFQItemRequest struct {
	ProductID   string  `json:"product_id" binding:"required,uuidv4"`
	Quantity    int     `json:"quantity" binding:"required,min=1"`
	Description *string `json:"description" binding:"omitempty,max=500"`
}

// Response DTOs

type CreateSupplierResponse struct {
	SupplierID string `json:"supplier_id"`
	Name       string `json:"name"`
	Message    string `json:"message"`
}

type CreatePurchaseOrderResponse struct {
	POID        string  `json:"po_id"`
	PONumber    string  `json:"po_number"`
	SupplierID  string  `json:"supplier_id"`
	Status      string  `json:"status"`
	TotalAmount float64 `json:"total_amount"`
	Message     string  `json:"message"`
}

type AddPurchaseOrderItemResponse struct {
	ItemID    string  `json:"item_id"`
	PoID      string  `json:"po_id"`
	ProductID string  `json:"product_id"`
	Quantity  int     `json:"quantity"`
	UnitCost  float64 `json:"unit_cost"`
	LineTotal float64 `json:"line_total"`
	Message   string  `json:"message"`
}

type PurchaseOrderActionResponse struct {
	POID        string  `json:"po_id"`
	PONumber    string  `json:"po_number"`
	Status      string  `json:"status"`
	TotalAmount float64 `json:"total_amount"`
	Message     string  `json:"message"`
}
