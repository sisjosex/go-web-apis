-- Rollback: Performance optimization indexes
-- Restores original index structure

-- Restore old broad index on deleted_at
CREATE INDEX idx_product_categories_deleted_at 
ON inventory.product_categories(deleted_at);

-- Drop new partial/composite indexes
DROP INDEX IF EXISTS inventory.idx_product_categories_active;
DROP INDEX IF EXISTS inventory.idx_product_categories_hierarchy_active;
DROP INDEX IF EXISTS inventory.idx_product_categories_status_active;
DROP INDEX IF EXISTS inventory.idx_product_category_mapping_category_fast;

-- Restore original product_id index on mapping
CREATE INDEX idx_product_category_mapping_category_id 
ON inventory.product_category_mapping(category_id);
