-- Product Categories Tables
-- Hierarchical product categorization with product mapping

-- ===== MAIN CATEGORIES TABLE =====
CREATE TABLE IF NOT EXISTS inventory.product_categories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    parent_id UUID REFERENCES inventory.product_categories(id) ON DELETE SET NULL,
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(255) NOT NULL UNIQUE,
    description TEXT,
    icon_url VARCHAR(512),
    display_order INTEGER DEFAULT 0,
    is_active BOOLEAN DEFAULT TRUE,
    deleted_at TIMESTAMP,  -- Soft delete
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_product_categories_parent_id ON inventory.product_categories(parent_id);
CREATE INDEX idx_product_categories_slug ON inventory.product_categories(slug);
CREATE INDEX idx_product_categories_is_active ON inventory.product_categories(is_active);
CREATE INDEX idx_product_categories_deleted_at ON inventory.product_categories(deleted_at);

-- ===== PRODUCT-CATEGORY MAPPING TABLE =====
CREATE TABLE IF NOT EXISTS inventory.product_category_mapping (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id UUID NOT NULL REFERENCES inventory.products(id) ON DELETE CASCADE,
    category_id UUID NOT NULL REFERENCES inventory.product_categories(id) ON DELETE CASCADE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(product_id, category_id)
);

CREATE INDEX idx_product_category_mapping_product_id ON inventory.product_category_mapping(product_id);
CREATE INDEX idx_product_category_mapping_category_id ON inventory.product_category_mapping(category_id);
