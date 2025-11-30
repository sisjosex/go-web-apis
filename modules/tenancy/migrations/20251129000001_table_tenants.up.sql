-- Create tenancy schema
CREATE SCHEMA IF NOT EXISTS tenancy;

-- Tenants table in main database
CREATE TABLE IF NOT EXISTS tenancy.tenants (
    id UUID PRIMARY KEY DEFAULT public.uuid_generate_v4(),
    slug VARCHAR(100) UNIQUE NOT NULL,
    name VARCHAR(255) NOT NULL,
    database_url TEXT DEFAULT NULL, -- NULL means use same DB with schema isolation
    schema_name VARCHAR(63) DEFAULT 'public', -- PostgreSQL schema for tenant data
    is_active BOOLEAN DEFAULT TRUE,
    is_suspended BOOLEAN DEFAULT FALSE,
    suspended_reason TEXT DEFAULT NULL,
    settings JSONB DEFAULT '{}', -- Tenant-specific configuration
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for tenants table
CREATE INDEX idx_tenants_slug ON tenancy.tenants (slug);
CREATE INDEX idx_tenants_is_active ON tenancy.tenants (is_active);
CREATE INDEX idx_tenants_is_suspended ON tenancy.tenants (is_suspended);
CREATE INDEX idx_tenants_created_at ON tenancy.tenants (created_at);

-- Trigger to update updated_at timestamp
CREATE OR REPLACE FUNCTION tenancy.update_tenant_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_update_tenant_updated_at
BEFORE UPDATE ON tenancy.tenants
FOR EACH ROW
EXECUTE FUNCTION tenancy.update_tenant_updated_at();
