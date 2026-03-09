-- Supplier product rates - tracks historical pricing for price comparison
CREATE TABLE IF NOT EXISTS purchasing.supplier_rates (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    supplier_id UUID NOT NULL REFERENCES purchasing.suppliers(id) ON DELETE CASCADE,
    product_id UUID NOT NULL REFERENCES inventory.products(id) ON DELETE CASCADE,
    unit_cost DECIMAL(12,2) NOT NULL,
    payment_terms INT DEFAULT 30,
    last_price_date TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_supplier_rates_supplier ON purchasing.supplier_rates(supplier_id);
CREATE INDEX IF NOT EXISTS idx_supplier_rates_product ON purchasing.supplier_rates(product_id);
CREATE INDEX IF NOT EXISTS idx_supplier_rates_cost ON purchasing.supplier_rates(unit_cost);
CREATE UNIQUE INDEX IF NOT EXISTS idx_supplier_rates_unique ON purchasing.supplier_rates(supplier_id, product_id);
