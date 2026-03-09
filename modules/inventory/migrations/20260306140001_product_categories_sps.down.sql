-- Rollback: Product Categories Stored Procedures

DROP FUNCTION IF EXISTS inventory.sp_get_category_product_count(UUID);
DROP FUNCTION IF EXISTS inventory.sp_get_products_by_category(UUID, INTEGER, INTEGER);
DROP FUNCTION IF EXISTS inventory.sp_remove_product_from_category(UUID, UUID);
DROP FUNCTION IF EXISTS inventory.sp_assign_product_to_category(UUID, UUID);
DROP FUNCTION IF EXISTS inventory.sp_search_categories(VARCHAR, BOOLEAN, INTEGER);
DROP FUNCTION IF EXISTS inventory.sp_list_categories(UUID, BOOLEAN, VARCHAR, INTEGER, INTEGER);
DROP FUNCTION IF EXISTS inventory.sp_get_category(UUID);
DROP FUNCTION IF EXISTS inventory.sp_delete_category(UUID);
DROP FUNCTION IF EXISTS inventory.sp_update_category(UUID, VARCHAR, VARCHAR, UUID, TEXT, VARCHAR, INTEGER, BOOLEAN) CASCADE;
DROP FUNCTION IF EXISTS inventory.sp_create_category(VARCHAR, VARCHAR, UUID, TEXT, VARCHAR, INTEGER) CASCADE;
