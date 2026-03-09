-- Drop purchasing operations SPs
DROP FUNCTION IF EXISTS purchasing.sp_add_purchase_order_item(UUID, UUID, INT, DECIMAL);
DROP FUNCTION IF EXISTS purchasing.sp_approve_purchase_order(UUID);
DROP FUNCTION IF EXISTS purchasing.sp_receive_purchase_order_items(UUID, TIMESTAMP, UUID, TEXT);
