package interfaces

import (
	"context"

	"josex/web/modules/purchasing/models"

	"github.com/google/uuid"
)

// PurchasingRepository interface defines methods for purchasing data access
type PurchasingRepository interface {
	// Supplier operations
	CreateSupplier(ctx context.Context, tenantID uuid.UUID, dto *models.CreateSupplierRequestDto) (*models.Supplier, error)
	GetSupplier(ctx context.Context, tenantID uuid.UUID, supplierID uuid.UUID) (*models.Supplier, error)
	UpdateSupplier(ctx context.Context, tenantID uuid.UUID, supplierID uuid.UUID, dto *models.UpdateSupplierRequestDto) (*models.Supplier, error)
	ListSuppliers(ctx context.Context, tenantID uuid.UUID, page, pageSize int) ([]models.Supplier, int, error)
	DeleteSupplier(ctx context.Context, tenantID uuid.UUID, supplierID uuid.UUID) error

	// Purchase Order operations
	CreatePurchaseOrder(ctx context.Context, tenantID uuid.UUID, supplierID uuid.UUID, expectedDelivery *string, notes *string, createdBy uuid.UUID) (*models.PurchaseOrder, error)
	GetPurchaseOrder(ctx context.Context, tenantID uuid.UUID, poID uuid.UUID) (*models.PurchaseOrder, error)
	ListPurchaseOrders(ctx context.Context, tenantID uuid.UUID, status *string, page, pageSize int) ([]models.PurchaseOrder, int, error)
	ApprovePurchaseOrder(ctx context.Context, tenantID uuid.UUID, poID uuid.UUID) (*models.PurchaseOrder, error)

	// PO Items
	AddPurchaseOrderItem(ctx context.Context, tenantID uuid.UUID, poID, productID uuid.UUID, quantity int, unitCost float64) (*models.PurchaseOrderItem, error)
	GetPurchaseOrderItems(ctx context.Context, tenantID uuid.UUID, poID uuid.UUID) ([]models.PurchaseOrderItem, error)

	// PO Receipt
	ReceivePurchaseOrder(ctx context.Context, tenantID uuid.UUID, poID uuid.UUID, receivedBy *uuid.UUID, notes *string) (*models.PurchaseOrderReceipt, error)

	// PO Invoice
	AddInvoice(ctx context.Context, tenantID uuid.UUID, poID uuid.UUID, dto *models.AddInvoiceRequestDto) (*models.PurchaseOrderInvoice, error)
	GetInvoices(ctx context.Context, tenantID uuid.UUID, poID uuid.UUID) ([]models.PurchaseOrderInvoice, error)
	MarkInvoiceAsPaid(ctx context.Context, tenantID uuid.UUID, invoiceID uuid.UUID) (*models.PurchaseOrderInvoice, error)

	// Reports
	GetPendingPayments(ctx context.Context, tenantID uuid.UUID) ([]models.PurchaseOrder, error)

	// Phase 2A: Price Comparison
	GetPriceComparison(ctx context.Context, tenantID uuid.UUID, productID uuid.UUID) ([]models.PriceComparison, error)
	GetBestSupplierForProduct(ctx context.Context, tenantID uuid.UUID, productID uuid.UUID) (*models.Supplier, error)

	// Phase 2A: FIFO Batch Tracking
	CreateProductBatch(ctx context.Context, tenantID uuid.UUID, batch *models.ProductBatch) (*models.ProductBatch, error)
	GetProductBatches(ctx context.Context, tenantID uuid.UUID, productID uuid.UUID) ([]models.ProductBatch, error)
	GetOldestBatchForSale(ctx context.Context, tenantID uuid.UUID, productID uuid.UUID) (*models.ProductBatch, error)

	// Phase 2A: RFQ (Request for Quote)
	CreateRFQ(ctx context.Context, tenantID uuid.UUID, rfq *models.RequestForQuote) (*models.RequestForQuote, error)
	GetRFQ(ctx context.Context, tenantID uuid.UUID, rfqID uuid.UUID) (*models.RequestForQuote, error)
	GetRFQItems(ctx context.Context, tenantID uuid.UUID, rfqID uuid.UUID) ([]models.RFQItem, error)
	AddRFQItem(ctx context.Context, tenantID uuid.UUID, rfqID uuid.UUID, item *models.RFQItem) (*models.RFQItem, error)
	AddRFQResponse(ctx context.Context, tenantID uuid.UUID, response *models.RFQResponse) (*models.RFQResponse, error)
	GetRFQResponses(ctx context.Context, tenantID uuid.UUID, rfqID uuid.UUID) ([]models.RFQResponse, error)
	SelectBestRFQResponse(ctx context.Context, tenantID uuid.UUID, rfqID, responseID uuid.UUID) error
	GetRFQComparison(ctx context.Context, tenantID uuid.UUID, rfqID uuid.UUID) (map[string]interface{}, error)
}

// PurchasingService interface defines business logic for purchasing
type PurchasingService interface {
	// Supplier operations
	CreateSupplier(ctx context.Context, tenantID uuid.UUID, dto *models.CreateSupplierRequestDto) (*models.Supplier, error)
	GetSupplier(ctx context.Context, tenantID uuid.UUID, supplierID uuid.UUID) (*models.Supplier, error)
	UpdateSupplier(ctx context.Context, tenantID uuid.UUID, supplierID uuid.UUID, dto *models.UpdateSupplierRequestDto) (*models.Supplier, error)
	ListSuppliers(ctx context.Context, tenantID uuid.UUID, page, pageSize int) ([]models.Supplier, int, error)

	// Purchase Order operations
	CreatePurchaseOrder(ctx context.Context, tenantID uuid.UUID, supplierID uuid.UUID, expectedDelivery *string, notes *string, createdBy uuid.UUID) (*models.PurchaseOrder, error)
	GetPurchaseOrder(ctx context.Context, tenantID uuid.UUID, poID uuid.UUID) (*models.PurchaseOrder, error)
	ListPurchaseOrders(ctx context.Context, tenantID uuid.UUID, status *string, page, pageSize int) ([]models.PurchaseOrder, int, error)
	ApprovePurchaseOrder(ctx context.Context, tenantID uuid.UUID, poID uuid.UUID) (*models.PurchaseOrder, error)
	AddItemToPurchaseOrder(ctx context.Context, tenantID uuid.UUID, poID, productID uuid.UUID, quantity int, unitCost float64) (*models.PurchaseOrderItem, error)
	ReceivePurchaseOrder(ctx context.Context, tenantID uuid.UUID, poID uuid.UUID, receivedBy *uuid.UUID, notes *string) (*models.PurchaseOrderReceipt, error)
	AddInvoice(ctx context.Context, tenantID uuid.UUID, poID uuid.UUID, dto *models.AddInvoiceRequestDto) (*models.PurchaseOrderInvoice, error)
	GetPendingPayments(ctx context.Context, tenantID uuid.UUID) ([]models.PurchaseOrder, error)

	// Phase 2A: Price Comparison
	GetPriceComparison(ctx context.Context, tenantID uuid.UUID, productID uuid.UUID) ([]models.PriceComparison, error)
	GetBestSupplierForProduct(ctx context.Context, tenantID uuid.UUID, productID uuid.UUID) (*models.Supplier, error)

	// Phase 2A: FIFO Batch Tracking
	CreateProductBatch(ctx context.Context, tenantID uuid.UUID, batch *models.ProductBatch) (*models.ProductBatch, error)
	GetProductBatches(ctx context.Context, tenantID uuid.UUID, productID uuid.UUID) ([]models.ProductBatch, error)
	GetOldestBatchForSale(ctx context.Context, tenantID uuid.UUID, productID uuid.UUID) (*models.ProductBatch, error)

	// Phase 2A: RFQ (Request for Quote)
	CreateRFQ(ctx context.Context, tenantID uuid.UUID, rfq *models.RequestForQuote) (*models.RequestForQuote, error)
	GetRFQ(ctx context.Context, tenantID uuid.UUID, rfqID uuid.UUID) (*models.RequestForQuote, error)
	AddRFQItem(ctx context.Context, tenantID uuid.UUID, rfqID uuid.UUID, item *models.RFQItem) (*models.RFQItem, error)
	AddRFQResponse(ctx context.Context, tenantID uuid.UUID, response *models.RFQResponse) (*models.RFQResponse, error)
	GetRFQResponses(ctx context.Context, tenantID uuid.UUID, rfqID uuid.UUID) ([]models.RFQResponse, error)
	SelectBestRFQResponse(ctx context.Context, tenantID uuid.UUID, rfqID, responseID uuid.UUID) error
	GetRFQComparison(ctx context.Context, tenantID uuid.UUID, rfqID uuid.UUID) (map[string]interface{}, error)
}
