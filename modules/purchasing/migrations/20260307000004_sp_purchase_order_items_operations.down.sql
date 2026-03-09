-- Rollback: sp_purchase_order_items_operations
-- Module: purchasing

DROP FUNCTION IF EXISTS purchasing.sp_get_purchase_order_items(UUID) CASCADE;
