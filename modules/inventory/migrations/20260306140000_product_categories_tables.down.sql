-- Rollback: Product Categories Tables

DROP INDEX IF EXISTS inventory.idx_product_category_mapping_category_id;
DROP INDEX IF EXISTS inventory.idx_product_category_mapping_product_id;
DROP TABLE IF EXISTS inventory.product_category_mapping;

DROP INDEX IF EXISTS inventory.idx_product_categories_deleted_at;
DROP INDEX IF EXISTS inventory.idx_product_categories_is_active;
DROP INDEX IF EXISTS inventory.idx_product_categories_slug;
DROP INDEX IF EXISTS inventory.idx_product_categories_parent_id;
DROP TABLE IF EXISTS inventory.product_categories;
