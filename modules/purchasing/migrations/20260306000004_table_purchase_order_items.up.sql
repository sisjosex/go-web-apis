-- Purchase Order Items
CREATE TABLE IF NOT EXISTS purchasing.purchase_order_items (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    purchase_order_id UUID NOT NULL REFERENCES purchasing.purchase_orders(id) ON DELETE CASCADE,
    product_id UUID NOT NULL REFERENCES inventory.products(id),
    quantity INT NOT NULL CHECK (quantity > 0),
    unit_cost DECIMAL(12,2) NOT NULL,
    line_total DECIMAL(12,2) GENERATED ALWAYS AS (quantity * unit_cost) STORED,
    received_quantity INT DEFAULT 0 CHECK (received_quantity >= 0),
    status VARCHAR(20) DEFAULT 'pending',   -- pending, partial, received, cancelled
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_po_items_po ON purchasing.purchase_order_items(purchase_order_id);
CREATE INDEX idx_po_items_product ON purchasing.purchase_order_items(product_id);
CREATE INDEX idx_po_items_status ON purchasing.purchase_order_items(status);
