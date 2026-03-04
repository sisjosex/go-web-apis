-- Drop customer SPs
DROP FUNCTION IF EXISTS sales.sp_create_customer CASCADE;
DROP FUNCTION IF EXISTS sales.sp_get_customer_by_id CASCADE;
DROP FUNCTION IF EXISTS sales.sp_get_customers_by_tenant CASCADE;
DROP FUNCTION IF EXISTS sales.sp_get_customer_by_email CASCADE;
DROP FUNCTION IF EXISTS sales.sp_update_customer CASCADE;
DROP FUNCTION IF EXISTS sales.sp_delete_customer CASCADE;
