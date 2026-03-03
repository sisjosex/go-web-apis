-- Product variant options (Vanilla, Large, Extra Cheese, etc.)
CREATE TABLE IF NOT EXISTS inventory.product_variant_options (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    variant_group_id UUID NOT NULL REFERENCES inventory.product_variant_groups(id) ON DELETE CASCADE,
    option_name VARCHAR(255) NOT NULL,
    price_modifier DECIMAL(12,2) DEFAULT 0,
    is_available BOOLEAN DEFAULT TRUE,
    sort_order INT DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_variant_options_group ON inventory.product_variant_options(variant_group_id);
CREATE UNIQUE INDEX idx_variant_options_group_name ON inventory.product_variant_options(variant_group_id, option_name);
