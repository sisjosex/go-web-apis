package repositories

import (
	"context"
	"errors"

	"josex/web/modules/core/services"
	"josex/web/modules/purchasing/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
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
func (r *PurchasingRepository) CreateSupplier(ctx context.Context, tenantID uuid.UUID, dto *models.CreateSupplierRequestDto) (*models.Supplier, error) {
	supplier := &models.Supplier{}
	if err := r.dbService.QueryRow(ctx,
		`SELECT * FROM purchasing.sp_create_supplier($1, $2, $3, $4, $5, $6, $7)`,
		tenantID, dto.Name, dto.ContactPerson, dto.Email, dto.Phone, dto.Address, dto.PaymentTerms,
	).Scan(
		&supplier.ID, &supplier.Name, &supplier.ContactPerson, &supplier.Email,
		&supplier.Phone, &supplier.Address, &supplier.PaymentTerms, &supplier.IsActive,
		&supplier.CreatedAt, &supplier.UpdatedAt,
	); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return nil, pgErr
		}
		return nil, err
	}
	return supplier, nil
}

// GetSupplier retrieves a single supplier by ID
func (r *PurchasingRepository) GetSupplier(ctx context.Context, tenantID uuid.UUID, supplierID uuid.UUID) (*models.Supplier, error) {
	supplier := &models.Supplier{}
	if err := r.dbService.QueryRow(ctx,
		`SELECT * FROM purchasing.sp_get_supplier($1, $2)`,
		supplierID, tenantID,
	).Scan(
		&supplier.ID, &supplier.Name, &supplier.ContactPerson, &supplier.Email,
		&supplier.Phone, &supplier.Address, &supplier.PaymentTerms, &supplier.IsActive,
		&supplier.CreatedAt, &supplier.UpdatedAt,
	); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return nil, pgErr
		}
		return nil, err
	}
	return supplier, nil
}

// UpdateSupplier updates supplier information
func (r *PurchasingRepository) UpdateSupplier(ctx context.Context, tenantID uuid.UUID, supplierID uuid.UUID, dto *models.UpdateSupplierRequestDto) (*models.Supplier, error) {
	supplier := &models.Supplier{}
	if err := r.dbService.QueryRow(ctx,
		`SELECT * FROM purchasing.sp_update_supplier($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		tenantID, supplierID, dto.Name, dto.ContactPerson, dto.Email, dto.Phone, dto.Address, dto.PaymentTerms, dto.IsActive,
	).Scan(
		&supplier.ID, &supplier.Name, &supplier.ContactPerson, &supplier.Email,
		&supplier.Phone, &supplier.Address, &supplier.PaymentTerms, &supplier.IsActive,
		&supplier.CreatedAt, &supplier.UpdatedAt,
	); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return nil, pgErr
		}
		return nil, err
	}
	return supplier, nil
}

// ListSuppliers retrieves paginated suppliers
func (r *PurchasingRepository) ListSuppliers(ctx context.Context, tenantID uuid.UUID, page, pageSize int) ([]models.Supplier, int, error) {
	offset := (page - 1) * pageSize
	suppliers := []models.Supplier{}

	rows, err := r.dbService.Query(ctx,
		`SELECT * FROM purchasing.sp_list_suppliers($1, $2, $3)`,
		tenantID, pageSize, offset,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return nil, 0, pgErr
		}
		return nil, 0, err
	}
	defer rows.Close()

	for rows.Next() {
		supplier := models.Supplier{}
		if err := rows.Scan(
			&supplier.ID, &supplier.Name, &supplier.ContactPerson, &supplier.Email,
			&supplier.Phone, &supplier.Address, &supplier.PaymentTerms, &supplier.IsActive,
			&supplier.CreatedAt, &supplier.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		suppliers = append(suppliers, supplier)
	}
	if err = rows.Err(); err != nil {
		return nil, 0, err
	}

	var totalCount int
	if err := r.dbService.QueryRow(ctx,
		`SELECT total_count FROM purchasing.sp_get_suppliers_count($1)`,
		tenantID,
	).Scan(&totalCount); err != nil {
		return nil, 0, err
	}

	return suppliers, totalCount, nil
}

// DeleteSupplier soft deletes a supplier
func (r *PurchasingRepository) DeleteSupplier(ctx context.Context, tenantID uuid.UUID, supplierID uuid.UUID) error {
	var deleted bool
	if err := r.dbService.QueryRow(ctx,
		`SELECT deleted FROM purchasing.sp_delete_supplier($1, $2)`,
		tenantID, supplierID,
	).Scan(&deleted); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return pgErr
		}
		return err
	}
	return nil
}

// ========== PURCHASE ORDER OPERATIONS ==========

// CreatePurchaseOrder inserts a new purchase order
func (r *PurchasingRepository) CreatePurchaseOrder(ctx context.Context, tenantID uuid.UUID, supplierID uuid.UUID, expectedDelivery *string, notes *string, createdBy uuid.UUID) (*models.PurchaseOrder, error) {
	po := &models.PurchaseOrder{}
	if err := r.dbService.QueryRow(ctx,
		`SELECT * FROM purchasing.sp_create_purchase_order($1, $2, $3::DATE, $4, $5)`,
		tenantID, supplierID, expectedDelivery, notes, createdBy,
	).Scan(
		&po.ID, &po.PONumber, &po.SupplierID, &po.Status,
		&po.TotalAmount, &po.ExpectedDeliveryDate, &po.Notes,
	); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return nil, pgErr
		}
		return nil, err
	}
	return po, nil
}

// GetPurchaseOrder retrieves a single PO with its items
func (r *PurchasingRepository) GetPurchaseOrder(ctx context.Context, tenantID uuid.UUID, poID uuid.UUID) (*models.PurchaseOrder, error) {
	po := &models.PurchaseOrder{}
	if err := r.dbService.QueryRow(ctx,
		`SELECT * FROM purchasing.sp_get_purchase_order($1, $2)`,
		poID, tenantID,
	).Scan(
		&po.ID, &po.SupplierID, &po.PONumber, &po.Status, &po.OrderDate,
		&po.ExpectedDeliveryDate, &po.TotalAmount, &po.PaidAmount, &po.Notes,
		&po.CreatedBy, &po.CreatedAt, &po.UpdatedAt,
	); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return nil, pgErr
		}
		return nil, err
	}

	items, err := r.GetPurchaseOrderItems(ctx, tenantID, poID)
	if err != nil {
		return nil, err
	}
	po.Items = items

	return po, nil
}

// ListPurchaseOrders retrieves paginated purchase orders
func (r *PurchasingRepository) ListPurchaseOrders(ctx context.Context, tenantID uuid.UUID, status *string, page, pageSize int) ([]models.PurchaseOrder, int, error) {
	offset := (page - 1) * pageSize
	pos := []models.PurchaseOrder{}

	rows, err := r.dbService.Query(ctx,
		`SELECT * FROM purchasing.sp_list_purchase_orders($1, $2, $3, $4)`,
		tenantID, status, pageSize, offset,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return nil, 0, pgErr
		}
		return nil, 0, err
	}
	defer rows.Close()

	for rows.Next() {
		po := models.PurchaseOrder{}
		if err := rows.Scan(
			&po.ID, &po.SupplierID, &po.PONumber, &po.Status, &po.OrderDate,
			&po.ExpectedDeliveryDate, &po.TotalAmount, &po.PaidAmount, &po.Notes,
			&po.CreatedBy, &po.CreatedAt, &po.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		pos = append(pos, po)
	}
	if err = rows.Err(); err != nil {
		return nil, 0, err
	}

	var totalCount int
	if err := r.dbService.QueryRow(ctx,
		`SELECT total_count FROM purchasing.sp_get_purchase_orders_count($1, $2)`,
		tenantID, status,
	).Scan(&totalCount); err != nil {
		return nil, 0, err
	}

	return pos, totalCount, nil
}

// ApprovePurchaseOrder changes PO status to approved
func (r *PurchasingRepository) ApprovePurchaseOrder(ctx context.Context, tenantID uuid.UUID, poID uuid.UUID) (*models.PurchaseOrder, error) {
	po := &models.PurchaseOrder{}
	if err := r.dbService.QueryRow(ctx,
		`SELECT * FROM purchasing.sp_approve_purchase_order($1, $2)`,
		tenantID, poID,
	).Scan(&po.ID, &po.Status, &po.TotalAmount, &po.Notes); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return nil, pgErr
		}
		return nil, err
	}
	return po, nil
}

// ========== PO ITEM OPERATIONS ==========

// AddPurchaseOrderItem inserts a new item to a PO
func (r *PurchasingRepository) AddPurchaseOrderItem(ctx context.Context, tenantID uuid.UUID, poID, productID uuid.UUID, quantity int, unitCost float64) (*models.PurchaseOrderItem, error) {
	item := &models.PurchaseOrderItem{}
	var message string
	if err := r.dbService.QueryRow(ctx,
		`SELECT * FROM purchasing.sp_add_purchase_order_item($1, $2, $3, $4, $5)`,
		tenantID, poID, productID, quantity, unitCost,
	).Scan(
		&item.ID, &item.PurchaseOrderID, &item.ProductID, &item.Quantity,
		&item.UnitCost, &item.LineTotal, &message,
	); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return nil, pgErr
		}
		return nil, err
	}
	return item, nil
}

// GetPurchaseOrderItems retrieves all items for a PO
func (r *PurchasingRepository) GetPurchaseOrderItems(ctx context.Context, tenantID uuid.UUID, poID uuid.UUID) ([]models.PurchaseOrderItem, error) {
	items := []models.PurchaseOrderItem{}

	rows, err := r.dbService.Query(ctx,
		`SELECT * FROM purchasing.sp_get_purchase_order_items($1, $2)`,
		tenantID, poID,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return nil, pgErr
		}
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		item := models.PurchaseOrderItem{}
		if err := rows.Scan(
			&item.ID, &item.PurchaseOrderID, &item.ProductID, &item.Quantity, &item.UnitCost,
			&item.LineTotal, &item.ReceivedQuantity, &item.Status, &item.CreatedAt, &item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}

// ========== PO RECEIPT OPERATIONS ==========

// ReceivePurchaseOrder records receipt of goods
func (r *PurchasingRepository) ReceivePurchaseOrder(ctx context.Context, tenantID uuid.UUID, poID uuid.UUID, receivedBy *uuid.UUID, notes *string) (*models.PurchaseOrderReceipt, error) {
	receipt := &models.PurchaseOrderReceipt{}
	if err := r.dbService.QueryRow(ctx,
		`SELECT * FROM purchasing.sp_receive_purchase_order_items($1, $2, CURRENT_TIMESTAMP, $3, $4)`,
		tenantID, poID, receivedBy, notes,
	).Scan(
		&receipt.ID, &receipt.PurchaseOrderID, &receipt.ReceiptNumber,
		&receipt.ReceivedBy, &receipt.Notes, &receipt.CreatedAt,
	); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return nil, pgErr
		}
		return nil, err
	}
	return receipt, nil
}

// ========== PO INVOICE OPERATIONS ==========

// AddInvoice adds a supplier invoice to a PO
func (r *PurchasingRepository) AddInvoice(ctx context.Context, tenantID uuid.UUID, poID uuid.UUID, dto *models.AddInvoiceRequestDto) (*models.PurchaseOrderInvoice, error) {
	invoice := &models.PurchaseOrderInvoice{}
	if err := r.dbService.QueryRow(ctx,
		`SELECT * FROM purchasing.sp_add_invoice($1, $2, $3, $4, $5, $6, $7)`,
		tenantID, poID, dto.InvoiceNumber, dto.InvoiceDate, dto.InvoiceAmount, dto.TaxAmount, dto.DueDate,
	).Scan(
		&invoice.ID, &invoice.PurchaseOrderID, &invoice.InvoiceNumber, &invoice.InvoiceDate,
		&invoice.InvoiceAmount, &invoice.TaxAmount, &invoice.DueDate, &invoice.Status,
		&invoice.CreatedAt, &invoice.UpdatedAt,
	); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return nil, pgErr
		}
		return nil, err
	}
	return invoice, nil
}

// GetInvoices retrieves all invoices for a PO
func (r *PurchasingRepository) GetInvoices(ctx context.Context, tenantID uuid.UUID, poID uuid.UUID) ([]models.PurchaseOrderInvoice, error) {
	invoices := []models.PurchaseOrderInvoice{}

	rows, err := r.dbService.Query(ctx,
		`SELECT * FROM purchasing.sp_get_invoices($1, $2)`,
		tenantID, poID,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return nil, pgErr
		}
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		inv := models.PurchaseOrderInvoice{}
		if err := rows.Scan(
			&inv.ID, &inv.PurchaseOrderID, &inv.InvoiceNumber, &inv.InvoiceDate,
			&inv.InvoiceAmount, &inv.TaxAmount, &inv.DueDate, &inv.Status,
			&inv.CreatedAt, &inv.UpdatedAt,
		); err != nil {
			return nil, err
		}
		invoices = append(invoices, inv)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}

	return invoices, nil
}

// MarkInvoiceAsPaid updates invoice status
func (r *PurchasingRepository) MarkInvoiceAsPaid(ctx context.Context, tenantID uuid.UUID, invoiceID uuid.UUID) (*models.PurchaseOrderInvoice, error) {
	invoice := &models.PurchaseOrderInvoice{}
	if err := r.dbService.QueryRow(ctx,
		`SELECT * FROM purchasing.sp_mark_invoice_as_paid($1, $2)`,
		tenantID, invoiceID,
	).Scan(
		&invoice.ID, &invoice.PurchaseOrderID, &invoice.InvoiceNumber, &invoice.InvoiceDate,
		&invoice.InvoiceAmount, &invoice.TaxAmount, &invoice.DueDate, &invoice.Status,
		&invoice.CreatedAt, &invoice.UpdatedAt,
	); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return nil, pgErr
		}
		return nil, err
	}
	return invoice, nil
}

// ========== REPORTS ==========

// GetPendingPayments retrieves all outstanding invoices (accounts payable)
func (r *PurchasingRepository) GetPendingPayments(ctx context.Context, tenantID uuid.UUID) ([]models.PurchaseOrder, error) {
	pos := []models.PurchaseOrder{}

	rows, err := r.dbService.Query(ctx,
		`SELECT * FROM purchasing.sp_get_pending_payments($1)`,
		tenantID,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return nil, pgErr
		}
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		po := models.PurchaseOrder{}
		if err := rows.Scan(
			&po.ID, &po.SupplierID, &po.PONumber, &po.Status, &po.OrderDate,
			&po.ExpectedDeliveryDate, &po.TotalAmount, &po.PaidAmount, &po.Notes,
			&po.CreatedBy, &po.CreatedAt, &po.UpdatedAt,
		); err != nil {
			return nil, err
		}
		pos = append(pos, po)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}

	return pos, nil
}

// ========== PHASE 2A: PRICE COMPARISON ==========

// GetPriceComparison retrieves all supplier prices for a specific product
func (r *PurchasingRepository) GetPriceComparison(ctx context.Context, tenantID uuid.UUID, productID uuid.UUID) ([]models.PriceComparison, error) {
	comparisons := []models.PriceComparison{}

	rows, err := r.dbService.Query(ctx,
		`SELECT * FROM purchasing.sp_get_price_comparison($1, $2)`,
		productID, tenantID,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return nil, pgErr
		}
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		pc := models.PriceComparison{}
		if err := rows.Scan(
			&pc.ProductID, &pc.SupplierID, &pc.SupplierName,
			&pc.UnitCost, &pc.PaymentTerms, &pc.LastPurchaseAt, &pc.AverageCost,
		); err != nil {
			return nil, err
		}
		comparisons = append(comparisons, pc)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}

	return comparisons, nil
}

// GetBestSupplierForProduct returns the supplier with lowest unit cost for a product
func (r *PurchasingRepository) GetBestSupplierForProduct(ctx context.Context, tenantID uuid.UUID, productID uuid.UUID) (*models.Supplier, error) {
	supplier := &models.Supplier{}
	if err := r.dbService.QueryRow(ctx,
		`SELECT * FROM purchasing.sp_get_best_supplier_for_product($1, $2)`,
		productID, tenantID,
	).Scan(
		&supplier.ID, &supplier.Name, &supplier.ContactPerson, &supplier.Email,
		&supplier.Phone, &supplier.Address, &supplier.PaymentTerms, &supplier.IsActive,
		&supplier.CreatedAt, &supplier.UpdatedAt,
	); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return nil, pgErr
		}
		return nil, err
	}
	return supplier, nil
}

// ========== PHASE 2A: FIFO BATCH TRACKING ==========

// CreateProductBatch records a new product batch for inventory tracking
func (r *PurchasingRepository) CreateProductBatch(ctx context.Context, tenantID uuid.UUID, batch *models.ProductBatch) (*models.ProductBatch, error) {
	newBatch := &models.ProductBatch{}
	if err := r.dbService.QueryRow(ctx,
		`SELECT * FROM purchasing.sp_create_product_batch($1, $2, $3, $4, $5, $6, $7, $8)`,
		tenantID, batch.ProductID, batch.BatchNumber, batch.Quantity, batch.UnitCost,
		batch.ReceiptDate, batch.ExpirationDate, batch.Status,
	).Scan(
		&newBatch.ID, &newBatch.ProductID, &newBatch.BatchNumber, &newBatch.Quantity,
		&newBatch.UnitCost, &newBatch.ReceiptDate, &newBatch.ExpirationDate,
		&newBatch.Status, &newBatch.CreatedAt, &newBatch.UpdatedAt,
	); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return nil, pgErr
		}
		return nil, err
	}
	return newBatch, nil
}

// GetProductBatches retrieves all batches for a product ordered by receipt date (oldest first)
func (r *PurchasingRepository) GetProductBatches(ctx context.Context, tenantID uuid.UUID, productID uuid.UUID) ([]models.ProductBatch, error) {
	batches := []models.ProductBatch{}

	rows, err := r.dbService.Query(ctx,
		`SELECT * FROM purchasing.sp_get_product_batches($1, $2)`,
		tenantID, productID,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return nil, pgErr
		}
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		batch := models.ProductBatch{}
		if err := rows.Scan(
			&batch.ID, &batch.ProductID, &batch.BatchNumber, &batch.Quantity,
			&batch.UnitCost, &batch.ReceiptDate, &batch.ExpirationDate,
			&batch.Status, &batch.CreatedAt, &batch.UpdatedAt,
		); err != nil {
			return nil, err
		}
		batches = append(batches, batch)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}

	return batches, nil
}

// GetOldestBatchForSale retrieves the oldest batch with remaining quantity (FIFO principle)
func (r *PurchasingRepository) GetOldestBatchForSale(ctx context.Context, tenantID uuid.UUID, productID uuid.UUID) (*models.ProductBatch, error) {
	batch := &models.ProductBatch{}
	if err := r.dbService.QueryRow(ctx,
		`SELECT * FROM purchasing.sp_get_oldest_batch_for_sale($1, $2)`,
		tenantID, productID,
	).Scan(
		&batch.ID, &batch.ProductID, &batch.BatchNumber, &batch.Quantity,
		&batch.UnitCost, &batch.ReceiptDate, &batch.ExpirationDate,
		&batch.Status, &batch.CreatedAt, &batch.UpdatedAt,
	); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return nil, pgErr
		}
		return nil, err
	}
	return batch, nil
}

// ========== PHASE 2A: RFQ (REQUEST FOR QUOTE) ==========

// CreateRFQ inserts a new request for quote
func (r *PurchasingRepository) CreateRFQ(ctx context.Context, tenantID uuid.UUID, rfq *models.RequestForQuote) (*models.RequestForQuote, error) {
	result := &models.RequestForQuote{}
	if err := r.dbService.QueryRow(ctx,
		`SELECT * FROM purchasing.sp_create_rfq($1, $2, $3)`,
		tenantID, rfq.RFQNumber, rfq.Status,
	).Scan(
		&result.ID, &result.RFQNumber, &result.Status,
		&result.CreatedAt, &result.UpdatedAt,
	); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return nil, pgErr
		}
		return nil, err
	}
	return result, nil
}

// GetRFQ retrieves a single RFQ by ID
func (r *PurchasingRepository) GetRFQ(ctx context.Context, tenantID uuid.UUID, rfqID uuid.UUID) (*models.RequestForQuote, error) {
	rfq := &models.RequestForQuote{}
	if err := r.dbService.QueryRow(ctx,
		`SELECT * FROM purchasing.sp_get_rfq($1, $2)`,
		tenantID, rfqID,
	).Scan(
		&rfq.ID, &rfq.RFQNumber, &rfq.Status,
		&rfq.CreatedAt, &rfq.UpdatedAt,
	); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return nil, pgErr
		}
		return nil, err
	}

	items, err := r.GetRFQItems(ctx, tenantID, rfqID)
	if err != nil {
		return nil, err
	}
	rfq.Items = items

	responses, err := r.GetRFQResponses(ctx, tenantID, rfqID)
	if err != nil {
		return nil, err
	}
	rfq.Responses = responses

	return rfq, nil
}

// GetRFQItems retrieves all items for an RFQ
func (r *PurchasingRepository) GetRFQItems(ctx context.Context, tenantID uuid.UUID, rfqID uuid.UUID) ([]models.RFQItem, error) {
	items := []models.RFQItem{}

	rows, err := r.dbService.Query(ctx,
		`SELECT * FROM purchasing.sp_get_rfq_items($1, $2)`,
		tenantID, rfqID,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return nil, pgErr
		}
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		item := models.RFQItem{}
		var quantity float64
		if err := rows.Scan(
			&item.ID, &item.RFQID, &item.ProductID,
			&quantity, &item.Description, &item.CreatedAt,
		); err != nil {
			return nil, err
		}
		item.Quantity = int(quantity)
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}

// AddRFQItem adds an item to an RFQ
func (r *PurchasingRepository) AddRFQItem(ctx context.Context, tenantID uuid.UUID, rfqID uuid.UUID, item *models.RFQItem) (*models.RFQItem, error) {
	result := &models.RFQItem{}
	var quantity float64
	if err := r.dbService.QueryRow(ctx,
		`SELECT * FROM purchasing.sp_add_rfq_item($1, $2, $3, $4, $5)`,
		tenantID, rfqID, item.ProductID, item.Quantity, item.Description,
	).Scan(
		&result.ID, &result.RFQID, &result.ProductID,
		&quantity, &result.Description, &result.CreatedAt,
	); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return nil, pgErr
		}
		return nil, err
	}
	result.Quantity = int(quantity)
	return result, nil
}

// AddRFQResponse adds a supplier response to an RFQ
func (r *PurchasingRepository) AddRFQResponse(ctx context.Context, tenantID uuid.UUID, response *models.RFQResponse) (*models.RFQResponse, error) {
	result := &models.RFQResponse{}
	var paymentTerms string
	if err := r.dbService.QueryRow(ctx,
		`SELECT * FROM purchasing.sp_add_rfq_response($1, $2, $3, $4, $5, $6, $7)`,
		tenantID, response.RFQID, response.SupplierID, response.TotalPrice,
		response.DeliveryDays, response.PaymentTerms, response.Notes,
	).Scan(
		&result.ID, &result.RFQID, &result.SupplierID, &result.TotalPrice,
		&result.DeliveryDays, &paymentTerms, &result.Notes,
		&result.CreatedAt, &result.UpdatedAt,
	); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return nil, pgErr
		}
		return nil, err
	}
	return result, nil
}

// GetRFQResponses retrieves all supplier responses for an RFQ
func (r *PurchasingRepository) GetRFQResponses(ctx context.Context, tenantID uuid.UUID, rfqID uuid.UUID) ([]models.RFQResponse, error) {
	responses := []models.RFQResponse{}

	rows, err := r.dbService.Query(ctx,
		`SELECT * FROM purchasing.sp_get_rfq_responses($1, $2)`,
		tenantID, rfqID,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return nil, pgErr
		}
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		resp := models.RFQResponse{}
		var paymentTerms string
		if err := rows.Scan(
			&resp.ID, &resp.RFQID, &resp.SupplierID, &resp.TotalPrice,
			&resp.DeliveryDays, &paymentTerms, &resp.Notes,
			&resp.CreatedAt, &resp.UpdatedAt,
		); err != nil {
			return nil, err
		}
		responses = append(responses, resp)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}

	return responses, nil
}

// SelectBestRFQResponse closes an RFQ by selecting the winning response
func (r *PurchasingRepository) SelectBestRFQResponse(ctx context.Context, tenantID uuid.UUID, rfqID, _ uuid.UUID) error {
	var updated bool
	if err := r.dbService.QueryRow(ctx,
		`SELECT * FROM purchasing.sp_select_best_rfq_response($1, $2)`,
		tenantID, rfqID,
	).Scan(&updated); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return pgErr
		}
		return err
	}
	return nil
}

// GetRFQComparison builds a comparison map of supplier responses for an RFQ
func (r *PurchasingRepository) GetRFQComparison(ctx context.Context, tenantID uuid.UUID, rfqID uuid.UUID) (map[string]any, error) {
	items, err := r.GetRFQItems(ctx, tenantID, rfqID)
	if err != nil {
		return nil, err
	}

	responses, err := r.GetRFQResponses(ctx, tenantID, rfqID)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"rfq_id":    rfqID,
		"items":     items,
		"responses": responses,
	}, nil
}
