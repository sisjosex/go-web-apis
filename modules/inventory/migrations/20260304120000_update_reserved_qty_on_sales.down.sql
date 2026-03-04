-- Rollback functions for reserved quantity management
DROP FUNCTION IF EXISTS inventory.sp_reserve_stock_for_order(UUID, DECIMAL);
DROP FUNCTION IF EXISTS inventory.sp_release_reserved_stock(UUID, DECIMAL);
DROP FUNCTION IF EXISTS inventory.sp_update_reorder_level(UUID, DECIMAL);
