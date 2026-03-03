-- Product stock (by warehouse)
CREATE TABLE IF NOT EXISTS inventory.product_stock (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    product_id UUID NOT NULL REFERENCES inventory.products(id) ON DELETE CASCADE,
    current_quantity DECIMAL(12,2) NOT NULL DEFAULT 0,
    last_updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(product_id)
);

CREATE INDEX idx_product_stock_product ON inventory.product_stock(product_id);
