-- Supplier products mapping (what suppliers have and their cost)
CREATE TABLE IF NOT EXISTS purchasing.supplier_products (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    supplier_id UUID NOT NULL REFERENCES purchasing.suppliers(id) ON DELETE CASCADE,
    product_id UUID NOT NULL REFERENCES inventory.products(id) ON DELETE CASCADE,
    supplier_sku VARCHAR(100),           -- SKU from supplier's system
    cost_per_unit DECIMAL(12,2) NOT NULL,
    min_order_qty INT DEFAULT 1,
    lead_time_days INT DEFAULT 3,         -- Days to delivery
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_supplier_products_supplier ON purchasing.supplier_products(supplier_id);
CREATE INDEX idx_supplier_products_product ON purchasing.supplier_products(product_id);
CREATE UNIQUE INDEX idx_supplier_products_unique ON purchasing.supplier_products(supplier_id, product_id);
