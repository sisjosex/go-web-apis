-- Drop sales order SPs
DROP FUNCTION IF EXISTS sales.sp_create_sales_order CASCADE;
DROP FUNCTION IF EXISTS sales.sp_get_sales_order_by_id CASCADE;
DROP FUNCTION IF EXISTS sales.sp_get_sales_orders_by_tenant CASCADE;
DROP FUNCTION IF EXISTS sales.sp_get_sales_orders_by_customer CASCADE;
DROP FUNCTION IF EXISTS sales.sp_get_sales_order_by_number CASCADE;
DROP FUNCTION IF EXISTS sales.sp_update_sales_order CASCADE;
DROP FUNCTION IF EXISTS sales.sp_add_order_item CASCADE;
DROP FUNCTION IF EXISTS sales.sp_get_order_items CASCADE;
DROP SEQUENCE IF EXISTS sales.order_seq;
