package services

import (
	"context"

	"josex/web/modules/purchasing/interfaces"
	"josex/web/modules/purchasing/models"

	"github.com/google/uuid"
)

// PurchasingService handles purchasing business logic
type PurchasingService struct {
	repo interfaces.PurchasingRepository
}

// NewPurchasingService creates a new instance
func NewPurchasingService(repo interfaces.PurchasingRepository) *PurchasingService {
	return &PurchasingService{
		repo: repo,
	}
}

// ========== SUPPLIER OPERATIONS ==========

// CreateSupplier delegates to repo
func (s *PurchasingService) CreateSupplier(ctx context.Context, tenantID uuid.UUID, dto *models.CreateSupplierRequestDto) (*models.Supplier, error) {
	return s.repo.CreateSupplier(ctx, tenantID, dto)
}

// GetSupplier delegates to repo
func (s *PurchasingService) GetSupplier(ctx context.Context, supplierID, tenantID uuid.UUID) (*models.Supplier, error) {
	return s.repo.GetSupplier(ctx, supplierID, tenantID)
}

// UpdateSupplier delegates to repo
func (s *PurchasingService) UpdateSupplier(ctx context.Context, supplierID, tenantID uuid.UUID, dto *models.UpdateSupplierRequestDto) (*models.Supplier, error) {
	return s.repo.UpdateSupplier(ctx, supplierID, tenantID, dto)
}

// ListSuppliers delegates to repo
func (s *PurchasingService) ListSuppliers(ctx context.Context, tenantID uuid.UUID, page, pageSize int) ([]models.Supplier, int, error) {
	return s.repo.ListSuppliers(ctx, tenantID, page, pageSize)
}

// ========== PURCHASE ORDER OPERATIONS ==========

// CreatePurchaseOrder creates a new purchase order
func (s *PurchasingService) CreatePurchaseOrder(ctx context.Context, tenantID, supplierID uuid.UUID, expectedDelivery *string, notes *string, createdBy uuid.UUID) (*models.PurchaseOrder, error) {
	return s.repo.CreatePurchaseOrder(ctx, tenantID, supplierID, expectedDelivery, notes, createdBy)
}

// GetPurchaseOrder retrieves a PO with items
func (s *PurchasingService) GetPurchaseOrder(ctx context.Context, poID, tenantID uuid.UUID) (*models.PurchaseOrder, error) {
	return s.repo.GetPurchaseOrder(ctx, poID, tenantID)
}

// ListPurchaseOrders retrieves paginated POs
func (s *PurchasingService) ListPurchaseOrders(ctx context.Context, tenantID uuid.UUID, status *string, page, pageSize int) ([]models.PurchaseOrder, int, error) {
	return s.repo.ListPurchaseOrders(ctx, tenantID, status, page, pageSize)
}

// ApprovePurchaseOrder changes PO status to approved
func (s *PurchasingService) ApprovePurchaseOrder(ctx context.Context, poID, tenantID uuid.UUID) (*models.PurchaseOrder, error) {
	return s.repo.ApprovePurchaseOrder(ctx, poID, tenantID)
}

// ========== PO ITEM OPERATIONS ==========

// AddItemToPurchaseOrder adds an item to a PO
func (s *PurchasingService) AddItemToPurchaseOrder(ctx context.Context, poID, productID uuid.UUID, quantity int, unitCost float64) (*models.PurchaseOrderItem, error) {
	return s.repo.AddPurchaseOrderItem(ctx, poID, productID, quantity, unitCost)
}

// ========== PO RECEIPT OPERATIONS ==========

// ReceivePurchaseOrder records receipt of goods
func (s *PurchasingService) ReceivePurchaseOrder(ctx context.Context, poID uuid.UUID, receivedBy *uuid.UUID, notes *string) (*models.PurchaseOrderReceipt, error) {
	return s.repo.ReceivePurchaseOrder(ctx, poID, receivedBy, notes)
}

// ========== PO INVOICE OPERATIONS ==========

// AddInvoice adds a supplier invoice
func (s *PurchasingService) AddInvoice(ctx context.Context, poID uuid.UUID, dto *models.AddInvoiceRequestDto) (*models.PurchaseOrderInvoice, error) {
	return s.repo.AddInvoice(ctx, poID, dto)
}

// ========== REPORTS ==========

// GetPendingPayments gets all outstanding invoices
func (s *PurchasingService) GetPendingPayments(ctx context.Context, tenantID uuid.UUID) ([]models.PurchaseOrder, error) {
	return s.repo.GetPendingPayments(ctx, tenantID)
}

// ========== PHASE 2A: PRICE COMPARISON ==========

// GetPriceComparison gets all supplier prices for a product
func (s *PurchasingService) GetPriceComparison(ctx context.Context, tenantID, productID uuid.UUID) ([]models.PriceComparison, error) {
	return s.repo.GetPriceComparison(ctx, tenantID, productID)
}

// GetBestSupplierForProduct gets the lowest-cost supplier for a product
func (s *PurchasingService) GetBestSupplierForProduct(ctx context.Context, tenantID, productID uuid.UUID) (*models.Supplier, error) {
	return s.repo.GetBestSupplierForProduct(ctx, tenantID, productID)
}

// ========== PHASE 2A: FIFO BATCH TRACKING ==========

// CreateProductBatch records a new product batch
func (s *PurchasingService) CreateProductBatch(ctx context.Context, tenantID uuid.UUID, batch *models.ProductBatch) (*models.ProductBatch, error) {
	return s.repo.CreateProductBatch(ctx, tenantID, batch)
}

// GetProductBatches gets all active batches for a product
func (s *PurchasingService) GetProductBatches(ctx context.Context, tenantID, productID uuid.UUID) ([]models.ProductBatch, error) {
	return s.repo.GetProductBatches(ctx, tenantID, productID)
}

// GetOldestBatchForSale gets the oldest batch with inventory (FIFO)
func (s *PurchasingService) GetOldestBatchForSale(ctx context.Context, tenantID, productID uuid.UUID) (*models.ProductBatch, error) {
	return s.repo.GetOldestBatchForSale(ctx, tenantID, productID)
}

// ========== PHASE 2A: RFQ (REQUEST FOR QUOTE) ==========

// CreateRFQ creates a new Request for Quote
func (s *PurchasingService) CreateRFQ(ctx context.Context, tenantID uuid.UUID, rfq *models.RequestForQuote) (*models.RequestForQuote, error) {
	return s.repo.CreateRFQ(ctx, tenantID, rfq)
}

// GetRFQ retrieves an RFQ with all items and responses
func (s *PurchasingService) GetRFQ(ctx context.Context, rfqID, tenantID uuid.UUID) (*models.RequestForQuote, error) {
	return s.repo.GetRFQ(ctx, rfqID, tenantID)
}

// AddRFQItem adds a line item to an RFQ
func (s *PurchasingService) AddRFQItem(ctx context.Context, rfqID uuid.UUID, item *models.RFQItem) (*models.RFQItem, error) {
	return s.repo.AddRFQItem(ctx, rfqID, item)
}

// AddRFQResponse records a supplier's quote response
func (s *PurchasingService) AddRFQResponse(ctx context.Context, response *models.RFQResponse) (*models.RFQResponse, error) {
	return s.repo.AddRFQResponse(ctx, response)
}

// GetRFQResponses gets all responses for an RFQ
func (s *PurchasingService) GetRFQResponses(ctx context.Context, rfqID uuid.UUID) ([]models.RFQResponse, error) {
	return s.repo.GetRFQResponses(ctx, rfqID)
}

// SelectBestRFQResponse marks the chosen response and closes the RFQ
func (s *PurchasingService) SelectBestRFQResponse(ctx context.Context, rfqID, responseID, tenantID uuid.UUID) error {
	return s.repo.SelectBestRFQResponse(ctx, rfqID, responseID, tenantID)
}

// GetRFQComparison gets a side-by-side comparison of all RFQ responses
func (s *PurchasingService) GetRFQComparison(ctx context.Context, rfqID uuid.UUID) (map[string]interface{}, error) {
	return s.repo.GetRFQComparison(ctx, rfqID)
}
