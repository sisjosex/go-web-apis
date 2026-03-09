-- Rollback: Full-text search indexes

-- Drop search indexes
DROP INDEX IF EXISTS inventory.idx_product_categories_search_gin;
DROP INDEX IF EXISTS inventory.idx_product_categories_description_search_gin;

-- Note: pg_trgm extension is left intact (doesn't hurt, other modules may use it)
-- Rollback will work even if extension is not dropped
