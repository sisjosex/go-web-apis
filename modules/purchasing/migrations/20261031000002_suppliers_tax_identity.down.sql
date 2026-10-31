DROP INDEX IF EXISTS purchasing.idx_suppliers_tenant_active_name;
ALTER TABLE purchasing.suppliers DROP COLUMN IF EXISTS legal_name, DROP COLUMN IF EXISTS tax_id;
