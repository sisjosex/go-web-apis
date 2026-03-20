-- Products table
CREATE TABLE IF NOT EXISTS inventory.products (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL,
    sku VARCHAR(50) NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    category VARCHAR(50),
    base_price DECIMAL(12,2) NOT NULL,
    has_variants BOOLEAN DEFAULT FALSE,
    status VARCHAR(20) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'discontinued')),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX idx_products_sku ON inventory.products(tenant_id, sku);
CREATE INDEX idx_products_tenant_id ON inventory.products(tenant_id);
CREATE INDEX idx_products_status ON inventory.products(status);
