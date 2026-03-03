-- Product variant groups (Flavor, Size, Topping, etc.)
CREATE TABLE IF NOT EXISTS inventory.product_variant_groups (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    product_id UUID NOT NULL REFERENCES inventory.products(id) ON DELETE CASCADE,
    group_type VARCHAR(50) NOT NULL,
    is_required BOOLEAN DEFAULT FALSE,
    max_selections INT DEFAULT 1,
    sort_order INT DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_variant_groups_product ON inventory.product_variant_groups(product_id);
CREATE UNIQUE INDEX idx_variant_groups_product_type ON inventory.product_variant_groups(product_id, group_type);
