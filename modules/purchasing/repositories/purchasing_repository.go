package repositories

import (
	"context"
	"fmt"

	"josex/web/modules/core/services"
	"josex/web/modules/purchasing/models"

	"github.com/google/uuid"
)

// PurchasingRepository handles all purchasing data operations
type PurchasingRepository struct {
	dbService services.DatabaseService
}

// NewPurchasingRepository creates a new instance
func NewPurchasingRepository(dbService services.DatabaseService) *PurchasingRepository {
	return &PurchasingRepository{
		dbService: dbService,
	}
}

// ========== SUPPLIER OPERATIONS ==========

// CreateSupplier inserts a new supplier
func (r *PurchasingRepository) CreateSupplier(ctx context.Context, dto *models.CreateSupplierRequestDto) (*models.Supplier, error) {
	supplier := &models.Supplier{}

	row := r.dbService.QueryRow(ctx,
		`SELECT * FROM purchasing.sp_create_supplier($1, $2, $3, $4, $5, $6)`,
		dto.Name, dto.ContactPerson, dto.Email, dto.Phone, dto.Address, dto.PaymentTerms,
	)

	err := row.Scan(&supplier.ID, &supplier.Name, &supplier.ContactPerson, &supplier.Email,
		&supplier.Phone, &supplier.Address, &supplier.PaymentTerms, &supplier.IsActive, &supplier.CreatedAt, &supplier.UpdatedAt)

	return supplier, err
}

// GetSupplier retrieves a single supplier
func (r *PurchasingRepository) GetSupplier(ctx context.Context, supplierID uuid.UUID) (*models.Supplier, error) {
	supplier := &models.Supplier{}

	row := r.dbService.QueryRow(ctx,
		`SELECT id, name, contact_person, email, phone, address, payment_terms, is_active, created_at, updated_at
		 FROM purchasing.suppliers
		 WHERE id = $1`,
		supplierID,
	)

	err := row.Scan(&supplier.ID, &supplier.Name, &supplier.ContactPerson, &supplier.Email,
		&supplier.Phone, &supplier.Address, &supplier.PaymentTerms, &supplier.IsActive, &supplier.CreatedAt, &supplier.UpdatedAt)

	return supplier, err
}

// UpdateSupplier updates supplier information
func (r *PurchasingRepository) UpdateSupplier(ctx context.Context, supplierID uuid.UUID, dto *models.UpdateSupplierRequestDto) (*models.Supplier, error) {
	supplier := &models.Supplier{}

	row := r.dbService.QueryRow(ctx,
		`SELECT * FROM purchasing.sp_update_supplier($1, $2, $3, $4, $5, $6, $7, $8)`,
		supplierID, dto.Name, dto.ContactPerson, dto.Email, dto.Phone, dto.Address, dto.PaymentTerms, dto.IsActive,
	)

	err := row.Scan(&supplier.ID, &supplier.Name, &supplier.ContactPerson, &supplier.Email,
		&supplier.Phone, &supplier.Address, &supplier.PaymentTerms, &supplier.IsActive, &supplier.CreatedAt, &supplier.UpdatedAt)

	return supplier, err
}

// ListSuppliers retrieves paginated suppliers
func (r *PurchasingRepository) ListSuppliers(ctx context.Context, page, pageSize int) ([]models.Supplier, int, error) {
	offset := (page - 1) * pageSize
	suppliers := []models.Supplier{}

	rows, err := r.dbService.Query(ctx,
		`SELECT * FROM purchasing.sp_list_suppliers($1, $2)`,
		pageSize, offset,
	)
	if err != nil {
		return suppliers, 0, err
	}
	defer rows.Close()

	for rows.Next() {
		supplier := models.Supplier{}
		err := rows.Scan(&supplier.ID, &supplier.Name, &supplier.ContactPerson, &supplier.Email,
			&supplier.Phone, &supplier.Address, &supplier.PaymentTerms, &supplier.IsActive, &supplier.CreatedAt, &supplier.UpdatedAt)
		if err != nil {
			return suppliers, 0, err
		}
		suppliers = append(suppliers, supplier)
	}

	// Get total count
	var totalCount int
	countRow := r.dbService.QueryRow(ctx,
		`SELECT total_count FROM purchasing.sp_get_suppliers_count()`,
	)
	err = countRow.Scan(&totalCount)

	return suppliers, totalCount, err
}

// DeleteSupplier soft deletes a supplier
func (r *PurchasingRepository) DeleteSupplier(ctx context.Context, supplierID uuid.UUID) error {
	var deleted bool
	err := r.dbService.QueryRow(ctx,
		`SELECT deleted FROM purchasing.sp_delete_supplier($1)`,
		supplierID,
	).Scan(&deleted)
	return err
}

// ========== PURCHASE ORDER OPERATIONS ==========

// CreatePurchaseOrder inserts a new purchase order
func (r *PurchasingRepository) CreatePurchaseOrder(ctx context.Context, supplierID uuid.UUID, expectedDelivery *string, notes *string, createdBy uuid.UUID) (*models.PurchaseOrder, error) {
	po := &models.PurchaseOrder{}

	row := r.dbService.QueryRow(ctx,
		`SELECT * FROM purchasing.sp_create_purchase_order($1, $2::DATE, $3, $4)`,
		supplierID, expectedDelivery, notes, createdBy,
	)

	err := row.Scan(&po.ID, &po.PONumber, &po.SupplierID, &po.Status, &po.TotalAmount, &po.ExpectedDeliveryDate, &po.Notes)

	return po, err
}

// GetPurchaseOrder retrieves a single PO with items
func (r *PurchasingRepository) GetPurchaseOrder(ctx context.Context, poID uuid.UUID) (*models.PurchaseOrder, error) {
	po := &models.PurchaseOrder{}

	row := r.dbService.QueryRow(ctx,
		`SELECT id, supplier_id, po_number, status, order_date, expected_delivery_date,
		        total_amount, paid_amount, notes, created_by, created_at, updated_at
		 FROM purchasing.purchase_orders
		 WHERE id = $1`,
		poID,
	)

	err := row.Scan(&po.ID, &po.SupplierID, &po.PONumber, &po.Status, &po.OrderDate,
		&po.ExpectedDeliveryDate, &po.TotalAmount, &po.PaidAmount, &po.Notes, &po.CreatedBy, &po.CreatedAt, &po.UpdatedAt)

	if err != nil {
		return po, err
	}

	// Get items
	items, err := r.GetPurchaseOrderItems(ctx, poID)
	if err == nil {
		po.Items = items
	}

	return po, nil
}

// ListPurchaseOrders retrieves paginated purchase orders
func (r *PurchasingRepository) ListPurchaseOrders(ctx context.Context, status *string, page, pageSize int) ([]models.PurchaseOrder, int, error) {
	offset := (page - 1) * pageSize
	pos := []models.PurchaseOrder{}

	rows, err := r.dbService.Query(ctx,
		`SELECT * FROM purchasing.sp_list_purchase_orders($1, $2, $3)`,
		status, pageSize, offset,
	)
	if err != nil {
		return pos, 0, err
	}
	defer rows.Close()

	for rows.Next() {
		po := models.PurchaseOrder{}
		err := rows.Scan(&po.ID, &po.SupplierID, &po.PONumber, &po.Status, &po.OrderDate,
			&po.ExpectedDeliveryDate, &po.TotalAmount, &po.PaidAmount, &po.Notes, &po.CreatedBy, &po.CreatedAt, &po.UpdatedAt)
		if err != nil {
			return pos, 0, err
		}
		pos = append(pos, po)
	}

	// Get total count
	countRow := r.dbService.QueryRow(ctx,
		`SELECT total_count FROM purchasing.sp_get_purchase_orders_count($1)`,
		status,
	)
	var totalCount int
	err = countRow.Scan(&totalCount)

	return pos, totalCount, err
}

// ApprovePurchaseOrder changes PO status to approved
func (r *PurchasingRepository) ApprovePurchaseOrder(ctx context.Context, poID uuid.UUID) (*models.PurchaseOrder, error) {
	po := &models.PurchaseOrder{}

	row := r.dbService.QueryRow(ctx,
		`SELECT * FROM purchasing.sp_approve_purchase_order($1)`,
		poID,
	)

	err := row.Scan(&po.ID, &po.Status, &po.TotalAmount, &po.Notes)

	return po, err
}

// ========== PO ITEM OPERATIONS ==========

// AddPurchaseOrderItem inserts a new item to a PO
func (r *PurchasingRepository) AddPurchaseOrderItem(ctx context.Context, poID, productID uuid.UUID, quantity int, unitCost float64) (*models.PurchaseOrderItem, error) {
	item := &models.PurchaseOrderItem{}
	var message string

	row := r.dbService.QueryRow(ctx,
		`SELECT * FROM purchasing.sp_add_purchase_order_item($1, $2, $3, $4)`,
		poID, productID, quantity, unitCost,
	)

	err := row.Scan(&item.ID, &item.PurchaseOrderID, &item.ProductID, &item.Quantity, &item.UnitCost, &item.LineTotal, &message)

	return item, err
}

// GetPurchaseOrderItems retrieves all items for a PO
func (r *PurchasingRepository) GetPurchaseOrderItems(ctx context.Context, poID uuid.UUID) ([]models.PurchaseOrderItem, error) {
	items := []models.PurchaseOrderItem{}

	rows, err := r.dbService.Query(ctx,
		`SELECT * FROM purchasing.sp_get_purchase_order_items($1)`,
		poID,
	)
	if err != nil {
		return items, err
	}
	defer rows.Close()

	for rows.Next() {
		item := models.PurchaseOrderItem{}
		err := rows.Scan(&item.ID, &item.PurchaseOrderID, &item.ProductID, &item.Quantity, &item.UnitCost,
			&item.LineTotal, &item.ReceivedQuantity, &item.Status, &item.CreatedAt, &item.UpdatedAt)
		if err != nil {
			return items, err
		}
		items = append(items, item)
	}

	return items, nil
}

// ========== PO RECEIPT OPERATIONS ==========

// ReceivePurchaseOrder records receipt of goods
func (r *PurchasingRepository) ReceivePurchaseOrder(ctx context.Context, poID uuid.UUID, receivedBy *uuid.UUID, notes *string) (*models.PurchaseOrderReceipt, error) {
	receipt := &models.PurchaseOrderReceipt{}

	row := r.dbService.QueryRow(ctx,
		`SELECT * FROM purchasing.sp_receive_purchase_order_items($1, CURRENT_TIMESTAMP, $2, $3)`,
		poID, receivedBy, notes,
	)

	err := row.Scan(&receipt.ID, &receipt.PurchaseOrderID, &receipt.ReceiptNumber, &receipt.ReceivedBy, &receipt.Notes, &receipt.CreatedAt)

	return receipt, err
}

// ========== PO INVOICE OPERATIONS ==========

// AddInvoice adds a supplier invoice to a PO
func (r *PurchasingRepository) AddInvoice(ctx context.Context, poID uuid.UUID, dto *models.AddInvoiceRequestDto) (*models.PurchaseOrderInvoice, error) {
	invoice := &models.PurchaseOrderInvoice{}

	row := r.dbService.QueryRow(ctx,
		`SELECT * FROM purchasing.sp_add_invoice($1, $2, $3, $4, $5, $6)`,
		poID, dto.InvoiceNumber, dto.InvoiceDate, dto.InvoiceAmount, dto.TaxAmount, dto.DueDate,
	)

	err := row.Scan(&invoice.ID, &invoice.PurchaseOrderID, &invoice.InvoiceNumber, &invoice.InvoiceDate,
		&invoice.InvoiceAmount, &invoice.TaxAmount, &invoice.DueDate, &invoice.Status, &invoice.CreatedAt, &invoice.UpdatedAt)

	return invoice, err
}

// GetInvoices retrieves all invoices for a PO
func (r *PurchasingRepository) GetInvoices(ctx context.Context, poID uuid.UUID) ([]models.PurchaseOrderInvoice, error) {
	invoices := []models.PurchaseOrderInvoice{}

	rows, err := r.dbService.Query(ctx,
		`SELECT * FROM purchasing.sp_get_invoices($1)`,
		poID,
	)
	if err != nil {
		return invoices, err
	}
	defer rows.Close()

	for rows.Next() {
		inv := models.PurchaseOrderInvoice{}
		err := rows.Scan(&inv.ID, &inv.PurchaseOrderID, &inv.InvoiceNumber, &inv.InvoiceDate,
			&inv.InvoiceAmount, &inv.TaxAmount, &inv.DueDate, &inv.Status, &inv.CreatedAt, &inv.UpdatedAt)
		if err != nil {
			return invoices, err
		}
		invoices = append(invoices, inv)
	}

	return invoices, nil
}

// MarkInvoiceAsPaid updates invoice status
func (r *PurchasingRepository) MarkInvoiceAsPaid(ctx context.Context, invoiceID uuid.UUID) (*models.PurchaseOrderInvoice, error) {
	invoice := &models.PurchaseOrderInvoice{}

	row := r.dbService.QueryRow(ctx,
		`SELECT * FROM purchasing.sp_mark_invoice_as_paid($1)`,
		invoiceID,
	)

	err := row.Scan(&invoice.ID, &invoice.PurchaseOrderID, &invoice.InvoiceNumber, &invoice.InvoiceDate,
		&invoice.InvoiceAmount, &invoice.TaxAmount, &invoice.DueDate, &invoice.Status, &invoice.CreatedAt, &invoice.UpdatedAt)

	return invoice, err
}

// ========== REPORTS ==========

// GetPendingPayments retrieves all outstanding invoices (accounts payable)
func (r *PurchasingRepository) GetPendingPayments(ctx context.Context) ([]models.PurchaseOrder, error) {
	pos := []models.PurchaseOrder{}

	rows, err := r.dbService.Query(ctx,
		`SELECT * FROM purchasing.sp_get_pending_payments()`,
	)
	if err != nil {
		return pos, err
	}
	defer rows.Close()

	for rows.Next() {
		po := models.PurchaseOrder{}
		err := rows.Scan(&po.ID, &po.SupplierID, &po.PONumber, &po.Status, &po.OrderDate,
			&po.ExpectedDeliveryDate, &po.TotalAmount, &po.PaidAmount, &po.Notes, &po.CreatedBy, &po.CreatedAt, &po.UpdatedAt)
		if err != nil {
			return pos, err
		}
		pos = append(pos, po)
	}

	return pos, nil
}

// ========== PHASE 2A: PRICE COMPARISON ==========

// GetPriceComparison retrieves all supplier prices for a specific product
func (r *PurchasingRepository) GetPriceComparison(ctx context.Context, productID uuid.UUID) ([]models.PriceComparison, error) {
	comparisons := []models.PriceComparison{}

	rows, err := r.dbService.Query(ctx,
		`SELECT
			sr.product_id,
			sr.supplier_id,
			s.name,
			sr.unit_cost,
			sr.payment_terms,
			sr.last_price_date,
			COALESCE(AVG(sr.unit_cost) OVER (PARTITION BY sr.product_id), sr.unit_cost) as average_cost
		 FROM purchasing.supplier_rates sr
		 JOIN purchasing.suppliers s ON sr.supplier_id = s.id
		 WHERE sr.product_id = $1
		 ORDER BY sr.unit_cost ASC`,
		productID,
	)
	if err != nil {
		return comparisons, err
	}
	defer rows.Close()

	for rows.Next() {
		pc := models.PriceComparison{}
		err := rows.Scan(&pc.ProductID, &pc.SupplierID, &pc.SupplierName, &pc.UnitCost, &pc.PaymentTerms, &pc.LastPurchaseAt, &pc.AverageCost)
		if err != nil {
			return comparisons, err
		}
		comparisons = append(comparisons, pc)
	}

	return comparisons, nil
}

// GetBestSupplierForProduct returns the supplier with lowest unit cost for a product
func (r *PurchasingRepository) GetBestSupplierForProduct(ctx context.Context, productID uuid.UUID) (*models.Supplier, error) {
	supplier := &models.Supplier{}

	row := r.dbService.QueryRow(ctx,
		`SELECT s.id, s.name, s.contact_person, s.email, s.phone, s.address, s.payment_terms, s.is_active, s.created_at, s.updated_at
		 FROM purchasing.suppliers s
		 JOIN purchasing.supplier_rates sr ON s.id = sr.supplier_id
		 WHERE sr.product_id = $1
		 ORDER BY sr.unit_cost ASC
		 LIMIT 1`,
		productID,
	)

	err := row.Scan(&supplier.ID, &supplier.Name, &supplier.ContactPerson, &supplier.Email,
		&supplier.Phone, &supplier.Address, &supplier.PaymentTerms, &supplier.IsActive, &supplier.CreatedAt, &supplier.UpdatedAt)

	return supplier, err
}

// ========== PHASE 2A: FIFO BATCH TRACKING ==========

// CreateProductBatch records a new product batch for inventory tracking
func (r *PurchasingRepository) CreateProductBatch(ctx context.Context, batch *models.ProductBatch) (*models.ProductBatch, error) {
	newBatch := &models.ProductBatch{}

	row := r.dbService.QueryRow(ctx,
		`SELECT * FROM purchasing.sp_create_product_batch($1, $2, $3, $4, $5, $6, $7)`,
		batch.ProductID, batch.BatchNumber, batch.Quantity, batch.UnitCost, batch.ReceiptDate, batch.ExpirationDate, batch.Status,
	)

	err := row.Scan(&newBatch.ID, &newBatch.ProductID, &newBatch.BatchNumber, &newBatch.Quantity,
		&newBatch.UnitCost, &newBatch.ReceiptDate, &newBatch.ExpirationDate, &newBatch.Status, &newBatch.CreatedAt, &newBatch.UpdatedAt)

	return newBatch, err
}

// GetProductBatches retrieves all batches for a product ordered by receipt date (oldest first)
func (r *PurchasingRepository) GetProductBatches(ctx context.Context, productID uuid.UUID) ([]models.ProductBatch, error) {
	batches := []models.ProductBatch{}

	rows, err := r.dbService.Query(ctx,
		`SELECT * FROM purchasing.sp_get_product_batches($1)`,
		productID,
	)
	if err != nil {
		return batches, err
	}
	defer rows.Close()

	for rows.Next() {
		batch := models.ProductBatch{}
		err := rows.Scan(&batch.ID, &batch.ProductID, &batch.BatchNumber, &batch.Quantity,
			&batch.UnitCost, &batch.ReceiptDate, &batch.ExpirationDate, &batch.Status, &batch.CreatedAt, &batch.UpdatedAt)
		if err != nil {
			return batches, err
		}
		batches = append(batches, batch)
	}

	return batches, nil
}

// GetOldestBatchForSale retrieves the oldest batch with remaining quantity (FIFO principle)
func (r *PurchasingRepository) GetOldestBatchForSale(ctx context.Context, productID uuid.UUID) (*models.ProductBatch, error) {
	batch := &models.ProductBatch{}

	row := r.dbService.QueryRow(ctx,
		`SELECT * FROM purchasing.sp_get_oldest_batch_for_sale($1)`,
		productID,
	)

	err := row.Scan(&batch.ID, &batch.ProductID, &batch.BatchNumber, &batch.Quantity,
		&batch.UnitCost, &batch.ReceiptDate, &batch.ExpirationDate, &batch.Status, &batch.CreatedAt, &batch.UpdatedAt)

	return batch, err
}

// ========== PHASE 2A: RFQ (REQUEST FOR QUOTE) ==========

// CreateRFQ creates a new Request for Quote
func (r *PurchasingRepository) CreateRFQ(ctx context.Context, rfq *models.RequestForQuote) (*models.RequestForQuote, error) {
	newRFQ := &models.RequestForQuote{}

	row := r.dbService.QueryRow(ctx,
		`SELECT * FROM purchasing.sp_create_rfq($1, $2)`,
		rfq.RFQNumber, rfq.Status,
	)

	err := row.Scan(&newRFQ.ID, &newRFQ.RFQNumber, &newRFQ.Status, &newRFQ.CreatedAt, &newRFQ.UpdatedAt)

	return newRFQ, err
}

// GetRFQ retrieves a Request for Quote with all items and responses
func (r *PurchasingRepository) GetRFQ(ctx context.Context, rfqID uuid.UUID) (*models.RequestForQuote, error) {
	rfq := &models.RequestForQuote{}

	row := r.dbService.QueryRow(ctx,
		`SELECT id, rfq_number, status, created_at, updated_at
		 FROM purchasing.request_for_quotes
		 WHERE id = $1`,
		rfqID,
	)

	err := row.Scan(&rfq.ID, &rfq.RFQNumber, &rfq.Status, &rfq.CreatedAt, &rfq.UpdatedAt)
	if err != nil {
		return rfq, err
	}

	// Get RFQ items
	items, err := r.GetRFQItems(ctx, rfqID)
	if err != nil {
		return rfq, err
	}
	rfq.Items = items

	// Get RFQ responses
	responses, err := r.GetRFQResponses(ctx, rfqID)
	if err != nil {
		return rfq, err
	}
	rfq.Responses = responses

	return rfq, nil
}

// GetRFQItems retrieves all line items for an RFQ
func (r *PurchasingRepository) GetRFQItems(ctx context.Context, rfqID uuid.UUID) ([]models.RFQItem, error) {
	items := []models.RFQItem{}

	rows, err := r.dbService.Query(ctx,
		`SELECT * FROM purchasing.sp_get_rfq_items($1)`,
		rfqID,
	)
	if err != nil {
		return items, err
	}
	defer rows.Close()

	for rows.Next() {
		item := models.RFQItem{}
		err := rows.Scan(&item.ID, &item.RFQID, &item.ProductID, &item.Quantity, &item.Description, &item.CreatedAt)
		if err != nil {
			return items, err
		}
		items = append(items, item)
	}

	return items, nil
}

// AddRFQItem adds a line item to an RFQ
func (r *PurchasingRepository) AddRFQItem(ctx context.Context, rfqID uuid.UUID, item *models.RFQItem) (*models.RFQItem, error) {
	newItem := &models.RFQItem{}

	row := r.dbService.QueryRow(ctx,
		`SELECT * FROM purchasing.sp_add_rfq_item($1, $2, $3, $4)`,
		rfqID, item.ProductID, item.Quantity, item.Description,
	)

	err := row.Scan(&newItem.ID, &newItem.RFQID, &newItem.ProductID, &newItem.Quantity, &newItem.Description, &newItem.CreatedAt)

	return newItem, err
}

// AddRFQResponse records a supplier's quote response
func (r *PurchasingRepository) AddRFQResponse(ctx context.Context, response *models.RFQResponse) (*models.RFQResponse, error) {
	newResponse := &models.RFQResponse{}

	row := r.dbService.QueryRow(ctx,
		`SELECT * FROM purchasing.sp_add_rfq_response($1, $2, $3, $4, $5, $6)`,
		response.RFQID, response.SupplierID, response.TotalPrice, response.DeliveryDays, response.PaymentTerms, response.Notes,
	)

	err := row.Scan(&newResponse.ID, &newResponse.RFQID, &newResponse.SupplierID, &newResponse.TotalPrice,
		&newResponse.DeliveryDays, &newResponse.PaymentTerms, &newResponse.Notes, &newResponse.CreatedAt, &newResponse.UpdatedAt)

	return newResponse, err
}

// GetRFQResponses retrieves all supplier responses for an RFQ
func (r *PurchasingRepository) GetRFQResponses(ctx context.Context, rfqID uuid.UUID) ([]models.RFQResponse, error) {
	responses := []models.RFQResponse{}

	rows, err := r.dbService.Query(ctx,
		`SELECT * FROM purchasing.sp_get_rfq_responses($1)`,
		rfqID,
	)
	if err != nil {
		return responses, err
	}
	defer rows.Close()

	for rows.Next() {
		resp := models.RFQResponse{}
		err := rows.Scan(&resp.ID, &resp.RFQID, &resp.SupplierID, &resp.TotalPrice, &resp.DeliveryDays,
			&resp.PaymentTerms, &resp.Notes, &resp.CreatedAt, &resp.UpdatedAt)
		if err != nil {
			return responses, err
		}
		responses = append(responses, resp)
	}

	return responses, nil
}

// SelectBestRFQResponse marks the chosen supplier response and closes the RFQ
func (r *PurchasingRepository) SelectBestRFQResponse(ctx context.Context, rfqID, responseID uuid.UUID) error {
	var updated bool
	err := r.dbService.QueryRow(ctx,
		`SELECT status_updated FROM purchasing.sp_select_best_rfq_response($1)`,
		rfqID,
	).Scan(&updated)
	return err
}

// GetRFQComparison aggregates all RFQ responses for side-by-side comparison
func (r *PurchasingRepository) GetRFQComparison(ctx context.Context, rfqID uuid.UUID) (map[string]interface{}, error) {
	comparison := map[string]interface{}{}

	rfq, err := r.GetRFQ(ctx, rfqID)
	if err != nil {
		return comparison, err
	}

	comparison["rfq_number"] = rfq.RFQNumber
	comparison["items_count"] = len(rfq.Items)
	comparison["responses"] = []map[string]interface{}{}

	for _, resp := range rfq.Responses {
		respMap := map[string]interface{}{
			"supplier_id":    resp.SupplierID.String(),
			"total_price":    resp.TotalPrice,
			"delivery_days":  resp.DeliveryDays,
			"payment_terms":  resp.PaymentTerms,
			"price_per_item": fmt.Sprintf("%.2f", resp.TotalPrice/float64(len(rfq.Items))),
		}
		comparison["responses"] = append(comparison["responses"].([]map[string]interface{}), respMap)
	}

	return comparison, nil
}
