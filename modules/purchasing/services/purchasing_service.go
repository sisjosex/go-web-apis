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

func (s *PurchasingService) CreateSupplier(ctx context.Context, dto *models.CreateSupplierRequestDto) (*models.Supplier, error) {
	return s.repo.CreateSupplier(ctx, dto)
}

func (s *PurchasingService) GetSupplier(ctx context.Context, supplierID uuid.UUID) (*models.Supplier, error) {
	return s.repo.GetSupplier(ctx, supplierID)
}

func (s *PurchasingService) UpdateSupplier(ctx context.Context, supplierID uuid.UUID, dto *models.UpdateSupplierRequestDto) (*models.Supplier, error) {
	return s.repo.UpdateSupplier(ctx, supplierID, dto)
}

func (s *PurchasingService) ListSuppliers(ctx context.Context, page, pageSize int) ([]models.Supplier, int, error) {
	return s.repo.ListSuppliers(ctx, page, pageSize)
}

// ========== PURCHASE ORDER OPERATIONS ==========

func (s *PurchasingService) CreatePurchaseOrder(ctx context.Context, supplierID uuid.UUID, expectedDelivery *string, notes *string, createdBy uuid.UUID) (*models.PurchaseOrder, error) {
	return s.repo.CreatePurchaseOrder(ctx, supplierID, expectedDelivery, notes, createdBy)
}

func (s *PurchasingService) GetPurchaseOrder(ctx context.Context, poID uuid.UUID) (*models.PurchaseOrder, error) {
	return s.repo.GetPurchaseOrder(ctx, poID)
}

func (s *PurchasingService) ListPurchaseOrders(ctx context.Context, status *string, page, pageSize int) ([]models.PurchaseOrder, int, error) {
	return s.repo.ListPurchaseOrders(ctx, status, page, pageSize)
}

func (s *PurchasingService) ApprovePurchaseOrder(ctx context.Context, poID uuid.UUID) (*models.PurchaseOrder, error) {
	return s.repo.ApprovePurchaseOrder(ctx, poID)
}

// ========== PO ITEM OPERATIONS ==========

func (s *PurchasingService) AddItemToPurchaseOrder(ctx context.Context, poID, productID uuid.UUID, quantity int, unitCost float64) (*models.PurchaseOrderItem, error) {
	return s.repo.AddPurchaseOrderItem(ctx, poID, productID, quantity, unitCost)
}

// ========== PO RECEIPT OPERATIONS ==========

func (s *PurchasingService) ReceivePurchaseOrder(ctx context.Context, poID uuid.UUID, receivedBy *uuid.UUID, notes *string) (*models.PurchaseOrderReceipt, error) {
	return s.repo.ReceivePurchaseOrder(ctx, poID, receivedBy, notes)
}

// ========== PO INVOICE OPERATIONS ==========

func (s *PurchasingService) AddInvoice(ctx context.Context, poID uuid.UUID, dto *models.AddInvoiceRequestDto) (*models.PurchaseOrderInvoice, error) {
	return s.repo.AddInvoice(ctx, poID, dto)
}

// ========== REPORTS ==========

func (s *PurchasingService) GetPendingPayments(ctx context.Context) ([]models.PurchaseOrder, error) {
	return s.repo.GetPendingPayments(ctx)
}

// ========== PHASE 2A: PRICE COMPARISON ==========

func (s *PurchasingService) GetPriceComparison(ctx context.Context, productID uuid.UUID) ([]models.PriceComparison, error) {
	return s.repo.GetPriceComparison(ctx, productID)
}

func (s *PurchasingService) GetBestSupplierForProduct(ctx context.Context, productID uuid.UUID) (*models.Supplier, error) {
	return s.repo.GetBestSupplierForProduct(ctx, productID)
}

// ========== PHASE 2A: FIFO BATCH TRACKING ==========

func (s *PurchasingService) CreateProductBatch(ctx context.Context, batch *models.ProductBatch) (*models.ProductBatch, error) {
	return s.repo.CreateProductBatch(ctx, batch)
}

func (s *PurchasingService) GetProductBatches(ctx context.Context, productID uuid.UUID) ([]models.ProductBatch, error) {
	return s.repo.GetProductBatches(ctx, productID)
}

func (s *PurchasingService) GetOldestBatchForSale(ctx context.Context, productID uuid.UUID) (*models.ProductBatch, error) {
	return s.repo.GetOldestBatchForSale(ctx, productID)
}

// ========== PHASE 2A: RFQ (REQUEST FOR QUOTE) ==========

func (s *PurchasingService) CreateRFQ(ctx context.Context, rfq *models.RequestForQuote) (*models.RequestForQuote, error) {
	return s.repo.CreateRFQ(ctx, rfq)
}

func (s *PurchasingService) GetRFQ(ctx context.Context, rfqID uuid.UUID) (*models.RequestForQuote, error) {
	return s.repo.GetRFQ(ctx, rfqID)
}

func (s *PurchasingService) AddRFQItem(ctx context.Context, rfqID uuid.UUID, item *models.RFQItem) (*models.RFQItem, error) {
	return s.repo.AddRFQItem(ctx, rfqID, item)
}

func (s *PurchasingService) AddRFQResponse(ctx context.Context, response *models.RFQResponse) (*models.RFQResponse, error) {
	return s.repo.AddRFQResponse(ctx, response)
}

func (s *PurchasingService) GetRFQResponses(ctx context.Context, rfqID uuid.UUID) ([]models.RFQResponse, error) {
	return s.repo.GetRFQResponses(ctx, rfqID)
}

func (s *PurchasingService) SelectBestRFQResponse(ctx context.Context, rfqID, responseID uuid.UUID) error {
	return s.repo.SelectBestRFQResponse(ctx, rfqID, responseID)
}

func (s *PurchasingService) GetRFQComparison(ctx context.Context, rfqID uuid.UUID) (map[string]interface{}, error) {
	return s.repo.GetRFQComparison(ctx, rfqID)
}
