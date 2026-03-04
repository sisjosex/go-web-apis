-- Drop sales orders table
DROP TRIGGER IF EXISTS sales_orders_update_timestamp ON sales.sales_orders;
DROP FUNCTION IF EXISTS sales.sales_orders_update_timestamp();
DROP TABLE IF EXISTS sales.sales_orders;
