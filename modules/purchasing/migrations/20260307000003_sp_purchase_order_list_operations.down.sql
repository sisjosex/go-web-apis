-- Rollback: sp_purchase_order_list_operations
-- Module: purchasing

DROP FUNCTION IF EXISTS purchasing.sp_list_purchase_orders(UUID, VARCHAR, INT, INT) CASCADE;
DROP FUNCTION IF EXISTS purchasing.sp_get_purchase_orders_count(UUID, VARCHAR) CASCADE;
DROP FUNCTION IF EXISTS purchasing.sp_get_pending_payments(UUID) CASCADE;
