-- Purchase Order Invoices (supplier invoices for payment)
CREATE TABLE IF NOT EXISTS purchasing.purchase_order_invoices (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    purchase_order_id UUID NOT NULL REFERENCES purchasing.purchase_orders(id) ON DELETE CASCADE,
    invoice_number VARCHAR(100) NOT NULL,   -- Invoice from supplier
    invoice_date DATE NOT NULL,
    invoice_amount DECIMAL(12,2) NOT NULL,
    tax_amount DECIMAL(12,2) DEFAULT 0,
    due_date DATE NOT NULL,
    status VARCHAR(20) DEFAULT 'received',  -- received, validated, partially_paid, paid, disputed
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_po_invoices_po ON purchasing.purchase_order_invoices(purchase_order_id);
CREATE INDEX idx_po_invoices_status ON purchasing.purchase_order_invoices(status);
CREATE UNIQUE INDEX idx_po_invoice_number ON purchasing.purchase_order_invoices(purchase_order_id, invoice_number);
