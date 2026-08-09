-- Rollback: sp_get_categories_by_product

DROP FUNCTION IF EXISTS inventory.sp_get_categories_by_product(UUID, UUID, INTEGER, INTEGER);
