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

func (s *PurchasingService) CreateSupplier(ctx context.Context, tenantID uuid.UUID, dto *models.CreateSupplierRequestDto) (*models.Supplier, error) {
	return s.repo.CreateSupplier(ctx, tenantID, dto)
}

func (s *PurchasingService) GetSupplier(ctx context.Context, tenantID uuid.UUID, supplierID uuid.UUID) (*models.Supplier, error) {
	return s.repo.GetSupplier(ctx, tenantID, supplierID)
}

func (s *PurchasingService) UpdateSupplier(ctx context.Context, tenantID uuid.UUID, supplierID uuid.UUID, dto *models.UpdateSupplierRequestDto) (*models.Supplier, error) {
	return s.repo.UpdateSupplier(ctx, tenantID, supplierID, dto)
}

func (s *PurchasingService) ListSuppliers(ctx context.Context, tenantID uuid.UUID, page, pageSize int) ([]models.Supplier, int, error) {
	return s.repo.ListSuppliers(ctx, tenantID, page, pageSize)
}

// ========== PURCHASE ORDER OPERATIONS ==========

func (s *PurchasingService) CreatePurchaseOrder(ctx context.Context, tenantID uuid.UUID, supplierID uuid.UUID, expectedDelivery *string, notes *string, createdBy uuid.UUID) (*models.PurchaseOrder, error) {
	return s.repo.CreatePurchaseOrder(ctx, tenantID, supplierID, expectedDelivery, notes, createdBy)
}

func (s *PurchasingService) GetPurchaseOrder(ctx context.Context, tenantID uuid.UUID, poID uuid.UUID) (*models.PurchaseOrder, error) {
	return s.repo.GetPurchaseOrder(ctx, tenantID, poID)
}

func (s *PurchasingService) ListPurchaseOrders(ctx context.Context, tenantID uuid.UUID, status *string, page, pageSize int) ([]models.PurchaseOrder, int, error) {
	return s.repo.ListPurchaseOrders(ctx, tenantID, status, page, pageSize)
}

func (s *PurchasingService) ApprovePurchaseOrder(ctx context.Context, tenantID uuid.UUID, poID uuid.UUID) (*models.PurchaseOrder, error) {
	return s.repo.ApprovePurchaseOrder(ctx, tenantID, poID)
}

// ========== PO ITEM OPERATIONS ==========

func (s *PurchasingService) AddItemToPurchaseOrder(ctx context.Context, tenantID uuid.UUID, poID, productID uuid.UUID, quantity int, unitCost float64) (*models.PurchaseOrderItem, error) {
	return s.repo.AddPurchaseOrderItem(ctx, tenantID, poID, productID, quantity, unitCost)
}

// ========== PO RECEIPT OPERATIONS ==========

func (s *PurchasingService) ReceivePurchaseOrder(ctx context.Context, tenantID uuid.UUID, poID uuid.UUID, receivedBy *uuid.UUID, notes *string) (*models.PurchaseOrderReceipt, error) {
	return s.repo.ReceivePurchaseOrder(ctx, tenantID, poID, receivedBy, notes)
}

// ========== PO INVOICE OPERATIONS ==========

func (s *PurchasingService) AddInvoice(ctx context.Context, tenantID uuid.UUID, poID uuid.UUID, dto *models.AddInvoiceRequestDto) (*models.PurchaseOrderInvoice, error) {
	return s.repo.AddInvoice(ctx, tenantID, poID, dto)
}

// ========== REPORTS ==========

func (s *PurchasingService) GetPendingPayments(ctx context.Context, tenantID uuid.UUID) ([]models.PurchaseOrder, error) {
	return s.repo.GetPendingPayments(ctx, tenantID)
}

// ========== PHASE 2A: PRICE COMPARISON ==========

func (s *PurchasingService) GetPriceComparison(ctx context.Context, tenantID uuid.UUID, productID uuid.UUID) ([]models.PriceComparison, error) {
	return s.repo.GetPriceComparison(ctx, tenantID, productID)
}

func (s *PurchasingService) GetBestSupplierForProduct(ctx context.Context, tenantID uuid.UUID, productID uuid.UUID) (*models.Supplier, error) {
	return s.repo.GetBestSupplierForProduct(ctx, tenantID, productID)
}

// ========== PHASE 2A: FIFO BATCH TRACKING ==========

func (s *PurchasingService) CreateProductBatch(ctx context.Context, tenantID uuid.UUID, batch *models.ProductBatch) (*models.ProductBatch, error) {
	return s.repo.CreateProductBatch(ctx, tenantID, batch)
}

func (s *PurchasingService) GetProductBatches(ctx context.Context, tenantID uuid.UUID, productID uuid.UUID) ([]models.ProductBatch, error) {
	return s.repo.GetProductBatches(ctx, tenantID, productID)
}

func (s *PurchasingService) GetOldestBatchForSale(ctx context.Context, tenantID uuid.UUID, productID uuid.UUID) (*models.ProductBatch, error) {
	return s.repo.GetOldestBatchForSale(ctx, tenantID, productID)
}

// ========== PHASE 2A: RFQ (REQUEST FOR QUOTE) ==========

func (s *PurchasingService) CreateRFQ(ctx context.Context, tenantID uuid.UUID, rfq *models.RequestForQuote) (*models.RequestForQuote, error) {
	return s.repo.CreateRFQ(ctx, tenantID, rfq)
}

func (s *PurchasingService) GetRFQ(ctx context.Context, tenantID uuid.UUID, rfqID uuid.UUID) (*models.RequestForQuote, error) {
	return s.repo.GetRFQ(ctx, tenantID, rfqID)
}

func (s *PurchasingService) AddRFQItem(ctx context.Context, tenantID uuid.UUID, rfqID uuid.UUID, item *models.RFQItem) (*models.RFQItem, error) {
	return s.repo.AddRFQItem(ctx, tenantID, rfqID, item)
}

func (s *PurchasingService) AddRFQResponse(ctx context.Context, tenantID uuid.UUID, response *models.RFQResponse) (*models.RFQResponse, error) {
	return s.repo.AddRFQResponse(ctx, tenantID, response)
}

func (s *PurchasingService) GetRFQResponses(ctx context.Context, tenantID uuid.UUID, rfqID uuid.UUID) ([]models.RFQResponse, error) {
	return s.repo.GetRFQResponses(ctx, tenantID, rfqID)
}

func (s *PurchasingService) SelectBestRFQResponse(ctx context.Context, tenantID uuid.UUID, rfqID, responseID uuid.UUID) error {
	return s.repo.SelectBestRFQResponse(ctx, tenantID, rfqID, responseID)
}

func (s *PurchasingService) GetRFQComparison(ctx context.Context, tenantID uuid.UUID, rfqID uuid.UUID) (map[string]interface{}, error) {
	return s.repo.GetRFQComparison(ctx, tenantID, rfqID)
}
