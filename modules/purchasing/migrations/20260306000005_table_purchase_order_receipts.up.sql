-- Purchase Order Receipts (when goods arrive at warehouse)
CREATE TABLE IF NOT EXISTS purchasing.purchase_order_receipts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    purchase_order_id UUID NOT NULL REFERENCES purchasing.purchase_orders(id) ON DELETE CASCADE,
    receipt_number VARCHAR(50) NOT NULL,
    receipt_date TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    received_by UUID REFERENCES auth.users(id),
    notes TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_po_receipts_po ON purchasing.purchase_order_receipts(purchase_order_id);
CREATE UNIQUE INDEX idx_po_receipt_number ON purchasing.purchase_order_receipts(purchase_order_id, receipt_number);
