ALTER TABLE tenancy.tenants
    DROP COLUMN IF EXISTS primary_module,
    DROP COLUMN IF EXISTS verticals;
