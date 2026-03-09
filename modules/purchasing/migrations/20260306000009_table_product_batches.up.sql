-- Product batches for FIFO tracking and expiration management
CREATE TABLE IF NOT EXISTS purchasing.product_batches (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL,
    product_id UUID NOT NULL REFERENCES inventory.products(id) ON DELETE CASCADE,
    batch_number VARCHAR(100) NOT NULL,
    quantity INT NOT NULL DEFAULT 0,
    unit_cost DECIMAL(12,2) NOT NULL,
    receipt_date TIMESTAMP NOT NULL,
    expiration_date TIMESTAMP,                -- NULL for non-perishable items
    status VARCHAR(50) DEFAULT 'received',   -- received, partial_sold, sold, expired
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_product_batches_tenant ON purchasing.product_batches(tenant_id);
CREATE INDEX idx_product_batches_product ON purchasing.product_batches(product_id);
CREATE INDEX idx_product_batches_receipt_date ON purchasing.product_batches(receipt_date);
CREATE INDEX idx_product_batches_expiration ON purchasing.product_batches(expiration_date);
CREATE INDEX idx_product_batches_status ON purchasing.product_batches(status);
CREATE UNIQUE INDEX idx_product_batches_batch_unique ON purchasing.product_batches(tenant_id, product_id, batch_number);
