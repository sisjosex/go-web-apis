-- Rollback: make_tenant_id_nullable
-- Module: tracking

-- Revert tenant_id back to NOT NULL
-- Note: This will fail if there are any NULL tenant_id values in the table
ALTER TABLE tracking.transport_companies
ALTER COLUMN tenant_id SET NOT NULL;

