-- Drop customers table and schema
DROP TRIGGER IF EXISTS customers_update_timestamp ON sales.customers;
DROP FUNCTION IF EXISTS sales.customers_update_timestamp();
DROP TABLE IF EXISTS sales.customers;
DROP SCHEMA IF EXISTS sales CASCADE;
