-- Rollback: add_client_tenant_to_companies
-- Module: tracking

-- Rollback: Remove client_tenant_id column
DROP INDEX IF EXISTS tracking.idx_transport_companies_client_tenant;
ALTER TABLE tracking.transport_companies DROP COLUMN IF EXISTS client_tenant_id;
-- Example:
-- DROP TABLE IF EXISTS auth.my_table;

